package sqlc_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sqlc-contrib/sqlc-gen-template/internal/sqlc"
)

var _ = Describe("ParseOptions", func() {
	When("the payload is empty", func() {
		It("fails validation because templates_dir is required", func() {
			opts, err := sqlc.ParseOptions(nil)
			Expect(err).To(HaveOccurred())
			Expect(opts.TemplatesDir).To(BeEmpty())
		})
	})

	When("the payload is valid JSON", func() {
		It("decodes templates_dir and extra", func() {
			data := []byte(`{"templates_dir":"./templates","extra":{"package_name":"db","emit_json_tags":true}}`)
			opts, err := sqlc.ParseOptions(data)
			Expect(err).NotTo(HaveOccurred())
			Expect(opts.TemplatesDir).To(Equal("./templates"))
			Expect(opts.Extra).To(HaveKeyWithValue("package_name", "db"))
			Expect(opts.Extra).To(HaveKeyWithValue("emit_json_tags", true))
		})
	})

	When("the payload has an unknown field", func() {
		It("rejects it to catch typos", func() {
			data := []byte(`{"templates_dir":"./t","unknown":1}`)
			_, err := sqlc.ParseOptions(data)
			Expect(err).To(HaveOccurred())
		})
	})

	When("templates_dir is missing", func() {
		It("fails validation", func() {
			data := []byte(`{"extra":{"k":"v"}}`)
			_, err := sqlc.ParseOptions(data)
			Expect(err).To(MatchError(ContainSubstring("templates_dir")))
		})
	})

	When("the payload is not valid JSON", func() {
		It("returns a decode error", func() {
			_, err := sqlc.ParseOptions([]byte(`{not json`))
			Expect(err).To(HaveOccurred())
		})
	})
})
