// Package compdb reads C++ JSON compilation databases without executing any
// command text contained in them.
package compdb

import "errors"

const maxDatabaseBytes = 16 << 20

var ErrNotFound = errors.New("compile_commands.json not found")

type Database struct {
	Path    string
	Root    string
	Entries []Entry
}

type Entry struct {
	Directory    string
	File         string
	RelativeFile string
	Compiler     string
	IncludeDirs  []string
	Defines      []string
	Undefines    []string
	Standard     string
}

type rawEntry struct {
	Directory string   `json:"directory"`
	File      string   `json:"file"`
	Arguments []string `json:"arguments"`
	Command   string   `json:"command"`
}
