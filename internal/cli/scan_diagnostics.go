package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"sync"
	"time"

	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

const scanHeartbeatInterval = 30 * time.Second

type scanMonitor struct {
	mu      sync.Mutex
	writer  io.Writer
	started time.Time
}

func newScanMonitor(writer io.Writer, started time.Time) *scanMonitor {
	return &scanMonitor{writer: writer, started: started}
}

func (monitor *scanMonitor) writeStarted() {
	monitor.write("scan started\n")
}

func (monitor *scanMonitor) writeHeartbeat(now time.Time, heapBytes uint64, memoryLimit int64) {
	elapsed := now.Sub(monitor.started).Round(time.Second)
	monitor.write("scan in progress: elapsed=%s heap=%d MiB memory_limit=%s\n",
		elapsed, heapBytes/(1<<20), formatMemoryLimit(memoryLimit))
}

func (monitor *scanMonitor) writeSectionComplete(section service.SectionResult) {
	monitor.write("completed %s: %s (%d findings)\n", section.Name, section.Status, len(section.Findings))
}

func (monitor *scanMonitor) write(format string, args ...any) {
	if monitor == nil || monitor.writer == nil {
		return
	}
	monitor.mu.Lock()
	defer monitor.mu.Unlock()
	_, _ = fmt.Fprintf(monitor.writer, format, args...)
}

func formatMemoryLimit(limit int64) string {
	if limit < 0 || limit >= 1<<62 {
		return "unlimited"
	}
	return fmt.Sprintf("%d MiB", limit/(1<<20))
}

func startScanMonitoring(monitor *scanMonitor, heapProfilePath string, interval time.Duration) func() error {
	monitor.writeStarted()
	if heapProfilePath != "" {
		if err := writeHeapProfile(heapProfilePath); err != nil {
			monitor.write("heap profile update failed: %v\n", err)
		}
	}

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case now := <-ticker.C:
				var stats runtime.MemStats
				runtime.ReadMemStats(&stats)
				monitor.writeHeartbeat(now, stats.HeapAlloc, debug.SetMemoryLimit(-1))
				if heapProfilePath != "" {
					if err := writeHeapProfile(heapProfilePath); err != nil {
						monitor.write("heap profile update failed: %v\n", err)
					}
				}
			case <-stop:
				return
			}
		}
	}()

	return func() error {
		close(stop)
		<-done
		if heapProfilePath != "" {
			if err := writeHeapProfile(heapProfilePath); err != nil {
				return fmt.Errorf("write final heap profile: %w", err)
			}
		}
		return nil
	}
}

func writeHeapProfile(path string) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".codeguard-heap-*.pprof")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if err := pprof.WriteHeapProfile(tmp); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
