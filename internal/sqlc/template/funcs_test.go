package template_test

import (
	"bytes"
	tt "text/template"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sqlc-dev/plugin-sdk-go/plugin"

	"github.com/sqlc-contrib/sqlc-gen-template/internal/sqlc/template"
)

// execFunc applies a single FuncMap entry via a one-off template render.
// It keeps assertions focused on the textual output a user would see.
func execFunc(body string, data any) string {
	t, err := tt.New("t").Funcs(template.FuncMap()).Parse(body)
	Expect(err).NotTo(HaveOccurred())
	var buf bytes.Buffer
	Expect(t.Execute(&buf, data)).To(Succeed())
	return buf.String()
}

var _ = Describe("Naming helpers", func() {
	DescribeTable("case conversions",
		func(body, input, want string) {
			Expect(execFunc(body, input)).To(Equal(want))
		},
		Entry("camelCase from snake", `{{ camelCase . }}`, "user_id", "userID"),
		Entry("camelCase from kebab", `{{ camelCase . }}`, "user-name", "userName"),
		Entry("pascalCase preserves acronym", `{{ pascalCase . }}`, "user_url", "UserURL"),
		Entry("pascalCase from camel", `{{ pascalCase . }}`, "userID", "UserID"),
		Entry("snakeCase from pascal", `{{ snakeCase . }}`, "UserID", "user_id"),
		Entry("kebabCase from snake", `{{ kebabCase . }}`, "user_name", "user-name"),
		Entry("screamingSnake from pascal", `{{ screamingSnake . }}`, "UserName", "USER_NAME"),
		Entry("upperFirst", `{{ upperFirst . }}`, "hello", "Hello"),
		Entry("upperFirst empty", `{{ upperFirst . }}`, "", ""),
		Entry("lowerFirst", `{{ lowerFirst . }}`, "Hello", "hello"),
		Entry("lowerFirst empty", `{{ lowerFirst . }}`, "", ""),
		Entry("goNormalize empty", `{{ goNormalize . }}`, "___", "_"),
		Entry("goNormalize handles leading digit", `{{ goNormalize . }}`, "1abc", "_1Abc"),
		Entry("goNormalize strips non-ident", `{{ goNormalize . }}`, "foo.bar-baz", "FooBarBaz"),
		Entry("singular", `{{ singular . }}`, "users", "user"),
		Entry("plural", `{{ plural . }}`, "user", "users"),
	)
})

var _ = Describe("Proto navigation helpers", func() {
	req := &plugin.GenerateRequest{
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
				Enums: []*plugin.Enum{{Name: "role", Vals: []string{"admin", "user"}}},
			}},
		},
		Queries: []*plugin.Query{
			{Name: "GetUser", Cmd: ":one"},
			{Name: "ListUsers", Cmd: ":many"},
			{Name: "CountUsers", Cmd: ":one"},
		},
	}

	It("findTable returns the named table", func() {
		out := execFunc(
			`{{ $t := findTable .Request "public" "users" }}{{ $t.Rel.Name }}`,
			template.Context{Request: req},
		)
		Expect(out).To(Equal("users"))
	})

	It("findTable returns nil for missing table", func() {
		out := execFunc(
			`{{ if findTable .Request "public" "missing" }}yes{{ else }}no{{ end }}`,
			template.Context{Request: req},
		)
		Expect(out).To(Equal("no"))
	})

	It("findEnum returns the named enum", func() {
		out := execFunc(
			`{{ $e := findEnum .Request "" "role" }}{{ len $e.Vals }}`,
			template.Context{Request: req},
		)
		Expect(out).To(Equal("2"))
	})

	It("queriesByCmd filters queries by command", func() {
		out := execFunc(
			`{{ len (queriesByCmd ":one" .Request.Queries) }}`,
			template.Context{Request: req},
		)
		Expect(out).To(Equal("2"))
	})

	It("hasColumn detects presence", func() {
		out := execFunc(
			`{{ $t := findTable .Request "public" "users" }}{{ hasColumn $t "email" }}-{{ hasColumn $t "missing" }}`,
			template.Context{Request: req},
		)
		Expect(out).To(Equal("true-false"))
	})

	It("optionOr returns fallback when key missing", func() {
		out := execFunc(
			`{{ optionOr "nope" "default" .Options }}`,
			template.Context{Options: map[string]any{"other": 1}},
		)
		Expect(out).To(Equal("default"))
	})
})

