// Package sqlc provides the sqlc codegen plugin entry point and
// configuration parsing for sqlc-gen-template.
package sqlc

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// PluginName is the name under which this plugin is typically registered
// in sqlc.yaml.
const PluginName = "template"

// Options holds plugin-specific options decoded from the JSON payload
// that sqlc sends in GenerateRequest.PluginOptions.
type Options struct {
	// TemplatesDir is the directory to walk for *.tmpl files. Required.
	// Relative paths resolve against the sqlc configuration directory.
	TemplatesDir string `json:"templates_dir"`
	// Extra is a free-form map surfaced to templates as .Options.Extra.
	Extra map[string]any `json:"extra,omitempty"`
}

// ParseOptions decodes the JSON plugin options payload. Unknown fields are
// rejected to catch typos in sqlc.yaml. An empty payload yields zero-value
// Options, which then fails Validate().
func ParseOptions(data []byte) (Options, error) {
	var opts Options
	if len(data) == 0 {
		return opts, opts.Validate()
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&opts); err != nil {
		return opts, fmt.Errorf("decode plugin options: %w", err)
	}
	return opts, opts.Validate()
}

// Validate returns an error if required options are missing.
func (o Options) Validate() error {
	if o.TemplatesDir == "" {
		return fmt.Errorf("plugin option %q is required", "templates_dir")
	}
	return nil
}
