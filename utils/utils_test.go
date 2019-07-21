package utils_test

import (
	. "github.com/bosh-loki/loki-firehose-nozzle/utils"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Testing Utils packages", func() {
	Describe("UUID Formated", func() {
		Context("Called with proper UUID", func() {
			It("Should return formated String", func() {
				uuid := &events.UUID{High: proto.Uint64(0), Low: proto.Uint64(0)}
				Expect(FormatUUID(uuid)).To(Equal(("00000000-0000-0000-0000-000000000000")))
			})
		})
	})
})
