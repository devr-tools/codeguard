package corpus_test

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"testing"
)

// assertManifestMatchesFixtures fails when the fixture tree and the manifest
// drift apart in either direction, so every committed fixture stays covered.
func assertManifestMatchesFixtures(t *testing.T, man manifest) {
	t.Helper()
	for _, group := range man.Groups {
		onDisk := listFixtureFiles(t, group.Root)
		listed := make(map[string]bool, len(group.Files))
		for _, file := range group.Files {
			if listed[file.Path] {
				t.Errorf("group %s lists %s twice", group.Name, file.Path)
			}
			listed[file.Path] = true
			if !onDisk[file.Path] {
				t.Errorf("group %s lists %s, which does not exist under %s", group.Name, file.Path, group.Root)
			}
		}
		for _, path := range sortedKeys(onDisk) {
			if !listed[path] {
				t.Errorf("fixture %s in group %s is missing from the manifest", path, group.Name)
			}
		}
	}
}

func listFixtureFiles(t *testing.T, root string) map[string]bool {
	t.Helper()
	files := make(map[string]bool)
	err := filepath.WalkDir(filepath.FromSlash(root), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, relErr := filepath.Rel(filepath.FromSlash(root), path)
		if relErr != nil {
			return relErr
		}
		files[filepath.ToSlash(rel)] = true
		return nil
	})
	if err != nil {
		t.Fatalf("walk fixtures under %s: %v", root, err)
	}
	return files
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// describeLine renders a manifest line constraint for error messages.
func describeLine(line int) string {
	if line == 0 {
		return "any line"
	}
	return fmt.Sprintf("line %d", line)
}
