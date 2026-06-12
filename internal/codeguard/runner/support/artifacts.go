package support

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// ArtifactSink collects report artifacts emitted by checks during a scan.
type ArtifactSink struct {
	artifacts []core.ReportArtifact
}

func NewArtifactSink() *ArtifactSink {
	return &ArtifactSink{}
}

func (s *ArtifactSink) Add(artifact core.ReportArtifact) {
	if s == nil {
		return
	}
	s.artifacts = append(s.artifacts, artifact)
}

func (s *ArtifactSink) List() []core.ReportArtifact {
	if s == nil || len(s.artifacts) == 0 {
		return nil
	}
	return append([]core.ReportArtifact(nil), s.artifacts...)
}

// VisitTargetFiles walks target files like ScanTargetFiles but bypasses the
// findings cache, so callers that build cross-file state (such as import
// graphs) always observe every file.
func VisitTargetFiles(sc Context, target core.TargetConfig, include func(string) bool, visit func(rel string, data []byte)) {
	files, _ := WalkFiles(target.Path, sc.Cfg.Exclude, include)
	for _, file := range files {
		data, err := os.ReadFile(filepath.Join(target.Path, file))
		if err != nil {
			continue
		}
		visit(file, data)
	}
}

// ChangedDiffFiles returns the sorted set of changed file paths in diff mode.
func ChangedDiffFiles(sc Context) []string {
	if len(sc.Diff) == 0 {
		return nil
	}
	files := make([]string, 0, len(sc.Diff))
	for path := range sc.Diff {
		files = append(files, path)
	}
	sort.Strings(files)
	return files
}
