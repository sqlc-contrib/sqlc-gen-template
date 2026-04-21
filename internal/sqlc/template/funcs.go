package template

import (
	"maps"
	"regexp"
	"strings"
	"sync"
	"text/template"
	"unicode"

	sprig "github.com/Masterminds/sprig/v3"
	"github.com/go-openapi/inflect"
	"github.com/sqlc-dev/plugin-sdk-go/plugin"
)

// FuncMap returns the template function map used by Render. Each call
// produces a fresh FuncMap because setStore/getStore close over a
// per-render map so cross-render state never leaks.
func FuncMap() template.FuncMap {
	funcs := sprig.TxtFuncMap()

	// Naming helpers. Our definitions take precedence over any sprig
	// overlap (sprig has snakecase/camelcase but with different rules).
	maps.Copy(funcs, namingFuncs())
	maps.Copy(funcs, navFuncs())
	maps.Copy(funcs, languageFuncs())

	// Per-render scratch store.
	var mu sync.Mutex
	store := map[string]any{}
	funcs["setStore"] = func(key string, value any) string {
		mu.Lock()
		defer mu.Unlock()
		store[key] = value
		return ""
	}
	funcs["getStore"] = func(key string) any {
		mu.Lock()
		defer mu.Unlock()
		return store[key]
	}

	return funcs
}

// ---------- naming --------------------------------------------------------

var acronyms = map[string]bool{
	"ID": true, "URL": true, "URI": true, "UUID": true, "API": true,
	"HTTP": true, "JSON": true, "XML": true, "SQL": true, "DB": true,
	"IP": true, "TCP": true, "UDP": true, "TLS": true, "SSL": true,
}

// splitWords breaks an arbitrary identifier (snake, kebab, camel, pascal,
// space- or punctuation-separated) into lowercase word tokens. Every
// non-alphanumeric rune acts as a separator.
func splitWords(s string) []string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}

	var out []string
	for chunk := range strings.SplitSeq(b.String(), "_") {
		if chunk == "" {
			continue
		}
		out = append(out, splitCamel(chunk)...)
	}
	return out
}

// splitCamel splits a single camelCase / PascalCase token into words.
func splitCamel(s string) []string {
	var words []string
	runes := []rune(s)
	start := 0
	for i := 1; i < len(runes); i++ {
		prev, cur := runes[i-1], runes[i]
		boundary := false
		switch {
		case unicode.IsLower(prev) && unicode.IsUpper(cur):
			boundary = true
		case unicode.IsUpper(prev) && unicode.IsUpper(cur) && i+1 < len(runes) && unicode.IsLower(runes[i+1]):
			boundary = true
		case unicode.IsDigit(prev) != unicode.IsDigit(cur) && cur != '_':
			boundary = true
		}
		if boundary {
			words = append(words, strings.ToLower(string(runes[start:i])))
			start = i
		}
	}
	if start < len(runes) {
		words = append(words, strings.ToLower(string(runes[start:])))
	}
	return words
}

func pascalCase(s string) string {
	var b strings.Builder
	for _, w := range splitWords(s) {
		upper := strings.ToUpper(w)
		if acronyms[upper] {
			b.WriteString(upper)
			continue
		}
		b.WriteString(strings.ToUpper(w[:1]))
		b.WriteString(w[1:])
	}
	return b.String()
}

func camelCase(s string) string {
	words := splitWords(s)
	if len(words) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(words[0])
	for _, w := range words[1:] {
		upper := strings.ToUpper(w)
		if acronyms[upper] {
			b.WriteString(upper)
			continue
		}
		b.WriteString(strings.ToUpper(w[:1]))
		b.WriteString(w[1:])
	}
	return b.String()
}

func snakeCase(s string) string     { return strings.Join(splitWords(s), "_") }
func kebabCase(s string) string     { return strings.Join(splitWords(s), "-") }
func screamingSnake(s string) string { return strings.ToUpper(snakeCase(s)) }

func upperFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// goNormalize produces an exported Go identifier that preserves common
// acronyms (ID, URL, …). Non-identifier characters are replaced with
// underscores; a leading digit is prefixed with "_".
var goNormalizeInvalid = regexp.MustCompile(`[^A-Za-z0-9]+`)

