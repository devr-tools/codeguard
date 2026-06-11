package prompts

import (
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/codeguard/checks/support"
	"github.com/devr-tools/codeguard/codeguard/core"
)

func discoverPromptFiles(targetPath string, rules promptRules, scope core.ScanScope) ([]string, error) {
	files, err := support.ScopedCandidateTextFiles(targetPath, scope)
	if err != nil {
		return nil, err
	}
	var matched []string
	for _, file := range files {
		if isPromptFile(file, rules) {
			matched = append(matched, file)
		}
	}
	return matched, nil
}

func isPromptFile(path string, rules promptRules) bool {
	lowerPath := strings.ToLower(filepath.ToSlash(path))
	ext := strings.ToLower(filepath.Ext(path))
	for _, candidate := range rules.fileExtensions {
		if ext == candidate {
			return true
		}
	}
	for _, candidate := range rules.pathContains {
		if strings.Contains(lowerPath, candidate) {
			return true
		}
	}
	return false
}
