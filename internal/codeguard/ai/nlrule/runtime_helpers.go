package nlrule

import (
	"os"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func formatStderr(stderr string) string {
	trimmed := strings.TrimSpace(stderr)
	if trimmed == "" {
		return ""
	}
	return ": " + trimmed
}

func runtimeCommand(cfg core.AIConfig) string {
	if strings.TrimSpace(cfg.Provider.Type) == "command" && strings.TrimSpace(cfg.Provider.Command) != "" {
		return strings.Join(append([]string{cfg.Provider.Command}, cfg.Provider.Args...), " ")
	}
	return strings.TrimSpace(os.Getenv(runtimeCommandEnv))
}