func goNormalize(s string) string {
	id := pascalCase(s)
	id = goNormalizeInvalid.ReplaceAllString(id, "")
	if id == "" {
		return "_"
	}
	if unicode.IsDigit(rune(id[0])) {
		return "_" + id
	}
	return id
}

func namingFuncs() template.FuncMap {
	return template.FuncMap{
		"singular":       inflect.Singularize,
		"plural":         inflect.Pluralize,
		"camelize":       inflect.Camelize,
		"camelCase":      camelCase,
		"pascalCase":     pascalCase,
		"snakeCase":      snakeCase,
		"kebabCase":      kebabCase,
		"screamingSnake": screamingSnake,
		"upperFirst":     upperFirst,
		"lowerFirst":     lowerFirst,
		"goNormalize":    goNormalize,
	}
}

// ---------- proto navigation ---------------------------------------------

func navFuncs() template.FuncMap {
	return template.FuncMap{
		"findTable": func(req *plugin.GenerateRequest, schemaName, tableName string) *plugin.Table {
			if req == nil || req.Catalog == nil {
				return nil
			}
			for _, s := range req.Catalog.Schemas {
				if schemaName != "" && s.Name != schemaName {
					continue
				}
				for _, t := range s.Tables {
					if t.Rel != nil && t.Rel.Name == tableName {
						return t
					}
				}
			}
			return nil
		},
		"findEnum": func(req *plugin.GenerateRequest, schemaName, enumName string) *plugin.Enum {
			if req == nil || req.Catalog == nil {
				return nil
			}
			for _, s := range req.Catalog.Schemas {
				if schemaName != "" && s.Name != schemaName {
					continue
				}
				for _, e := range s.Enums {
					if e.Name == enumName {
						return e
					}
				}
			}
			return nil
		},
		"queriesByCmd": func(cmd string, queries []*plugin.Query) []*plugin.Query {
			var out []*plugin.Query
			for _, q := range queries {
				if q.Cmd == cmd {
					out = append(out, q)
				}
			}
			return out
		},
		"hasColumn": func(table *plugin.Table, columnName string) bool {
			if table == nil {
				return false
			}
			for _, c := range table.Columns {
				if c.Name == columnName {
					return true
				}
			}
			return false
		},
		"columnComment": func(col *plugin.Column) string {
			if col == nil {
				return ""
			}
			return strings.TrimSpace(strings.TrimPrefix(col.Comment, "--"))
		},
		"option": func(key string, options map[string]any) any {
			if options == nil {
				return nil
			}
			return options[key]
		},
		"optionOr": func(key string, fallback any, options map[string]any) any {
			if options == nil {
				return fallback
			}
			if v, ok := options[key]; ok {
				return v
			}
			return fallback
		},
	}
}

// ---------- language type helpers ----------------------------------------

func languageFuncs() template.FuncMap {
	return template.FuncMap{
		"goType":      goType,
		"goZeroValue": goZeroValue,
		"pyType":      pyType,
		"tsType":      tsType,
		"rustType":    rustType,
		"kotlinType":  kotlinType,
		"cppType":     cppType,
	}
}

// columnType extracts the lowercase database type name from a column,
// defensive against nil identifiers.
func columnType(col *plugin.Column) string {
	if col == nil || col.Type == nil {
		return ""
	}
	return strings.ToLower(col.Type.Name)
}

// wrap applies nullable and array decorations around a base type using
// the rules passed in via w.
type wrap struct {
	array    func(string) string
	nullable func(string) string
}

func (w wrap) apply(col *plugin.Column, base string) string {
	out := base
	if col != nil && col.IsArray && w.array != nil {
		out = w.array(out)
	}
	if col != nil && !col.NotNull && w.nullable != nil {
		out = w.nullable(out)
	}
	return out
}

// --- Go ---

func goType(col *plugin.Column) string {
	base := goBase(col)
	return wrap{
		array:    func(s string) string { return "[]" + s },
		nullable: func(s string) string {
			// Pointer wrap unless the type is already a *pointer/slice/map.
			if strings.HasPrefix(s, "[]") || strings.HasPrefix(s, "*") || strings.HasPrefix(s, "map[") {
				return s
			}
			return "*" + s
		},
	}.apply(col, base)
}