var _ = Describe("Scratch store", func() {
	It("round-trips values within a single render", func() {
		t, err := tt.New("s").Funcs(template.FuncMap()).Parse(
			`{{ setStore "x" 42 }}{{ getStore "x" }}`,
		)
		Expect(err).NotTo(HaveOccurred())
		var buf bytes.Buffer
		Expect(t.Execute(&buf, nil)).To(Succeed())
		Expect(buf.String()).To(Equal("42"))
	})

	It("isolates state between FuncMap instances", func() {
		a := template.FuncMap()
		b := template.FuncMap()
		// Set in a; b must not see it.
		ta, err := tt.New("a").Funcs(a).Parse(`{{ setStore "k" "v" }}{{ getStore "k" }}`)
		Expect(err).NotTo(HaveOccurred())
		tb, err := tt.New("b").Funcs(b).Parse(`{{ getStore "k" }}`)
		Expect(err).NotTo(HaveOccurred())
		var bufA, bufB bytes.Buffer
		Expect(ta.Execute(&bufA, nil)).To(Succeed())
		Expect(tb.Execute(&bufB, nil)).To(Succeed())
		Expect(bufA.String()).To(Equal("v"))
		Expect(bufB.String()).To(Equal("<no value>"))
	})
})

var _ = Describe("Sprig integration", func() {
	It("exposes sprig funcs alongside ours", func() {
		Expect(execFunc(`{{ upper "abc" }}`, nil)).To(Equal("ABC"))
		Expect(execFunc(`{{ list 1 2 3 | join "," }}`, nil)).To(Equal("1,2,3"))
	})
})

