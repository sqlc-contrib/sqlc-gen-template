package integration_test

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sqlc-dev/plugin-sdk-go/plugin"
	"google.golang.org/protobuf/proto"
)

// runPlugin invokes the built plugin binary with the given GenerateRequest
// and returns the decoded response. It mirrors how sqlc invokes process
// plugins: proto-marshalled request on stdin, proto-marshalled response
// on stdout.
func runPlugin(req *plugin.GenerateRequest) *plugin.GenerateResponse {
	reqBytes, err := proto.Marshal(req)
	Expect(err).NotTo(HaveOccurred())

	cmd := exec.Command(binaryPath)
	cmd.Stdin = bytes.NewReader(reqBytes)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	Expect(cmd.Run()).To(Succeed(), "plugin failed: %s", stderr.String())

	var resp plugin.GenerateResponse
	Expect(proto.Unmarshal(stdout.Bytes(), &resp)).To(Succeed())
	return &resp
}

// runPluginExpectErr runs the plugin expecting a non-zero exit.
func runPluginExpectErr(req *plugin.GenerateRequest) string {
	reqBytes, err := proto.Marshal(req)
	Expect(err).NotTo(HaveOccurred())

	cmd := exec.Command(binaryPath)
	cmd.Stdin = bytes.NewReader(reqBytes)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err = cmd.Run()
	Expect(err).To(HaveOccurred())
	return stderr.String()
}

// loadExpected reads every file under dir and returns them keyed by
// forward-slash relative path, mirroring plugin.File semantics.
func loadExpected(dir string) map[string]string {
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	Expect(err).NotTo(HaveOccurred())
	return out
}

// responseFiles flattens a GenerateResponse into a map keyed by name.
func responseFiles(resp *plugin.GenerateResponse) map[string]string {
	out := map[string]string{}
	for _, f := range resp.Files {
		out[f.Name] = string(f.Contents)
	}
	return out
}

// fixtureRequest builds a reusable GenerateRequest mirroring a small
// postgres schema, then layers templates_dir into PluginOptions.
func fixtureRequest(templatesDir string, extra map[string]any) *plugin.GenerateRequest {
	opts := map[string]any{"templates_dir": templatesDir}
	if extra != nil {
		opts["extra"] = extra
	}
	raw, err := json.Marshal(opts)
	Expect(err).NotTo(HaveOccurred())

	return &plugin.GenerateRequest{
		SqlcVersion:   "v1.30.0",
		Settings:      &plugin.Settings{Engine: "postgresql"},
		PluginOptions: raw,
		Catalog: &plugin.Catalog{
			DefaultSchema: "public",
			Schemas: []*plugin.Schema{{
				Name: "public",
				Tables: []*plugin.Table{{
					Rel: &plugin.Identifier{Schema: "public", Name: "users"},
					Columns: []*plugin.Column{
						{Name: "id", Type: &plugin.Identifier{Name: "int4"}, NotNull: true},
						{Name: "email", Type: &plugin.Identifier{Name: "text"}, NotNull: true},
						{Name: "tags", Type: &plugin.Identifier{Name: "text"}, NotNull: true, IsArray: true},
						{Name: "bio", Type: &plugin.Identifier{Name: "text"}},
					},
				}},
			}},
		},
		Queries: []*plugin.Query{
			{Name: "GetUser", Cmd: ":one"},
			{Name: "ListUsers", Cmd: ":many"},
		},
	}
}

// sortedKeys returns map keys in lexical order for deterministic diffs.
func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

var _ = Describe("Plugin binary", func() {
	DescribeTable("renders testdata fixtures",
		func(fixture string, extra map[string]any) {
			root := filepath.Join("testdata", fixture)
			templates, err := filepath.Abs(filepath.Join(root, "templates"))
			Expect(err).NotTo(HaveOccurred())
			expectedDir, err := filepath.Abs(filepath.Join(root, "expected"))
			Expect(err).NotTo(HaveOccurred())

			resp := runPlugin(fixtureRequest(templates, extra))
			got := responseFiles(resp)
			want := loadExpected(expectedDir)

			Expect(sortedKeys(got)).To(Equal(sortedKeys(want)), "file list mismatch")
			for name := range want {
				Expect(got[name]).To(Equal(want[name]), "content mismatch for %s", name)
			}
		},
		Entry("basic single template", "basic", nil),
		Entry("multifile output", "multifile", nil),
		Entry("partials", "partials", nil),
		Entry("filename templating", "filename-templating", map[string]any{"package": "db"}),
		Entry("sprig smoke", "sprig", nil),
		Entry("language helpers", "language-helpers", nil),
	)

	It("exits non-zero on missing templates_dir", func() {
		req := &plugin.GenerateRequest{PluginOptions: []byte(`{}`)}
		out := runPluginExpectErr(req)
		Expect(out).To(ContainSubstring("templates_dir"))
	})
})
