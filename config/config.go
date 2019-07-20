package config

import (
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/prometheus/common/log"

	"github.com/BurntSushi/toml"
)

// Duration wrapper time.Duration for TOML
type duration struct {
	time.Duration
}

type Config struct {
	CF     cf
	Loki   loki
	Nozzle nozzle
}

type cf struct {
	APIEndpoint       string `toml:"api_endpoint" envconfig:"NOZZLE_API_ENDPOINT"`
	SkipSSLValidation bool   `toml:"skip_ssl_validation" envconfig:"NOZZLE_SKIP_SSL_VALIDATION"`
	SubscriptionID    string `toml:"subscription_id" envconfig:"NOZZLE_SUBSCRIPTION_ID"`
	UAAClientID       string `toml:"client_id" envconfig:"NOZZLE_UAA_CLIENT_ID"`
	UAAClientSecret   string `toml:"client_secret" envconfig:"NOZZLE_UAA_CLIENT_SECRET"`
}

type loki struct {
	BaseLabels string `toml:"base_labels" envconfig:"NOZZLE_BASE_LABELS"`
	Endpoint   string `toml:"endpoint" envconfig:"NOZZLE_LOKI_ENDPOINT"`
	Port       int    `toml:"port" envconfig:"NOZZLE_LOKI_PORT"`
}

type nozzle struct {
	AppCacheTTL        duration `toml:"app_cache_ttl" envconfig:"NOZZLE_APP_CACHE_INVALIDATE_TTL"`
	AppLimits          int      `toml:"app_limits" envconfig:"NOZZLE_APP_LIMITS"`
	BoltDBPath         string   `toml:"boltdb_path" envconfig:"NOZZLE_BOLTDB_PATH"`
	IgnoreMissingApps  bool     `toml:"ignore_missing_apps" envconfig:"NOZZLE_IGNORE_MISSING_APPS"`
	MissingAppCacheTTL duration `toml:"missing_app_cache_ttl" envconfig:"NOZZLE_MISSING_APP_CACHE_INVALIDATE_TTL"`
	OrgSpaceCacheTTL   duration `toml:"org_space_cache_ttl" envconfig:"NOZZLE_ORG_SPACE_CACHE_INVALIDATE_TTL"`
}

func (d *duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

func ParseConfig(path string) (Config, error) {
	var conf Config
	if _, err := toml.DecodeFile(path, &conf); err != nil {
		log.Fatalf("Error decoding config: %s", err.Error())
	}
	err := envconfig.Process("", &conf)
	if err != nil {
		log.Fatalf("Error while checking environment variables: %s", err)
	}
	return conf, nil
}
