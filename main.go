package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"

	"github.com/bosh-loki/loki-firehose-nozzle/extralabels"

	"github.com/bosh-loki/loki-firehose-nozzle/lokiclient"
	"github.com/bosh-loki/loki-firehose-nozzle/lokifirehosenozzle"

	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/prometheus/common/log"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	apiEndpoint       = kingpin.Flag("api-endpoint", "Cloud Foundry API Endpoint ($FIREHOSE_API_ENDPOINT)").Envar("FIREHOSE_API_ENDPOINT").Required().String()
	apiUsername       = kingpin.Flag("api-username", "Cloud Foundry API Username ($FIREHOSE_API_USERNAME)").Envar("FIREHOSE_API_USERNAME").Required().String()
	apiPassword       = kingpin.Flag("api-password", "Cloud Foundry API Password ($FIREHOSE_API_PASSWORD)").Envar("FIREHOSE_API_PASSWORD").Required().String()
	skipSSLValidation = kingpin.Flag("skip-ssl-verify", "Disable SSL Verify ($FIREHOSE_SKIP_SSL_VERIFY)").Envar("FIREHOSE_SKIP_SSL_VERIFY").Default("false").Bool()
	lokiEndpoint      = kingpin.Flag("loki-endpoint", "IP of Hostname where Loki run ($FIREHOSE_LOKI_ENDPOINT)").Envar("FIREHOSE_LOKI_ENDPOINT").Required().String()
	lokiPort          = kingpin.Flag("loki-port", "Port where Loki run ($FIREHOSE_LOKI_PORT)").Envar("FIREHOSE_LOKI_PORT").Default("3100").String()
	subscriptionID    = kingpin.Flag("subscription-id", "Id for the subscription ($FIREHOSE_SUBSCRIPTION_ID)").Envar("FIREHOSE_SUBSCRIPTION_ID").Default("loki").String()
	baseLabels        = kingpin.Flag("base-labels", "Extra labels you want to annotate your events with, example: '--base-labels=env:dev,something:other' ($FIREHOSE_BASE_LABELS)").Envar("$FIREHOSE_BASE_LABELS").Default("").String()
)

type LokiAdapter struct {
	client *lokiclient.Client
}

func main() {
	log.AddFlags(kingpin.CommandLine)
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	cfConfig := &cfclient.Config{
		ApiAddress:        *apiEndpoint,
		Username:          *apiUsername,
		Password:          *apiPassword,
		SkipSslValidation: *skipSSLValidation}

	cfClient, err := cfclient.NewClient(cfConfig)
	if err != nil {
		log.Fatal(err)
	}

	baseLabels, err := extralabels.SetBaseLabels(*baseLabels)
	if err != nil {
		log.Fatal(err)
	}
	lokiClient, err := lokiclient.NewWithDefaults(
		fmt.Sprintf("http://%s:%s/api/prom/push", *lokiEndpoint, *lokiPort),
		baseLabels,
	)
	if err != nil {
		log.Fatal(err)
	}

	client := lokifirehosenozzle.NewLokiFirehoseNozzle(cfConfig, cfClient, lokiClient, *subscriptionID)

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
