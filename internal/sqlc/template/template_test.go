package template_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sqlc-dev/plugin-sdk-go/plugin"

	"github.com/sqlc-contrib/sqlc-gen-template/internal/sqlc/template"
)

// write builds a templates tree under dir from a path→body map.
func write(dir string, files map[string]string) {
	for rel, body := range files {
		full := filepath.Join(dir, rel)
		Expect(os.MkdirAll(filepath.Dir(full), 0o755)).To(Succeed())
		Expect(os.WriteFile(full, []byte(body), 0o644)).To(Succeed())
	}
}

// sampleRequest returns a minimal GenerateRequest exercising a catalog
// with one table and one query.
func sampleRequest() *plugin.GenerateRequest {
	return &plugin.GenerateRequest{
		SqlcVersion: "v1.30.0",
		Settings:    &plugin.Settings{Engine: "postgresql"},
		Catalog: &plugin.Catalog{
			Schemas: []*plugin.Schema{{
				Name: "public",
				Tables: []*plugin.Table{{
					Rel: &plugin.Identifier{Schema: "public", Name: "users"},
					Columns: []*plugin.Column{
						{Name: "id", Type: &plugin.Identifier{Name: "int4"}, NotNull: true},
						{Name: "email", Type: &plugin.Identifier{Name: "text"}, NotNull: true},
					},
				}},
			}},
		},
		Queries: []*plugin.Query{
			{Name: "GetUser", Cmd: ":one"},
		},
	}
}

var _ = Describe("Render", func() {
	var dir string

	BeforeEach(func() {
		var err error
		dir, err = os.MkdirTemp("", "sqlc-gen-template-*")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { _ = os.RemoveAll(dir) })
	})

	When("the templates directory is missing", func() {
		It("returns a stat error", func() {
			_, err := template.Render(template.Context{TemplateDir: filepath.Join(dir, "missing")})
			Expect(err).To(HaveOccurred())
		})
	})

	When("the templates directory has no .tmpl files", func() {
		It("returns a no-templates error", func() {
			_, err := template.Render(template.Context{TemplateDir: dir})
			Expect(err).To(MatchError(ContainSubstring("no .tmpl files")))
		})
	})

	When("rendering a single template", func() {
		It("emits one file with the filename stripped of .tmpl", func() {
			write(dir, map[string]string{
				"models.go.tmpl": `package {{ .Options.package_name }}
// {{ len .Request.Catalog.Schemas }} schemas
`,
			})
			files, err := template.Render(template.Context{
				Request:     sampleRequest(),
				Options:     map[string]any{"package_name": "db"},
				TemplateDir: dir,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(files).To(HaveLen(1))
			Expect(files[0].Name).To(Equal("models.go"))
			Expect(string(files[0].Contents)).To(ContainSubstring("package db"))
			Expect(string(files[0].Contents)).To(ContainSubstring("1 schemas"))
		})
	})

	When("rendering multiple templates in nested dirs", func() {
		It("preserves directory structure", func() {
			write(dir, map[string]string{
				"models.go.tmpl":           `package x`,
				"queries/one.sql.tmpl":     `SELECT 1;`,
				"queries/sub/two.sql.tmpl": `SELECT 2;`,
			})
			files, err := template.Render(template.Context{
				Request:     sampleRequest(),
				TemplateDir: dir,
			})
			Expect(err).NotTo(HaveOccurred())
			names := make([]string, 0, len(files))
			for _, f := range files {
				names = append(names, f.Name)
			}
			Expect(names).To(ConsistOf("models.go", "queries/one.sql", "queries/sub/two.sql"))
		})
	})

	When("a template filename contains template expressions", func() {
		It("renders the filename against the context", func() {
			write(dir, map[string]string{
				"{{ .Options.name }}_models.go.tmpl": `package x`,
			})
			files, err := template.Render(template.Context{
				Request:     sampleRequest(),
				Options:     map[string]any{"name": "user"},
				TemplateDir: dir,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(files).To(HaveLen(1))
			Expect(files[0].Name).To(Equal("user_models.go"))
		})
	})

	When("a template file starts with underscore", func() {
		It("is parsed as a partial but not emitted", func() {
			write(dir, map[string]string{
				"_header.tmpl":   `// HEADER`,
				"models.go.tmpl": `{{ template "_header.tmpl" . }}` + "\npackage x",
			})
			files, err := template.Render(template.Context{
				Request:     sampleRequest(),
				TemplateDir: dir,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(files).To(HaveLen(1))
			Expect(files[0].Name).To(Equal("models.go"))
			Expect(string(files[0].Contents)).To(ContainSubstring("// HEADER"))
			Expect(string(files[0].Contents)).To(ContainSubstring("package x"))
		})
	})

	When("a template references a missing function", func() {
		It("returns a parse error", func() {
			write(dir, map[string]string{
				"bad.tmpl": `{{ nosuch . }}`,
			})
			_, err := template.Render(template.Context{
				Request:     sampleRequest(),
				TemplateDir: dir,
			})
			Expect(err).To(MatchError(ContainSubstring("parse")))
		})
	})

	When("the body uses language helpers", func() {
		It("resolves a column through goType", func() {
			write(dir, map[string]string{
				"out.txt.tmpl": `{{ range .Request.Catalog.Schemas }}{{ range .Tables }}{{ range .Columns }}{{ .Name }}={{ goType . }};{{ end }}{{ end }}{{ end }}`,
			})
			files, err := template.Render(template.Context{
				Request:     sampleRequest(),
				TemplateDir: dir,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(string(files[0].Contents)).To(Equal("id=int32;email=string;"))
		})
	})
})
