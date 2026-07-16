package compdb

import (
	"path/filepath"
	"strings"
)

func normalizeEntry(root, databaseDir string, raw rawEntry) (Entry, bool) {
	directory, ok := normalizeDirectory(databaseDir, raw.Directory)
	if !ok {
		return Entry{}, false
	}
	file, relative, ok := normalizeSource(root, directory, raw.File)
	if !ok {
		return Entry{}, false
	}
	arguments, ok := entryArguments(raw)
	if !ok {
		return Entry{}, false
	}
	entry := Entry{Directory: directory, File: file, RelativeFile: relative}
	entry.extractMetadata(root, arguments)
	return entry, true
}

func normalizeDirectory(databaseDir, value string) (string, bool) {
	directory := strings.TrimSpace(value)
	if directory == "" {
		directory = databaseDir
	} else if !filepath.IsAbs(directory) {
		directory = filepath.Join(databaseDir, filepath.FromSlash(directory))
	}
	directory, err := filepath.Abs(filepath.Clean(directory))
	if err != nil {
		return "", false
	}
	if resolved, err := filepath.EvalSymlinks(directory); err == nil {
		directory = resolved
	}
	return directory, true
}

func normalizeSource(root, directory, value string) (string, string, bool) {
	file := strings.TrimSpace(value)
	if file == "" {
		return "", "", false
	}
	if !filepath.IsAbs(file) {
		file = filepath.Join(directory, filepath.FromSlash(file))
	}
	file, err := filepath.Abs(filepath.Clean(file))
	if err != nil {
		return "", "", false
	}
	file, err = filepath.EvalSymlinks(file)
	if err != nil || !within(root, file) {
		return "", "", false
	}
	relative, err := filepath.Rel(root, file)
	if err != nil || relative == "." {
		return "", "", false
	}
	return file, filepath.ToSlash(relative), true
}

func entryArguments(raw rawEntry) ([]string, bool) {
	if len(raw.Arguments) > 0 {
		return append([]string(nil), raw.Arguments...), true
	}
	if strings.TrimSpace(raw.Command) == "" {
		return nil, true
	}
	arguments, err := splitCommandLine(raw.Command)
	return arguments, err == nil
}
