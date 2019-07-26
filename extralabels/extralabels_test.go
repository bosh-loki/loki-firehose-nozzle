package extralabels_test

import (
	. "github.com/bosh-loki/loki-firehose-nozzle/extralabels"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Extra Labels", func() {
	Describe("Parse Base Labels", func() {
		Context("called with an empty string", func() {
			It("should return an empty hash", func() {
				expected := map[string]string{}
				Expect(SetBaseLabels("")).To(Equal(expected))
			})
		})

		Context("called with an extra label", func() {
			It("should return a hash with labels that we want", func() {
				expected := map[string]string{"env": "prod", "foo": "bar"}
				extraLabels := "env:prod,foo:bar"
				Expect(SetBaseLabels(extraLabels)).To(Equal(expected))
			})
		})

		Context("called with an extra label with whitespace", func() {
			It("should return a hash with labels that we want", func() {
				expected := map[string]string{"env": "prod", "foo": "bar"}
				extraLabels := "    env:    \nprod,    foo:bar    "
				Expect(SetBaseLabels(extraLabels)).To(Equal(expected))
			})
		})

		Context("called with too many values in key-value pair", func() {
			It("should return a error", func() {
				extraLabels := "fizz:buzz:bazz"
				_, err := SetBaseLabels(extraLabels)
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
