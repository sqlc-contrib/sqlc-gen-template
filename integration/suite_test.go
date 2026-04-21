package integration_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var binaryPath string

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

var _ = BeforeSuite(func() {
	By("Building sqlc-gen-template", func() {
		path, err := gexec.Build("github.com/sqlc-contrib/sqlc-gen-template/cmd/sqlc-gen-template")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(gexec.CleanupBuildArtifacts)
		Expect(path).NotTo(BeEmpty())
		binaryPath = path
	})
})
