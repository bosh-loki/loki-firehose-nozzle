package config_test

import (
	"os"
	"time"

	. "github.com/bosh-loki/loki-firehose-nozzle/config"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("NozzleConfig", func() {
	BeforeEach(func() {
		os.Clearenv()
	})

	It("successfully parses a valid config", func() {
		conf, err := ParseConfig("testdata/test_config.toml")
		Expect(err).ToNot(HaveOccurred())
		Expect(conf.CF.APIEndpoint).To(Equal("https://api.cf.com"))
		Expect(conf.CF.SkipSSLValidation).To(Equal(true))
		Expect(conf.CF.SubscriptionID).To(Equal("loki-nozzle"))
		Expect(conf.CF.UAAClientID).To(Equal("user"))
		Expect(conf.CF.UAAClientSecret).To(Equal("password"))
		Expect(conf.Loki.BaseLabels).To(Equal("env:prod,region:us"))
		Expect(conf.Loki.Endpoint).To(Equal("10.244.0.2"))
		Expect(conf.Loki.Port).To(Equal(3100))
		Expect(conf.Nozzle.AppCacheTTL.Duration).To(BeEquivalentTo(0 * time.Second))
		Expect(conf.Nozzle.AppLimits).To(Equal(0))
		Expect(conf.Nozzle.BoltDBPath).To(Equal("/var/vcap/nozzle.db"))
		Expect(conf.Nozzle.IgnoreMissingApps).To(Equal(false))
		Expect(conf.Nozzle.MissingAppCacheTTL.Duration).To(BeEquivalentTo(0 * time.Second))
		Expect(conf.Nozzle.OrgSpaceCacheTTL.Duration).To(Equal(72 * time.Hour))
	})

	It("successfully overwrites file config values with environmental variables", func() {
		os.Setenv("NOZZLE_API_ENDPOINT", "https://api.cf-dev.com")
		os.Setenv("NOZZLE_APP_CACHE_INVALIDATE_TTL", "10s")
		os.Setenv("NOZZLE_APP_LIMITS", "1")
		os.Setenv("NOZZLE_BASE_LABELS", "env:stg,nozzle:foobar")
		os.Setenv("NOZZLE_BOLTDB_PATH", "/tmp/nozzle.db")
		os.Setenv("NOZZLE_IGNORE_MISSING_APPS", "true")
		os.Setenv("NOZZLE_LOKI_ENDPOINT", "192.168.1.111")
		os.Setenv("NOZZLE_LOKI_PORT", "3200")
		os.Setenv("NOZZLE_MISSING_APP_CACHE_INVALIDATE_TTL", "10s")
		os.Setenv("NOZZLE_ORG_SPACE_CACHE_INVALIDATE_TTL", "48h")
		os.Setenv("NOZZLE_SKIP_SSL_VALIDATION", "false")
		os.Setenv("NOZZLE_SUBSCRIPTION_ID", "loki-nozzle-dev")
		os.Setenv("NOZZLE_UAA_CLIENT_ID", "loki-client")
		os.Setenv("NOZZLE_UAA_CLIENT_SECRET", "supersecret")

		conf, err := ParseConfig("testdata/test_config.toml")
		Expect(err).ToNot(HaveOccurred())
		Expect(conf.CF.APIEndpoint).To(Equal("https://api.cf-dev.com"))
		Expect(conf.CF.SkipSSLValidation).To(Equal(false))
		Expect(conf.CF.SubscriptionID).To(Equal("loki-nozzle-dev"))
		Expect(conf.CF.UAAClientID).To(Equal("loki-client"))
		Expect(conf.CF.UAAClientSecret).To(Equal("supersecret"))
		Expect(conf.Loki.BaseLabels).To(Equal("env:stg,nozzle:foobar"))
		Expect(conf.Loki.Endpoint).To(Equal("192.168.1.111"))
		Expect(conf.Loki.Port).To(Equal(3200))
		Expect(conf.Nozzle.AppCacheTTL.Duration).To(BeEquivalentTo(10 * time.Second))
		Expect(conf.Nozzle.AppLimits).To(Equal(1))
		Expect(conf.Nozzle.BoltDBPath).To(Equal("/tmp/nozzle.db"))
		Expect(conf.Nozzle.IgnoreMissingApps).To(Equal(true))
		Expect(conf.Nozzle.MissingAppCacheTTL.Duration).To(BeEquivalentTo(10 * time.Second))
		Expect(conf.Nozzle.OrgSpaceCacheTTL.Duration).To(Equal(48 * time.Hour))
	})
})
