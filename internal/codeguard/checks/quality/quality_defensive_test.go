package quality

import (
	"regexp"
	"testing"
)

func TestNullableParamHasBlockExitGuardRequiresExitInsideGuard(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "return in guard",
			body: `if (!user) { return missingUser(); } return user.email;`,
			want: true,
		},
		{
			name: "return after guard",
			body: `if (!user) { logMissing(); } return user.email;`,
			want: false,
		},
		{
			name: "return in nested guard scope",
			body: `if (!user) { if (missing()) { return fallback; } throw err; } use(user);`,
			want: true,
		},
		{
			name: "unclosed guard",
			body: `if (!user) { logMissing(); return fallback;`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nullableParamHasBlockExitGuard(tt.body, regexp.QuoteMeta("user"))
			if got != tt.want {
				t.Fatalf("nullableParamHasBlockExitGuard() = %v, want %v", got, tt.want)
			}
		})
	}
}
