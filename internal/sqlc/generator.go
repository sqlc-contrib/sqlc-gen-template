package sqlc

import (
	"context"
	"fmt"

	"github.com/sqlc-dev/plugin-sdk-go/plugin"

	"github.com/sqlc-contrib/sqlc-gen-template/internal/sqlc/template"
)

// Generate is the sqlc codegen entry point. It parses plugin options,
// walks the configured template directory, renders each template against
// the raw GenerateRequest, and returns the produced files.
func Generate(_ context.Context, req *plugin.GenerateRequest) (*plugin.GenerateResponse, error) {
	opts, err := ParseOptions(req.GetPluginOptions())
	if err != nil {
		return nil, fmt.Errorf("sqlc-gen-template: %w", err)
	}

	files, err := template.Render(template.Context{
		Request:     req,
		Options:     opts.Extra,
		TemplatesDir: opts.TemplatesDir,
		SqlcVersion: req.GetSqlcVersion(),
	})
	if err != nil {
		return nil, fmt.Errorf("sqlc-gen-template: %w", err)
	}

	return &plugin.GenerateResponse{Files: files}, nil
}
