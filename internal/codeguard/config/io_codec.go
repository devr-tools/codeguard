package config

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"gopkg.in/yaml.v3"
)

func marshalConfig(path string, cfg core.Config) ([]byte, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		return yaml.Marshal(cfg)
	default:
		return json.MarshalIndent(cfg, "", "  ")
	}
}

func unmarshalConfig(data []byte, path string, cfg *core.Config) error {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		return yaml.Unmarshal(data, cfg)
	default:
		return json.Unmarshal(data, cfg)
	}
}
