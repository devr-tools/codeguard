package compdb

import (
	"path/filepath"
	"strings"
)

func (entry *Entry) addInclude(root, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	if !filepath.IsAbs(value) {
		value = filepath.Join(entry.Directory, filepath.FromSlash(value))
	}
	value, err := filepath.Abs(filepath.Clean(value))
	if err != nil || !within(root, value) {
		return
	}
	value, err = filepath.EvalSymlinks(value)
	if err != nil || !within(root, value) {
		return
	}
	for _, existing := range entry.IncludeDirs {
		if existing == value {
			return
		}
	}
	entry.IncludeDirs = append(entry.IncludeDirs, value)
}

func within(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func canonicalRoot(root string) (string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(root)
}