var _ = Describe("Language type helpers", func() {
	col := func(typeName string, notNull, isArray bool) *plugin.Column {
		return &plugin.Column{
			Name:    "c",
			Type:    &plugin.Identifier{Name: typeName},
			NotNull: notNull,
			IsArray: isArray,
		}
	}

	DescribeTable("goType",
		func(c *plugin.Column, want string) {
			out := execFunc(`{{ goType . }}`, c)
			Expect(out).To(Equal(want))
		},
		Entry("bool", col("bool", true, false), "bool"),
		Entry("smallint", col("smallint", true, false), "int16"),
		Entry("int4 non-null", col("int4", true, false), "int32"),
		Entry("int4 nullable", col("int4", false, false), "*int32"),
		Entry("bigint", col("bigint", true, false), "int64"),
		Entry("bigint unsigned", &plugin.Column{Type: &plugin.Identifier{Name: "bigint"}, NotNull: true, Unsigned: true}, "uint64"),
		Entry("tinyint unsigned", &plugin.Column{Type: &plugin.Identifier{Name: "tinyint"}, NotNull: true, Unsigned: true}, "uint8"),
		Entry("float4", col("float4", true, false), "float32"),
		Entry("numeric", col("numeric", true, false), "float64"),
		Entry("text array", col("text", true, true), "[]string"),
		Entry("nullable text array stays slice", col("text", false, true), "[]string"),
		Entry("timestamptz", col("timestamptz", true, false), "time.Time"),
		Entry("date", col("date", true, false), "time.Time"),
		Entry("uuid", col("uuid", true, false), "string"),
		Entry("bytea", col("bytea", true, false), "[]byte"),
		Entry("jsonb", col("jsonb", true, false), "json.RawMessage"),
		Entry("nil column falls back", (*plugin.Column)(nil), "any"),
		Entry("unknown falls back", col("geometry", true, false), "any"),
	)

	DescribeTable("pyType",
		func(c *plugin.Column, want string) {
			Expect(execFunc(`{{ pyType . }}`, c)).To(Equal(want))
		},
		Entry("bool", col("bool", true, false), "bool"),
		Entry("int nullable", col("int4", false, false), "Optional[int]"),
		Entry("float", col("float8", true, false), "float"),
		Entry("text array non-null", col("text", true, true), "List[str]"),
		Entry("bytea", col("bytea", true, false), "bytes"),
		Entry("date", col("date", true, false), "datetime.date"),
		Entry("time", col("time", true, false), "datetime.time"),
		Entry("jsonb", col("jsonb", true, false), "Any"),
		Entry("timestamptz", col("timestamptz", true, false), "datetime.datetime"),
		Entry("unknown", col("geometry", true, false), "Any"),
	)

	DescribeTable("tsType",
		func(c *plugin.Column, want string) {
			Expect(execFunc(`{{ tsType . }}`, c)).To(Equal(want))
		},
		Entry("bool", col("bool", true, false), "boolean"),
		Entry("int non-null", col("int4", true, false), "number"),
		Entry("text nullable", col("text", false, false), "string | null"),
		Entry("int array", col("int4", true, true), "number[]"),
		Entry("uuid", col("uuid", true, false), "string"),
		Entry("bytea", col("bytea", true, false), "Uint8Array"),
		Entry("timestamptz", col("timestamptz", true, false), "Date"),
		Entry("jsonb", col("jsonb", true, false), "unknown"),
		Entry("unknown", col("geometry", true, false), "unknown"),
	)

	DescribeTable("rustType",
		func(c *plugin.Column, want string) {
			Expect(execFunc(`{{ rustType . }}`, c)).To(Equal(want))
		},
		Entry("bool", col("bool", true, false), "bool"),
		Entry("smallint", col("smallint", true, false), "i16"),
		Entry("int non-null", col("int4", true, false), "i32"),
		Entry("int unsigned", &plugin.Column{Type: &plugin.Identifier{Name: "int4"}, NotNull: true, Unsigned: true}, "u32"),
		Entry("bigint", col("bigint", true, false), "i64"),
		Entry("tinyint", col("tinyint", true, false), "i8"),
		Entry("float4", col("float4", true, false), "f32"),
		Entry("float8", col("float8", true, false), "f64"),
		Entry("text nullable", col("text", false, false), "Option<String>"),
		Entry("uuid", col("uuid", true, false), "uuid::Uuid"),
		Entry("bytea", col("bytea", true, false), "Vec<u8>"),
		Entry("date", col("date", true, false), "chrono::NaiveDate"),
		Entry("time", col("time", true, false), "chrono::NaiveTime"),
		Entry("timestamp", col("timestamp", true, false), "chrono::NaiveDateTime"),
		Entry("timestamptz", col("timestamptz", true, false), "chrono::DateTime<chrono::Utc>"),
		Entry("jsonb", col("jsonb", true, false), "serde_json::Value"),
		Entry("unknown falls back", col("geometry", true, false), "serde_json::Value"),
	)

	DescribeTable("kotlinType",
		func(c *plugin.Column, want string) {
			Expect(execFunc(`{{ kotlinType . }}`, c)).To(Equal(want))
		},
		Entry("bool", col("bool", true, false), "Boolean"),
		Entry("smallint", col("smallint", true, false), "Short"),
		Entry("int non-null", col("int4", true, false), "Int"),
		Entry("bigint", col("bigint", true, false), "Long"),
		Entry("tinyint", col("tinyint", true, false), "Byte"),
		Entry("float4", col("float4", true, false), "Float"),
		Entry("float8", col("float8", true, false), "Double"),
		Entry("numeric", col("numeric", true, false), "java.math.BigDecimal"),
		Entry("text nullable", col("text", false, false), "String?"),
		Entry("uuid array", col("uuid", true, true), "List<java.util.UUID>"),
		Entry("bytea", col("bytea", true, false), "ByteArray"),
		Entry("date", col("date", true, false), "java.time.LocalDate"),
		Entry("time", col("time", true, false), "java.time.LocalTime"),
		Entry("timestamp", col("timestamp", true, false), "java.time.LocalDateTime"),
		Entry("timestamptz", col("timestamptz", true, false), "java.time.OffsetDateTime"),
		Entry("jsonb", col("jsonb", true, false), "String"),
		Entry("unknown falls back", col("geometry", true, false), "Any"),
	)

	DescribeTable("cppType",
		func(c *plugin.Column, want string) {
			Expect(execFunc(`{{ cppType . }}`, c)).To(Equal(want))
		},
		Entry("bool", col("bool", true, false), "bool"),
		Entry("smallint", col("smallint", true, false), "int16_t"),
		Entry("int non-null", col("int4", true, false), "int32_t"),
		Entry("int unsigned", &plugin.Column{Type: &plugin.Identifier{Name: "int4"}, NotNull: true, Unsigned: true}, "uint32_t"),
		Entry("bigint", col("bigint", true, false), "int64_t"),
		Entry("bigint unsigned", &plugin.Column{Type: &plugin.Identifier{Name: "bigint"}, NotNull: true, Unsigned: true}, "uint64_t"),
		Entry("tinyint", col("tinyint", true, false), "int8_t"),
		Entry("tinyint unsigned", &plugin.Column{Type: &plugin.Identifier{Name: "tinyint"}, NotNull: true, Unsigned: true}, "uint8_t"),
		Entry("float4", col("float4", true, false), "float"),
		Entry("float8", col("float8", true, false), "double"),
		Entry("text nullable", col("text", false, false), "std::optional<std::string>"),
		Entry("bytea", col("bytea", true, false), "std::vector<uint8_t>"),
		Entry("timestamptz", col("timestamptz", true, false), "std::chrono::system_clock::time_point"),
		Entry("jsonb", col("jsonb", true, false), "std::string"),
		Entry("unknown falls back", col("geometry", true, false), "std::string"),
	)

	DescribeTable("goZeroValue",
		func(c *plugin.Column, want string) {
			Expect(execFunc(`{{ goZeroValue . }}`, c)).To(Equal(want))
		},
		Entry("nullable is nil", col("int4", false, false), "nil"),
		Entry("array is nil", col("int4", true, true), "nil"),
		Entry("bool is false", col("bool", true, false), "false"),
		Entry("int is 0", col("int4", true, false), "0"),
		Entry("text is empty string", col("text", true, false), `""`),
		Entry("bytea is nil", col("bytea", true, false), "nil"),
		Entry("jsonb is nil", col("jsonb", true, false), "nil"),
		Entry("time is zero struct", col("timestamptz", true, false), "time.Time{}"),
		Entry("unknown is nil", col("geometry", true, false), "nil"),
	)
})
