package cli

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

func promptString(reader *bufio.Reader, stdout io.Writer, label string, fallback string) (string, error) {
	if _, err := fmt.Fprintf(stdout, "%s [%s]: ", label, fallback); err != nil {
		return "", err
	}
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return fallback, nil
	}
	return line, nil
}

func writeUsage(w io.Writer) {
	_, _ = io.WriteString(w, `codeguard checks repositories for code quality, design, security, CI, prompt safety, and custom policy rules.

Usage:
  codeguard init [-output codeguard.yaml] [-interactive] [-profile startup|strict|enterprise|ai-safe]
  codeguard validate [-config codeguard.yaml] [-profile startup|strict|enterprise|ai-safe]
  codeguard validate-patch [-config codeguard.yaml] [-format text|json|sarif|github] [-profile startup|strict|enterprise|ai-safe] [-ai] < patch.diff
  codeguard scan [-config codeguard.yaml] [-mode full|diff] [-base-ref main] [-format text|json|sarif|github] [-interactive] [-profile startup|strict|enterprise|ai-safe] [-ai]
  codeguard fix [-config codeguard.yaml] [-mode full|diff] [-base-ref main] [-profile startup|strict|enterprise|ai-safe] [-rule rule.id] [-path rel/path] [-line N] -ai
  codeguard baseline [-config codeguard.yaml] [-output codeguard-baseline.json] [-mode full|diff] [-base-ref main] [-profile startup|strict|enterprise|ai-safe]
  codeguard report -slop-history [-config codeguard.yaml] [-limit N] [-profile startup|strict|enterprise|ai-safe]
  codeguard rules [-config codeguard.yaml]
  codeguard explain [-config codeguard.yaml] [-format text|agent] <rule-id>
  codeguard serve --mcp [-config codeguard.yaml] [-profile startup|strict|enterprise|ai-safe]
  codeguard doctor [-config codeguard.yaml] [-profile startup|strict|enterprise|ai-safe]
  codeguard profiles
  codeguard version
`)
}

func loadConfigWithProfile(path string, profile string) (service.Config, error) {
	cfg, err := service.LoadConfigFile(path)
	if err != nil {
		return service.Config{}, err
	}
	if strings.TrimSpace(profile) != "" {
		cfg.Profile = strings.TrimSpace(profile)
		service.ApplyDefaults(&cfg)
		if err := service.ValidateConfig(cfg); err != nil {
			return service.Config{}, err
		}
	}
	return cfg, nil
}

func exampleConfigForProfile(profile string) (service.Config, error) {
	if strings.TrimSpace(profile) == "" {
		return service.ExampleConfig(), nil
	}
	return service.ExampleConfigForProfile(profile)
}
