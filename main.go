package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"

	"github.com/bosh-loki/loki-firehose-nozzle/cache"
	"github.com/bosh-loki/loki-firehose-nozzle/extralabels"

	"github.com/bosh-loki/loki-firehose-nozzle/lokiclient"
	"github.com/bosh-loki/loki-firehose-nozzle/lokifirehosenozzle"

	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/prometheus/common/log"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	apiEndpoint        = kingpin.Flag("api-endpoint", "Cloud Foundry API Endpoint ($FIREHOSE_API_ENDPOINT)").Envar("FIREHOSE_API_ENDPOINT").Required().String()
	uaaClientID        = kingpin.Flag("client-id", "Cloud Foundry UAA Client ID ($FIREHOSE_UAA_CLIENT_ID)").Envar("FIREHOSE_UAA_CLIENT_ID").Required().String()
	uaaClientSecret    = kingpin.Flag("client-secret", "Cloud Foundry UAA Client Secret ($FIREHOSE_UAA_CLIENT_SECRET)").Envar("FIREHOSE_UAA_CLIENT_SECRET").Required().String()
	skipSSLValidation  = kingpin.Flag("skip-ssl-verify", "Disable SSL Verify ($FIREHOSE_SKIP_SSL_VERIFY)").Envar("FIREHOSE_SKIP_SSL_VERIFY").Default("false").Bool()
	lokiEndpoint       = kingpin.Flag("loki-endpoint", "IP of Hostname where Loki run ($FIREHOSE_LOKI_ENDPOINT)").Envar("FIREHOSE_LOKI_ENDPOINT").Required().String()
	lokiPort           = kingpin.Flag("loki-port", "Port where Loki run ($FIREHOSE_LOKI_PORT)").Envar("FIREHOSE_LOKI_PORT").Default("3100").String()
	subscriptionID     = kingpin.Flag("subscription-id", "Id for the subscription ($FIREHOSE_SUBSCRIPTION_ID)").Envar("FIREHOSE_SUBSCRIPTION_ID").Default("loki").String()
	baseLabels         = kingpin.Flag("base-labels", "Extra labels you want to annotate your events with, example: '--base-labels=env:dev,something:other' ($FIREHOSE_BASE_LABELS)").Envar("$FIREHOSE_BASE_LABELS").Default("").String()
	boltDBPath         = kingpin.Flag("boltdb-path", "Bolt Database path ($FIREHOSE_BOLTDB_PATH)").Envar("FIREHOSE_BOLTDB_PATH").String()
	ignoreMissingApps  = kingpin.Flag("ignore-missing-apps", "Enable throttling on cache lookup for missing apps").Envar("FIREHOSE_IGNORE_MISSING_APPS").Default("false").Bool()
	missingAppCacheTTL = kingpin.Flag("missing-app-cache-invalidate-ttl", "If the application is missing, then stop repeatedly querying application info from Cloud Foundry.").Envar("MISSING_APP_CACHE_INVALIDATE_TTL").Default("0s").Duration()
	appCacheTTL        = kingpin.Flag("app-cache-invalidate-ttl", "How frequently the app info local cache invalidates").Envar("APP_CACHE_INVALIDATE_TTL").Default("0s").Duration()
	orgSpaceCacheTTL   = kingpin.Flag("org-space-cache-invalidate-ttl", "How frequently the org and space cache invalidates").Envar("ORG_SPACE_CACHE_INVALIDATE_TTL").Default("72h").Duration()
	appLimits          = kingpin.Flag("app-limits", "Restrict to APP_LIMITS most updated apps per request when populating the app metadata cache").Envar("APP_LIMITS").Default("0").Int()
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
		ClientID:          *uaaClientID,
		ClientSecret:      *uaaClientSecret,
		SkipSslValidation: *skipSSLValidation,
		UserAgent:         "loki-firehose-nozzle",
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

	cacheConfig := &cache.BoltdbConfig{
		Path:               *boltDBPath,
		IgnoreMissingApps:  *ignoreMissingApps,
		MissingAppCacheTTL: *missingAppCacheTTL,
		AppCacheTTL:        *appCacheTTL,
		OrgSpaceCacheTTL:   *orgSpaceCacheTTL,
		AppLimits:          *appLimits,
	}

	client := lokifirehosenozzle.NewLokiFirehoseNozzle(cfConfig, lokiClient, cacheConfig, *subscriptionID)

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
