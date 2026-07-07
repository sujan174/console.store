// Package orderapp holds the bundled MCP App UI served by the console MCP.
package orderapp

import _ "embed"

//go:embed order-app.html
var HTML string