func goBase(col *plugin.Column) string {
	t := columnType(col)
	switch t {
	case "bool", "boolean":
		return "bool"
	case "int2", "smallint", "smallserial":
		return "int16"
	case "int4", "integer", "int", "serial", "mediumint":
		if col != nil && col.Unsigned {
			return "uint32"
		}
		return "int32"
	case "int8", "bigint", "bigserial":
		if col != nil && col.Unsigned {
			return "uint64"
		}
		return "int64"
	case "tinyint":
		if col != nil && col.Unsigned {
			return "uint8"
		}
		return "int8"
	case "float4", "real":
		return "float32"
	case "float8", "double precision", "double", "decimal", "numeric":
		return "float64"
	case "text", "varchar", "char", "character varying", "citext", "bpchar", "name":
		return "string"
	case "uuid":
		return "string"
	case "bytea", "blob", "bytes", "binary", "varbinary":
		return "[]byte"
	case "date", "time", "timestamp", "timestamptz", "timestamp with time zone", "timestamp without time zone", "datetime":
		return "time.Time"
	case "json", "jsonb":
		return "json.RawMessage"
	}
	return "any"
}

func goZeroValue(col *plugin.Column) string {
	if col != nil && !col.NotNull {
		return "nil"
	}
	if col != nil && col.IsArray {
		return "nil"
	}
	switch goBase(col) {
	case "bool":
		return "false"
	case "int8", "int16", "int32", "int64", "uint8", "uint32", "uint64", "float32", "float64":
		return "0"
	case "string":
		return `""`
	case "[]byte":
		return "nil"
	case "time.Time":
		return "time.Time{}"
	case "json.RawMessage":
		return "nil"
	}
	return "nil"
}

// --- Python ---

func pyType(col *plugin.Column) string {
	base := pyBase(col)
	return wrap{
		array:    func(s string) string { return "List[" + s + "]" },
		nullable: func(s string) string { return "Optional[" + s + "]" },
	}.apply(col, base)
}

func pyBase(col *plugin.Column) string {
	switch columnType(col) {
	case "bool", "boolean":
		return "bool"
	case "int2", "int4", "int8", "smallint", "integer", "int", "bigint", "serial", "bigserial", "smallserial", "tinyint", "mediumint":
		return "int"
	case "float4", "float8", "real", "double precision", "double", "numeric", "decimal":
		return "float"
	case "text", "varchar", "char", "character varying", "citext", "bpchar", "name", "uuid":
		return "str"
	case "bytea", "blob", "bytes", "binary", "varbinary":
		return "bytes"
	case "date":
		return "datetime.date"
	case "time":
		return "datetime.time"
	case "timestamp", "timestamptz", "timestamp with time zone", "timestamp without time zone", "datetime":
		return "datetime.datetime"
	case "json", "jsonb":
		return "Any"
	}
	return "Any"
}

// --- TypeScript ---

func tsType(col *plugin.Column) string {
	base := tsBase(col)
	return wrap{
		array:    func(s string) string { return s + "[]" },
		nullable: func(s string) string { return s + " | null" },
	}.apply(col, base)
}

func tsBase(col *plugin.Column) string {
	switch columnType(col) {
	case "bool", "boolean":
		return "boolean"
	case "int2", "int4", "int8", "smallint", "integer", "int", "bigint", "serial", "bigserial", "smallserial", "tinyint", "mediumint",
		"float4", "float8", "real", "double precision", "double", "numeric", "decimal":
		return "number"
	case "text", "varchar", "char", "character varying", "citext", "bpchar", "name", "uuid":
		return "string"
	case "bytea", "blob", "bytes", "binary", "varbinary":
		return "Uint8Array"
	case "date", "time", "timestamp", "timestamptz", "timestamp with time zone", "timestamp without time zone", "datetime":
		return "Date"
	case "json", "jsonb":
		return "unknown"
	}
	return "unknown"
}

// --- Rust ---

func rustType(col *plugin.Column) string {
	base := rustBase(col)
	return wrap{
		array:    func(s string) string { return "Vec<" + s + ">" },
		nullable: func(s string) string { return "Option<" + s + ">" },
	}.apply(col, base)
}

