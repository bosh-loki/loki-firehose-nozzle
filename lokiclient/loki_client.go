package lokiclient

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/bosh-loki/loki-firehose-nozzle/messages"

	"github.com/prometheus/common/log"

	"github.com/golang/protobuf/ptypes/timestamp"

	"github.com/bosh-loki/loki-firehose-nozzle/logproto"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
)

const contentType = "application/x-protobuf"
const maxErrMsgLen = 1024

// Config describes configuration for a HTTP pusher client.
type Config struct {
	URL       string
	BatchWait time.Duration
	BatchSize int

	BackoffConfig  BackoffConfig     `yaml:"backoff_config"`
	ExternalLabels messages.LabelSet `yaml:"external_labels,omitempty"`
	Timeout        time.Duration     `yaml:"timeout"`
}

// Client for pushing logs in snappy-compressed protos over HTTP.
type Client struct {
	cfg            Config
	quit           chan struct{}
	entries        chan entry
	wg             sync.WaitGroup
	externalLabels messages.LabelSet
	stopLock       sync.Mutex
	stopped        bool
}

type entry struct {
	labels messages.LabelSet
	logproto.Entry
}

// NewWithDefaults makes a new Client with default config.
func NewWithDefaults(url string, externalLabels messages.LabelSet) (*Client, error) {
	cfg := Config{
		URL:       url,
		BatchWait: time.Second,
		BatchSize: 100 * 1024,
		Timeout:   10 * time.Second,
		BackoffConfig: BackoffConfig{
			MinBackoff: 100 * time.Millisecond,
			MaxBackoff: 10 * time.Second,
			MaxRetries: 10,
		},
		ExternalLabels: externalLabels,
	}
	return New(cfg)
}

// New makes a new Client.
func New(cfg Config) (*Client, error) {
	c := &Client{
		cfg:            cfg,
		quit:           make(chan struct{}),
		entries:        make(chan entry),
		externalLabels: cfg.ExternalLabels,
	}
	c.wg.Add(1)
	go c.run()
	return c, nil
}

func (c *Client) run() {
	batch := map[string]*logproto.Stream{}
	batchSize := 0
	maxWait := time.NewTimer(c.cfg.BatchWait)

	defer func() {
		c.sendBatch(batch)
		c.wg.Done()
	}()

	for {
		maxWait.Reset(c.cfg.BatchWait)
		select {
		case <-c.quit:
			return

		case e := <-c.entries:
			if batchSize+len(e.Line) > c.cfg.BatchSize {
				c.sendBatch(batch)
				batchSize = 0
				batch = map[string]*logproto.Stream{}
			}

			batchSize += len(e.Line)
			fp := e.labels.String()
			stream, ok := batch[fp]
			if !ok {
				stream = &logproto.Stream{
					Labels: fp,
				}
				batch[fp] = stream
			}
			stream.Entries = append(stream.Entries, &e.Entry)

		case <-maxWait.C:
			if len(batch) > 0 {
				c.sendBatch(batch)
				batchSize = 0
				batch = map[string]*logproto.Stream{}
			}
		}
	}
}

func (c *Client) sendBatch(batch map[string]*logproto.Stream) {
	buf, err := encodeBatch(batch)
	if err != nil {
		log.Errorf("Error encoding batch: %s", err)
		return
	}

	ctx := context.Background()
	backoff := NewBackoff(ctx, c.cfg.BackoffConfig)
	var status int
	for backoff.Ongoing() {
		status, err = c.send(ctx, buf)

		if err == nil {
			return
		}

		// Only retry 500s and connection-level errors.
		if status > 0 && status/100 != 5 {
			break
		}

		log.Warnf("Error sending batch, will retry %s %s", status, err)
		backoff.Wait()
	}

	if err != nil {
		log.Errorf("Final error sending batch %s %s", status, err)
	}
}

func encodeBatch(batch map[string]*logproto.Stream) ([]byte, error) {
	req := logproto.PushRequest{
		Streams: make([]*logproto.Stream, 0, len(batch)),
	}
	for _, stream := range batch {
		req.Streams = append(req.Streams, stream)
	}
	buf, err := proto.Marshal(&req)
	if err != nil {
		return nil, err
	}
	buf = snappy.Encode(nil, buf)
	return buf, nil
}

func (c *Client) send(ctx context.Context, buf []byte) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()
	req, err := http.NewRequest("POST", c.cfg.URL, bytes.NewReader(buf))
	if err != nil {
		return -1, err
	}
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", contentType)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return -1, err
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		scanner := bufio.NewScanner(io.LimitReader(resp.Body, maxErrMsgLen))
		line := ""
		if scanner.Scan() {
			line = scanner.Text()
		}
		err = fmt.Errorf("server returned HTTP status %s (%d): %s", resp.Status, resp.StatusCode, line)
	}
	return resp.StatusCode, err
}

// Stop the client.
func (c *Client) Stop() {
	c.stopLock.Lock()
	defer c.stopLock.Unlock()
	if c.stopped {
		return
	}
	log.Info("Loki client waiting for stop")
	c.stopped = true
	close(c.quit)
	c.wg.Wait()
}

// Handle implement EntryHandler; adds a new line to the next batch; send is async.
func (c *Client) Handle(ls messages.LabelSet, t time.Time, s string) error {
	if len(c.externalLabels) > 0 {
		ls = c.externalLabels.Merge(ls)
	}

	now := time.Now().UnixNano()
	c.entries <- entry{ls, logproto.Entry{
		Timestamp: &timestamp.Timestamp{
			Seconds: now / int64(time.Second),
			Nanos:   int32(now % int64(time.Second)),
		},
		Line: s,
	}}
	return nil
}
