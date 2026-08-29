package version

import (
	"runtime/debug"
	"testing"
)

func TestResolveVersionPrecedenceAndFallback(t *testing.T) {
	tests := []struct {
		name    string
		current string
		info    *debug.BuildInfo
		want    string
	}{
		{name: "linker injection wins", current: "v9.8.7", info: &debug.BuildInfo{Main: debug.Module{Version: "v1.2.3"}}, want: "v9.8.7"},
		{name: "go install module version", current: "devel", info: &debug.BuildInfo{Main: debug.Module{Version: "v9.8.7"}}, want: "v9.8.7"},
		{name: "missing build metadata", current: "devel", info: nil, want: "devel"},
		{name: "devel build metadata", current: "devel", info: &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, want: "devel"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Resolve(tc.current, tc.info); got != tc.want {
				t.Fatalf("Resolve() = %q, want %q", got, tc.want)
			}
		})
	}
}
