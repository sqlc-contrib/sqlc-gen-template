# sqlc-gen-template

[![CI](https://github.com/sqlc-contrib/sqlc-gen-template/actions/workflows/ci.yml/badge.svg)](https://github.com/sqlc-contrib/sqlc-gen-template/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/sqlc-contrib/sqlc-gen-template?include_prereleases)](https://github.com/sqlc-contrib/sqlc-gen-template/releases)
[![License](https://img.shields.io/github/license/sqlc-contrib/sqlc-gen-template)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![sqlc](https://img.shields.io/badge/sqlc-compatible-blue)](https://sqlc.dev)

A [sqlc](https://sqlc.dev) plugin that renders arbitrary code from
user-supplied Go `text/template` files. Point it at a templates directory and
it will render each `*.tmpl` file against sqlc's parsed catalog and queries.

A better equivalent of
[fdietze/sqlc-gen-from-template](https://github.com/fdietze/sqlc-gen-from-template)
with the ergonomics of
[protoc-contrib/protoc-gen-template](https://github.com/protoc-contrib/protoc-gen-template):
sprig functions, naming helpers, language type mappers, filename templating,
partials, and cross-file includes.

## Features

- Runs as a **process plugin** (native binary, reads templates from disk) or a
  **WASM plugin** (sandboxed, no filesystem access — templates must be on disk
  via process mode)
- Directory walk discovers every `*.tmpl` under `template_dir`
- Filename templating — the output path is itself rendered as a template
- Partials — files whose base name starts with `_` are parsed but not emitted
- Cross-file `{{ template "name" . }}` includes
- [Sprig v3](https://masterminds.github.io/sprig/) function library
- Naming helpers: `camelCase`, `pascalCase`, `snakeCase`, `kebabCase`,
  `screamingSnake`, `singular`, `plural`, `camelize`, `goNormalize`, …
- Language type helpers: `goType`, `pyType`, `tsType`, `rustType`, `kotlinType`,
  `cppType`, `goZeroValue`
- Proto navigation: `findTable`, `findEnum`, `queriesByCmd`, `hasColumn`,
  `columnComment`, `option`, `optionOr`
- Per-render scratch store: `setStore` / `getStore`
- Language-agnostic — template data is the raw sqlc protobuf; no
  pre-computed enriched model

## Installation

### Process mode (recommended — required for `template_dir`)

sqlc's WASM sandbox has **no filesystem access**, so reading templates from disk
requires running the plugin as a native process.

**Go projects** — add the binary as a `go tool` dependency:

```bash
go get -tool github.com/sqlc-contrib/sqlc-gen-template/cmd/sqlc-gen-template@latest
```

Then in `sqlc.yaml`:

```yaml
version: "2"
plugins:
  - name: template
    process:
      cmd: "go tool sqlc-gen-template"
```

**Other projects** — install the binary with Go and reference it by path:

```bash
go install github.com/sqlc-contrib/sqlc-gen-template/cmd/sqlc-gen-template@latest
```

```yaml
version: "2"
plugins:
  - name: template
    process:
      cmd: "sqlc-gen-template"
```

### WASM mode

WASM mode is supported for distribution convenience (no local binary needed),
but because sqlc's WASM sandbox provides no filesystem access, the plugin
**cannot read template files from disk** in this mode. `template_dir` will
always fail with `EBADF`. Only use WASM mode if you have a use case that does
not require disk-based templates.

```yaml
version: "2"
plugins:
  - name: template
    wasm:
      url: https://github.com/sqlc-contrib/sqlc-gen-template/releases/download/v0.1.1/sqlc-gen-template.wasm
      sha256: <sha256 from the release assets>
```

## Configuration

```yaml
sql:
  - engine: postgresql
    schema: db/schema.sql
    queries: db/queries.sql
    codegen:
      - plugin: template
        out: ./gen
        options:
          template_dir: ./templates # required
          extra: # free-form; surfaced as .Options
            package: db
            emit_json_tags: true
```

| Option          | Type   | Required | Description                                                   |
| --------------- | ------ | :------: | ------------------------------------------------------------- |
| `template_dir` | string |   yes    | Directory (relative to `sqlc.yaml`) walked for `*.tmpl` files |
| `extra`         | object |    no    | Arbitrary key/value map surfaced to templates as `.Options`   |

Unknown top-level options are rejected.

## Template discovery & output

- Every file under `template_dir` with a `.tmpl` suffix is parsed.
- All templates are loaded into a single template set, so
  `{{ template "some-other-file.tmpl" . }}` works across files.
- Files whose **base name** starts with `_` are partials — parsed but never
  emitted as output files.
- For each non-partial template, the output path is computed as the template's
  path relative to `template_dir`, with the `.tmpl` suffix stripped. That
  path is _itself_ executed as a template, so it can depend on `.Options`,
  range contexts, etc.

Example layout:

```
templates/
  _header.tmpl                              # partial (not emitted)
  models.go.tmpl                            # → gen/models.go
  {{ .Options.package }}/schema.sql.tmpl    # → gen/db/schema.sql  (when .Options.package = "db")
```

## Template context

The root value (`.`) is:

```go
type Context struct {
    Request      *plugin.GenerateRequest // raw sqlc protobuf
    Options      map[string]any          // the `extra` map from sqlc.yaml
    TemplateDir string                  // the resolved templates directory
    SqlcVersion  string                  // hoisted from Request.SqlcVersion
}
```

Useful proto field paths
(see the
[plugin-sdk-go types](https://pkg.go.dev/github.com/sqlc-dev/plugin-sdk-go/plugin)
for the full surface):

| Path                                                     | Meaning                                 |
| -------------------------------------------------------- | --------------------------------------- |
| `.Request.Settings.Engine`                               | `"postgresql"` / `"mysql"` / `"sqlite"` |
| `.Request.Catalog.DefaultSchema`                         | Default schema name                     |
| `.Request.Catalog.Schemas`                               | `[]*plugin.Schema`                      |
| `.Request.Catalog.Schemas[i].Tables`                     | `[]*plugin.Table`                       |
| `.Request.Catalog.Schemas[i].Tables[j].Columns`          | `[]*plugin.Column`                      |
| `.Request.Catalog.Schemas[i].Enums`                      | `[]*plugin.Enum`                        |
| `.Request.Queries`                                       | `[]*plugin.Query`                       |
| `.Request.Queries[i].Cmd`                                | `":one"`, `":many"`, `":exec"`, …       |
| `.Request.Queries[i].Params[k].Column`                   | `*plugin.Column`                        |
| `.Column.Type.Name`                                      | Database type name (e.g. `int4`)        |
| `.Column.NotNull`, `.Column.IsArray`, `.Column.Unsigned` | Column flags                            |

## Template function reference

### Naming

| Function           | Example input → output                    |
| ------------------ | ----------------------------------------- |
| `camelCase s`      | `user_id` → `userID`                      |
| `pascalCase s`     | `user_id` → `UserID`                      |
| `snakeCase s`      | `UserID` → `user_id`                      |
| `kebabCase s`      | `UserID` → `user-id`                      |
| `screamingSnake s` | `UserID` → `USER_ID`                      |
| `upperFirst s`     | `foo` → `Foo`                             |
| `lowerFirst s`     | `Foo` → `foo`                             |
| `goNormalize s`    | `1st_user-id` → `_1stUserID`              |
| `singular s`       | `users` → `user` (via go-openapi/inflect) |
| `plural s`         | `user` → `users`                          |
| `camelize s`       | `user_id` → `UserId`                      |

`camelCase`, `pascalCase`, `goNormalize` preserve common acronyms
(`ID`, `URL`, `URI`, `UUID`, `API`, `HTTP`, `JSON`, `XML`, `SQL`, `DB`, `IP`,
`TCP`, `UDP`, `TLS`, `SSL`).

### Proto navigation

| Function                        | Description                                                   |
| ------------------------------- | ------------------------------------------------------------- |
| `findTable req schema name`     | Returns the named `*plugin.Table` (empty schema = any schema) |
| `findEnum req schema name`      | Returns the named `*plugin.Enum`                              |
| `queriesByCmd cmd queries`      | Filters queries by `Cmd` (e.g. `":one"`)                      |
| `hasColumn table name`          | Reports whether `table` has a column named `name`             |
| `columnComment col`             | Returns the column's comment, stripped of the leading `--`    |
| `option key options`            | Looks up a key in the `extra` map (returns `nil` if missing)  |
| `optionOr key fallback options` | Like `option`, but returns `fallback` when the key is missing |

Example:

```gotemplate
{{ $users := findTable .Request "public" "users" }}
{{ range $users.Columns }}{{ .Name }}: {{ goType . }}
{{ end }}
```

### Language type helpers

Each helper takes a single `*plugin.Column` and returns the column's type in
the target language, honouring `NotNull`, `IsArray`, and (where meaningful)
`Unsigned`.

| Helper            | Returns         | Array wrap       | Nullable wrap      |
| ----------------- | --------------- | ---------------- | ------------------ |
| `goType col`      | Go type         | `[]T`            | `*T` (scalars)     |
| `goZeroValue col` | Go zero value   | —                | `nil`              |
| `pyType col`      | Python type     | `List[T]`        | `Optional[T]`      |
| `tsType col`      | TypeScript type | `T[]`            | `T \| null`        |
| `rustType col`    | Rust type       | `Vec<T>`         | `Option<T>`        |
| `kotlinType col`  | Kotlin type     | `List<T>`        | `T?`               |
| `cppType col`     | C++ type        | `std::vector<T>` | `std::optional<T>` |

Covered SQL types include `bool`, `int2`/`int4`/`int8` (and their `smallint`/
`integer`/`bigint`/serial aliases), `tinyint`/`mediumint`, `float4`/`float8`,
`numeric`/`decimal`, `text`/`varchar`/`char`/`citext`, `uuid`, `bytea`/`blob`,
`date`/`time`/`timestamp`/`timestamptz`, `json`/`jsonb`. Unknown types fall
through to each language's catch-all (`any`, `Any`, `unknown`,
`serde_json::Value`, `Any`, `std::string`).

Need a project-specific override? Use `optionOr` to let template authors pass
type overrides through `extra`:

```gotemplate
{{ $go := optionOr (printf "types.%s" .Type.Name) (goType .) .Options }}
```

### Scratch store

`setStore key value` writes to a per-render `map[string]any` and returns an
empty string (so it is safe to call from an action). `getStore key` reads
back. Useful for collecting imports or accumulating state across templates
in a single render.

```gotemplate
{{ setStore "imports" (list "time" "database/sql") }}
...
{{ range getStore "imports" }}import "{{ . }}"
{{ end }}
```

### Sprig

The full [sprig v3](https://masterminds.github.io/sprig/) `TxtFuncMap` is
available. Where sprig and this plugin define the same name (e.g.
`snakecase`/`snakeCase`), the plugin's helpers take precedence — sprig
functions are registered first and overwritten by the naming, navigation,
language, and store helpers.

## Writing templates

```gotemplate
{{- /* templates/{{ .Options.package }}/models.go.tmpl */ -}}
package {{ .Options.package }}

{{ range .Request.Catalog.Schemas }}
{{- range .Tables }}
type {{ pascalCase .Rel.Name | singular }} struct {
{{- range .Columns }}
    {{ pascalCase .Name }} {{ goType . }} `db:"{{ .Name }}"`
{{- end }}
}
{{ end }}
{{- end }}
```

## Caveats

- **Use process mode for `template_dir`.** sqlc's WASM sandbox provides no
  filesystem access; `os.Stat` / `filepath.WalkDir` return `EBADF` inside a
  WASM plugin. `template_dir` only works when the plugin runs as a native
  process. See the [Installation](#installation) section.
- **No `formatter_cmd`.** The plugin cannot spawn subprocesses. Format the
  generated output yourself (`go fmt ./...`, `prettier --write`, `rustfmt`, …).
- **Raw proto surface.** Template data is the sqlc SDK protobuf. The plugin
  pins a specific SDK version; field paths may shift across SDK bumps.

## Development

```bash
nix develop
go tool ginkgo run -r -coverprofile=coverage.out -covermode=atomic ./...
```

Build the WASM artifact locally:

```bash
nix build .#wasm
sha256sum result/bin/sqlc-gen-template.wasm
```

## License

[MIT](LICENSE)
