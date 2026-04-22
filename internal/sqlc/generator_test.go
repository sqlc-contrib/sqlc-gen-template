package sqlc_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sqlc-dev/plugin-sdk-go/plugin"

	"github.com/sqlc-contrib/sqlc-gen-template/internal/sqlc"
)

var _ = Describe("Generate", func() {
	It("returns an error when plugin options are missing template_dir", func() {
		req := &plugin.GenerateRequest{
			PluginOptions: []byte(`{}`),
		}
		_, err := sqlc.Generate(context.Background(), req)
		Expect(err).To(MatchError(ContainSubstring("template_dir")))
	})

	It("renders a minimal template end-to-end", func() {
		dir, err := os.MkdirTemp("", "sqlc-gen-template-*")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { _ = os.RemoveAll(dir) })

		Expect(os.WriteFile(
			filepath.Join(dir, "hello.txt.tmpl"),
			[]byte(`{{ .SqlcVersion }}`),
			0o644,
		)).To(Succeed())

		opts, err := json.Marshal(map[string]any{"template_dir": dir})
		Expect(err).NotTo(HaveOccurred())

		req := &plugin.GenerateRequest{
			SqlcVersion:   "v1.30.0",
			Settings:      &plugin.Settings{Engine: "postgresql"},
			Catalog:       &plugin.Catalog{},
			PluginOptions: opts,
		}

		resp, err := sqlc.Generate(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Files).To(HaveLen(1))
		Expect(resp.Files[0].Name).To(Equal("hello.txt"))
		Expect(string(resp.Files[0].Contents)).To(Equal("v1.30.0"))
	})
})
