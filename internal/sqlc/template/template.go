// Package template discovers, parses, and renders user-supplied Go templates
// against a sqlc GenerateRequest.
package template

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/sqlc-dev/plugin-sdk-go/plugin"
)

// Extension is the required suffix for template files discovered under
// the configured templates directory.
const Extension = ".tmpl"

// PartialPrefix marks template files that are parsed but not emitted as
// standalone outputs; they exist to be {{ template "_foo" . }}-included.
const PartialPrefix = "_"

// Context is the root value ('.') handed to every template and also used
// to render output-file names.
type Context struct {
	// Request is the raw sqlc GenerateRequest.
	Request *plugin.GenerateRequest
	// Options carries the user-supplied free-form extra map from sqlc.yaml.
	Options map[string]any
	// TemplatesDir is the absolute or sqlc-relative directory that was
	// walked to discover templates. Exposed to templates for reference.
	TemplatesDir string
	// SqlcVersion is hoisted from Request.SqlcVersion for convenience.
	SqlcVersion string
}

// Render walks ctx.TemplatesDir for *.tmpl files, parses them all into a
// shared template set (so cross-file {{ template }} includes work), then
// executes every non-partial template against ctx. Output filenames are
// the template's path relative to TemplatesDir, minus the .tmpl suffix,
// with the resulting string itself executed as a template.
func Render(ctx Context) ([]*plugin.File, error) {
	if ctx.TemplatesDir == "" {
		return nil, fmt.Errorf("templates_dir is empty")
	}
	info, err := os.Stat(ctx.TemplatesDir)
	if err != nil {
		return nil, fmt.Errorf("stat templates_dir %q: %w", ctx.TemplatesDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("templates_dir %q is not a directory", ctx.TemplatesDir)
	}

	paths, err := discover(ctx.TemplatesDir)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no %s files found under %q", Extension, ctx.TemplatesDir)
	}

	funcs := FuncMap()
	root := template.New("").Funcs(funcs)
	for _, p := range paths {
		body, err := os.ReadFile(filepath.Join(ctx.TemplatesDir, p))
		if err != nil {
			return nil, fmt.Errorf("read %q: %w", p, err)
		}
		if _, err := root.New(p).Parse(string(body)); err != nil {
			return nil, fmt.Errorf("parse %q: %w", p, err)
		}
	}

	var out []*plugin.File
	for _, p := range paths {
		if isPartial(p) {
			continue
		}

		outName, err := renderFilename(strings.TrimSuffix(p, Extension), ctx, funcs)
		if err != nil {
			return nil, fmt.Errorf("render filename for %q: %w", p, err)
		}

		var buf bytes.Buffer
		if err := root.ExecuteTemplate(&buf, p, ctx); err != nil {
			return nil, fmt.Errorf("execute %q: %w", p, err)
		}
		out = append(out, &plugin.File{Name: outName, Contents: buf.Bytes()})
	}
	return out, nil
}

// discover returns the sorted, forward-slash-normalised relative paths of
// every *.tmpl file under root.
func discover(root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), Extension) {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %q: %w", root, err)
	}
	return paths, nil
}

// isPartial reports whether the template's base name marks it as an
// include-only template. Partials are parsed but never emitted.
func isPartial(rel string) bool {
	return strings.HasPrefix(filepath.Base(rel), PartialPrefix)
}

// renderFilename executes name as an ad-hoc template with the same
// FuncMap as the body templates, so output paths can depend on options.
func renderFilename(name string, ctx Context, funcs template.FuncMap) (string, error) {
	tmpl, err := template.New("filename").Funcs(funcs).Parse(name)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return "", err
	}
	return buf.String(), nil
}
