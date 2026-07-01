package cli

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

// promptString and loadConfigWithProfile live here; the command menu is
// rendered by writeMenu in menu.go.

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
