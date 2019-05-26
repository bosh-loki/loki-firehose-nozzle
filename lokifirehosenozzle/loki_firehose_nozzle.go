package lokifirehosenozzle

import (
	"crypto/tls"
	"time"

	"github.com/bosh-loki/firehose-loki-client/messages"

	"github.com/prometheus/common/log"

	"github.com/bosh-loki/firehose-loki-client/lokiclient"

	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/cloudfoundry/noaa/consumer"
	"github.com/cloudfoundry/sonde-go/events"
)

type FirehoseHandler interface {
	HandleEvent(*events.Envelope) error
}

type Firehose interface {
	Connect() (<-chan *events.Envelope, <-chan error)
	PostToLoki(*events.Envelope)
}

type LokiFirehoseNozzle struct {
	cfConfig       *cfclient.Config
	cfClient       *cfclient.Client
	lokiClient     *lokiclient.Client
	subscriptionID string
}

func NewLokiFirehoseNozzle(cfConfig *cfclient.Config, cfClient *cfclient.Client, subscriptionID string) Firehose {
	return &LokiFirehoseNozzle{cfConfig: cfConfig, cfClient: cfClient, subscriptionID: subscriptionID}
}

func (c *LokiFirehoseNozzle) Connect() (<-chan *events.Envelope, <-chan error) {
	var err error
	c.lokiClient, err = c.createClient()
	if err != nil {
		log.Fatalln(err)
	}
	cfConsumer := consumer.New(
		c.cfClient.Endpoint.DopplerEndpoint,
		&tls.Config{InsecureSkipVerify: c.cfConfig.SkipSslValidation},
		nil)

	refresher := cfClientTokenRefresh{cfClient: c.cfClient}
	cfConsumer.SetIdleTimeout(time.Duration(30) * time.Second)
	cfConsumer.SetMaxRetryCount(20)
	cfConsumer.RefreshTokenFrom(&refresher)
	return cfConsumer.Firehose(c.subscriptionID, "")
}

func (c *LokiFirehoseNozzle) createClient() (*lokiclient.Client, error) {
	baseLabels := messages.LabelSet{}
	lokiClient, err := lokiclient.NewWithDefaults("http://localhost:3100/api/prom/push", baseLabels)
	if err != nil {
		log.Fatalln(err)
	}
	return lokiClient, nil
}

func (c *LokiFirehoseNozzle) PostToLoki(e *events.Envelope) {
	lastLineTime := time.Now()
	labels := messages.GetLabels(e)
	// log.Infoln(string(e.GetLogMessage().GetMessage()))
	_ = c.lokiClient.Handle(labels, lastLineTime, string(e.GetLogMessage().GetMessage()))
}

type cfClientTokenRefresh struct {
	cfClient *cfclient.Client
}

func (ct *cfClientTokenRefresh) RefreshAuthToken() (token string, err error) {
	return ct.cfClient.GetToken()
}
