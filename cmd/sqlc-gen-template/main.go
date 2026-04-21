// sqlc-gen-template is a sqlc plugin that renders arbitrary code from
// user-supplied Go templates against sqlc's parsed catalog and queries.
package main

import (
	"github.com/sqlc-dev/plugin-sdk-go/codegen"

	"github.com/sqlc-contrib/sqlc-gen-template/internal/sqlc"
)

func main() {
	codegen.Run(sqlc.Generate)
}