func rustBase(col *plugin.Column) string {
	switch columnType(col) {
	case "bool", "boolean":
		return "bool"
	case "int2", "smallint", "smallserial":
		return "i16"
	case "int4", "integer", "int", "serial", "mediumint":
		if col != nil && col.Unsigned {
			return "u32"
		}
		return "i32"
	case "int8", "bigint", "bigserial":
		if col != nil && col.Unsigned {
			return "u64"
		}
		return "i64"
	case "tinyint":
		if col != nil && col.Unsigned {
			return "u8"
		}
		return "i8"
	case "float4", "real":
		return "f32"
	case "float8", "double precision", "double", "numeric", "decimal":
		return "f64"
	case "text", "varchar", "char", "character varying", "citext", "bpchar", "name":
		return "String"
	case "uuid":
		return "uuid::Uuid"
	case "bytea", "blob", "bytes", "binary", "varbinary":
		return "Vec<u8>"
	case "date":
		return "chrono::NaiveDate"
	case "time":
		return "chrono::NaiveTime"
	case "timestamp", "timestamp without time zone", "datetime":
		return "chrono::NaiveDateTime"
	case "timestamptz", "timestamp with time zone":
		return "chrono::DateTime<chrono::Utc>"
	case "json", "jsonb":
		return "serde_json::Value"
	}
	return "serde_json::Value"
}

// --- Kotlin ---

func kotlinType(col *plugin.Column) string {
	base := kotlinBase(col)
	return wrap{
		array:    func(s string) string { return "List<" + s + ">" },
		nullable: func(s string) string { return s + "?" },
	}.apply(col, base)
}

func kotlinBase(col *plugin.Column) string {
	switch columnType(col) {
	case "bool", "boolean":
		return "Boolean"
	case "int2", "smallint", "smallserial":
		return "Short"
	case "int4", "integer", "int", "serial", "mediumint":
		return "Int"
	case "int8", "bigint", "bigserial":
		return "Long"
	case "tinyint":
		return "Byte"
	case "float4", "real":
		return "Float"
	case "float8", "double precision", "double":
		return "Double"
	case "numeric", "decimal":
		return "java.math.BigDecimal"
	case "text", "varchar", "char", "character varying", "citext", "bpchar", "name":
		return "String"
	case "uuid":
		return "java.util.UUID"
	case "bytea", "blob", "bytes", "binary", "varbinary":
		return "ByteArray"
	case "date":
		return "java.time.LocalDate"
	case "time":
		return "java.time.LocalTime"
	case "timestamp", "timestamp without time zone", "datetime":
		return "java.time.LocalDateTime"
	case "timestamptz", "timestamp with time zone":
		return "java.time.OffsetDateTime"
	case "json", "jsonb":
		return "String"
	}
	return "Any"
}

// --- C++ ---

func cppType(col *plugin.Column) string {
	base := cppBase(col)
	return wrap{
		array:    func(s string) string { return "std::vector<" + s + ">" },
		nullable: func(s string) string { return "std::optional<" + s + ">" },
	}.apply(col, base)
}

func cppBase(col *plugin.Column) string {
	switch columnType(col) {
	case "bool", "boolean":
		return "bool"
	case "int2", "smallint", "smallserial":
		return "int16_t"
	case "int4", "integer", "int", "serial", "mediumint":
		if col != nil && col.Unsigned {
			return "uint32_t"
		}
		return "int32_t"
	case "int8", "bigint", "bigserial":
		if col != nil && col.Unsigned {
			return "uint64_t"
		}
		return "int64_t"
	case "tinyint":
		if col != nil && col.Unsigned {
			return "uint8_t"
		}
		return "int8_t"
	case "float4", "real":
		return "float"
	case "float8", "double precision", "double", "numeric", "decimal":
		return "double"
	case "text", "varchar", "char", "character varying", "citext", "bpchar", "name", "uuid":
		return "std::string"
	case "bytea", "blob", "bytes", "binary", "varbinary":
		return "std::vector<uint8_t>"
	case "date", "time", "timestamp", "timestamptz", "timestamp with time zone", "timestamp without time zone", "datetime":
		return "std::chrono::system_clock::time_point"
	case "json", "jsonb":
		return "std::string"
	}
	return "std::string"
}
