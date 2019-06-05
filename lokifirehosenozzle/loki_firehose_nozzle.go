package lokifirehosenozzle

import (
	"crypto/tls"
	"time"

	"github.com/bosh-loki/loki-firehose-nozzle/cache"
	"github.com/bosh-loki/loki-firehose-nozzle/messages"
	"github.com/prometheus/common/log"

	"github.com/bosh-loki/loki-firehose-nozzle/lokiclient"

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
	cfClient       *cfclient.Client
	cfConfig       *cfclient.Config
	cachingConfig  *cache.BoltdbConfig
	cachingClient  cache.Cache
	lokiClient     *lokiclient.Client
	subscriptionID string
}

func NewLokiFirehoseNozzle(cfConfig *cfclient.Config, lokiClient *lokiclient.Client, cachingConfig *cache.BoltdbConfig, subscriptionID string) Firehose {
	return &LokiFirehoseNozzle{
		cfConfig:       cfConfig,
		lokiClient:     lokiClient,
		cachingConfig:  cachingConfig,
		subscriptionID: subscriptionID,
	}
}

func (c *LokiFirehoseNozzle) Connect() (<-chan *events.Envelope, <-chan error) {
	c.cfClient = c.createCFClinet()
	c.cachingClient = c.createCachingClinet()

	cfConsumer := consumer.New(
		c.cfClient.Endpoint.DopplerEndpoint,
		&tls.Config{InsecureSkipVerify: c.cfConfig.SkipSslValidation},
		nil)
	log.Infof("Using Doppler endpoint: %s", c.cfClient.Endpoint.DopplerEndpoint)

	refresher := cfClientTokenRefresh{cfClient: c.cfClient}
	cfConsumer.SetIdleTimeout(time.Duration(30) * time.Second)
	cfConsumer.SetMaxRetryCount(20)
	cfConsumer.RefreshTokenFrom(&refresher)
	return cfConsumer.Firehose(c.subscriptionID, "")
}

func (c *LokiFirehoseNozzle) PostToLoki(e *events.Envelope) {
	lastLineTime := time.Now()
	event := messages.GetMessage(e, c.cachingClient)
	_ = c.lokiClient.Handle(event.Labels, lastLineTime, event.Msg)
}

func (c *LokiFirehoseNozzle) createCFClinet() *cfclient.Client {
	cfClient, err := cfclient.NewClient(c.cfConfig)
	if err != nil {
		log.Errorf("Encountered an error while setting up the cf client: %v", err)
		return nil
	}
	return cfClient
}

type cfClientTokenRefresh struct {
	cfClient *cfclient.Client
}

func (ct *cfClientTokenRefresh) RefreshAuthToken() (token string, err error) {
	log.Infoln("Refreshing Auth Token")
	return ct.cfClient.GetToken()
}

// AppCache creates in-memory cache or boltDB cache
func (c *LokiFirehoseNozzle) appCache(client cache.AppClient) (cache.Cache, error) {
	if c.cachingConfig.Path != "" {
		log.Infoln("Using BoltDB for cache.")
		return cache.NewBoltdb(client, c.cachingConfig)
	}

	log.Infoln("Using in Memory cache.")
	return cache.NewNoCache(), nil
}

func (c *LokiFirehoseNozzle) createCachingClinet() cache.Cache {
	appCache, err := c.appCache(c.cfClient)
	if err != nil {
		log.Errorf("Encountered an error while setting up the caching client: %v", err)
		return nil
	}

	err = appCache.Open()
	if err != nil {
		log.Errorf("Error open cache: %v", err)
		return nil
	}
	defer appCache.Close()

	return appCache
}
