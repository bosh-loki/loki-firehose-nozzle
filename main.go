package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/bosh-loki/loki-firehose-nozzle/cache"
	"github.com/bosh-loki/loki-firehose-nozzle/config"
	"github.com/bosh-loki/loki-firehose-nozzle/extralabels"

	"github.com/bosh-loki/loki-firehose-nozzle/lokiclient"
	"github.com/bosh-loki/loki-firehose-nozzle/lokifirehosenozzle"

	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/prometheus/common/log"
)

var (
	configFile = flag.String("config", "", "Location of the nozzle config toml file")
)

type LokiAdapter struct {
	client *lokiclient.Client
}

func main() {
	flag.Parse()

	conf, err := config.ParseConfig(*configFile)
	if err != nil {
		log.Fatalf("Error parsing config: %s", err)
	}

	cfConfig := &cfclient.Config{
		ApiAddress:        conf.CF.APIEndpoint,
		ClientID:          conf.CF.UAAClientID,
		ClientSecret:      conf.CF.UAAClientSecret,
		SkipSslValidation: conf.CF.SkipSSLValidation,
		UserAgent:         "loki-firehose-nozzle",
	}

	baseLabels, err := extralabels.SetBaseLabels(conf.Loki.BaseLabels)
	if err != nil {
		log.Fatal(err)
	}
	lokiClient, err := lokiclient.NewWithDefaults(
		fmt.Sprintf("http://%s:%d/api/prom/push", conf.Loki.Endpoint, conf.Loki.Port),
		baseLabels,
	)
	if err != nil {
		log.Fatal(err)
	}

	cacheConfig := &cache.BoltdbConfig{
		Path:               conf.Nozzle.BoltDBPath,
		IgnoreMissingApps:  conf.Nozzle.IgnoreMissingApps,
		MissingAppCacheTTL: conf.Nozzle.MissingAppCacheTTL.Duration,
		AppCacheTTL:        conf.Nozzle.AppCacheTTL.Duration,
		OrgSpaceCacheTTL:   conf.Nozzle.OrgSpaceCacheTTL.Duration,
		AppLimits:          conf.Nozzle.AppLimits,
	}

	client := lokifirehosenozzle.NewLokiFirehoseNozzle(cfConfig, lokiClient, cacheConfig, conf.CF.SubscriptionID)

	firehose, errorhose := client.Connect()
	if firehose == nil {
		panic(errors.New("firehose was nil"))
	} else if errorhose == nil {
		panic(errors.New("errorhose was nil"))
	}
	exitSignal := make(chan os.Signal, 1)
	signal.Notify(exitSignal, os.Interrupt)

	for {
		select {
		case envelope := <-firehose:
			if envelope == nil {
				log.Errorln("received nil envelope")
			} else {
				client.PostToLoki(envelope)
			}
		case err := <-errorhose:
			if err == nil {
				log.Errorln("received nil envelope")
			} else {
				log.Errorln(err)
			}
		case <-exitSignal:
			os.Exit(0)
		}
	}
}
