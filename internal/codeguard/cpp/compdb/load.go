package compdb

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func Load(root, configured string) (*Database, error) {
	root, err := canonicalRoot(root)
	if err != nil {
		return nil, err
	}
	path, err := Find(root, strings.TrimSpace(configured))
	if err != nil {
		return nil, err
	}
	raw, err := readRawEntries(path)
	if err != nil {
		return nil, err
	}
	return buildDatabase(root, path, raw), nil
}

func readRawEntries(path string) ([]rawEntry, error) {
	// #nosec G304 -- Find canonicalizes the path and confines it to root.
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > maxDatabaseBytes {
		return nil, fmt.Errorf("compile_commands.json exceeds %d bytes", maxDatabaseBytes)
	}
	return decodeRawEntries(io.LimitReader(file, maxDatabaseBytes+1))
}

func decodeRawEntries(reader io.Reader) ([]rawEntry, error) {
	var raw []rawEntry
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(&raw); err != nil {
		return nil, fmt.Errorf("parse compile_commands.json: %w", err)
	}
	var trailing any
	err := decoder.Decode(&trailing)
	if errors.Is(err, io.EOF) {
		return raw, nil
	}
	if err == nil {
		return nil, fmt.Errorf("parse compile_commands.json: unexpected trailing JSON value")
	}
	return nil, fmt.Errorf("parse compile_commands.json: %w", err)
}

func buildDatabase(root, databasePath string, raw []rawEntry) *Database {
	db := &Database{Path: databasePath, Root: root, Entries: make([]Entry, 0, len(raw))}
	for _, item := range raw {
		if entry, ok := normalizeEntry(root, filepath.Dir(databasePath), item); ok {
			db.Entries = append(db.Entries, entry)
		}
	}
	return db
}
