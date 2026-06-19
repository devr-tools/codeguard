package cli

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// mcp_http.go implements a Streamable-HTTP transport for the MCP server so
// remote, cloud-hosted hosts (e.g. Devin) can reach codeguard over a URL. It
// reuses the transport-neutral core in mcp_dispatch.go: synchronous methods go
// through dispatchSyncMethod and tools/call streams progress over SSE.

const (
	mcpHTTPMaxBodyBytes       = 4 << 20 // 4 MiB request cap
	mcpHTTPMaxConcurrentTools = 8       // concurrent tools/call executions
	mcpSessionHeader          = "Mcp-Session-Id"
	mcpDefaultAuthHeader      = "Authorization"
	mcpDefaultHTTPPath        = "/mcp"
	mcpHealthPath             = "/healthz"
	contentTypeJSON           = "application/json"
	contentTypeEventStream    = "text/event-stream"
)

// mcpAuthConfig configures optional static-bearer auth for the HTTP transport.
// A blank token disables auth (suitable only behind a private network).
type mcpAuthConfig struct {
	token  string
	header string
}

func (a mcpAuthConfig) enabled() bool { return strings.TrimSpace(a.token) != "" }

func (a mcpAuthConfig) headerName() string {
	if strings.TrimSpace(a.header) == "" {
		return mcpDefaultAuthHeader
	}
	return a.header
}

// authorize reports whether the request carries the expected credential. For
// the Authorization header it accepts an optional "Bearer " scheme prefix.
func (a mcpAuthConfig) authorize(r *http.Request) bool {
	if !a.enabled() {
		return true
	}
	got := strings.TrimSpace(r.Header.Get(a.headerName()))
	if strings.EqualFold(a.headerName(), mcpDefaultAuthHeader) {
		if rest := strings.TrimSpace(strings.TrimPrefix(got, "Bearer ")); len(rest) != len(got) {
			got = rest
		} else if rest := strings.TrimSpace(strings.TrimPrefix(got, "bearer ")); len(rest) != len(got) {
			got = rest
		}
	}
	want := strings.TrimSpace(a.token)
	if len(got) != len(want) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}
