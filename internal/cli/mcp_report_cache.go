package cli

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

// The server keeps the most recent reports in memory so a cursor pages through
// the exact snapshot it was issued for. Re-running the scan per page would let
// the working tree change between pages and produce duplicates or gaps, and
// would cost a full scan per page. The cache is small and time-bounded because
// a report on a large repository can hold thousands of findings.
const (
	mcpReportCacheSize = 8
	mcpReportCacheTTL  = 30 * time.Minute
)

var (
	errCursorInvalid  = errors.New("invalid cursor")
	errCursorExpired  = errors.New("cursor expired: the scan it refers to is no longer cached; call the tool again without cursor to start a new scan")
	errCursorTool     = errors.New("cursor was issued by a different tool")
	errCursorFilters  = errors.New("cursor was issued for different filters; pass the same sections, rules, min_level, min_confidence and paths as the call that returned it, or omit cursor to start over")
	errCursorPastEnd  = errors.New("cursor points past the end of the matched findings")
	errScanIDUnusable = errors.New("could not allocate a scan id")
)

type mcpCachedReport struct {
	id      string
	tool    string
	report  service.Report
	created time.Time
}

type mcpReportCache struct {
	mu      sync.Mutex
	entries []mcpCachedReport
}

// put stores report and returns the id a cursor uses to find it again.
func (c *mcpReportCache) put(tool string, report service.Report) (string, error) {
	id, err := newScanID()
	if err != nil {
		return "", err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.evictExpiredLocked()
	c.entries = append(c.entries, mcpCachedReport{id: id, tool: tool, report: report, created: time.Now()})
	if overflow := len(c.entries) - mcpReportCacheSize; overflow > 0 {
		c.entries = append([]mcpCachedReport(nil), c.entries[overflow:]...)
	}
	return id, nil
}

func (c *mcpReportCache) get(id string) (mcpCachedReport, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.evictExpiredLocked()
	for _, entry := range c.entries {
		if entry.id == id {
			return entry, true
		}
	}
	return mcpCachedReport{}, false
}

func (c *mcpReportCache) evictExpiredLocked() {
	cutoff := time.Now().Add(-mcpReportCacheTTL)
	kept := c.entries[:0]
	for _, entry := range c.entries {
		if entry.created.After(cutoff) {
			kept = append(kept, entry)
		}
	}
	c.entries = kept
}

func newScanID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", errScanIDUnusable
	}
	return hex.EncodeToString(buf), nil
}

// mcpCursor is the decoded pagination token. It is opaque to clients; the
// scan id is a random 128-bit value, so a cursor cannot be guessed to read
// another caller's cached report.
type mcpCursor struct {
	Version int    `json:"v"`
	ScanID  string `json:"s"`
	Tool    string `json:"t"`
	Offset  int    `json:"o"`
	Filters string `json:"f"`
}

func encodeCursor(cursor mcpCursor) string {
	cursor.Version = 1
	data, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeCursor(raw string) (mcpCursor, error) {
	var cursor mcpCursor
	data, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return cursor, errCursorInvalid
	}
	if json.Unmarshal(data, &cursor) != nil || cursor.Version != 1 || cursor.ScanID == "" || cursor.Offset < 0 {
		return cursor, errCursorInvalid
	}
	return cursor, nil
}

// resolveCursor loads the cached report a cursor refers to and checks that it
// is being resumed by the same tool with the same filters.
func (c *mcpReportCache) resolveCursor(raw string, tool string, filters mcpFindingFilters) (mcpCursor, mcpCachedReport, error) {
	cursor, err := decodeCursor(raw)
	if err != nil {
		return cursor, mcpCachedReport{}, err
	}
	if cursor.Tool != tool {
		return cursor, mcpCachedReport{}, errCursorTool
	}
	if cursor.Filters != filters.fingerprint() {
		return cursor, mcpCachedReport{}, errCursorFilters
	}
	entry, ok := c.get(cursor.ScanID)
	if !ok {
		return cursor, mcpCachedReport{}, errCursorExpired
	}
	return cursor, entry, nil
}
