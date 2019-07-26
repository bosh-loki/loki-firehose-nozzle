package extralabels_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestExtralabels(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Extralabels Suite")
}
