package cli

// mcp_client.go holds the per-call transport callbacks that the tool layer
// reaches through context: a progress emitter (for streaming partial findings)
// and, added in a later phase, a client caller (for sampling/roots). Threading
// them via context keeps the many tool method signatures unchanged, and both
// are nil-safe so tools work when a transport does not supply them.
