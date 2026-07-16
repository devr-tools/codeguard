package checks_test

type securityGoCase struct {
	name    string
	source  []string
	status  string
	present []string
	absent  []string
}

func securityGoDetectionCases() []securityGoCase {
	return []securityGoCase{
		{
			name: "comment mentioning os/exec and exec.Command does not fire",
			source: []string{
				"package main",
				"",
				"// This package intentionally avoids os/exec; never call exec.Command(\"sh\").",
				"func main() {}",
			},
			status: "pass",
			absent: []string{"security.shell-execution"},
		},
		{
			name:   "import of os/exec without a call does not fire",
			source: []string{"package main", "", "import _ \"os/exec\"", "", "func main() {}"},
			status: "pass",
			absent: []string{"security.shell-execution"},
		},
		{
			name:   "string literal mentioning risky patterns does not fire",
			source: []string{"package main", "", "const usage = `avoid exec.Command(\"sh\") and InsecureSkipVerify: true in production`", "", "func main() {}"},
			status: "pass",
			absent: []string{"security.shell-execution", "security.insecure-tls"},
		},
		{
			name:   "exec.Command call fires",
			source: []string{"package main", "", "import \"os/exec\"", "", "func main() { _ = exec.Command(\"ls\") }"},
			status: "warn", present: []string{"security.shell-execution"},
		},
		{
			name:   "aliased exec.CommandContext call fires",
			source: []string{"package main", "", "import (", "\t\"context\"", "", "\trun \"os/exec\"", ")", "", "func main() { _ = run.CommandContext(context.Background(), \"ls\") }"},
			status: "warn", present: []string{"security.shell-execution"},
		},
		{
			name:   "syscall.Exec call fires",
			source: []string{"package main", "", "import \"syscall\"", "", "func main() { _ = syscall.Exec(\"/bin/ls\", nil, nil) }"},
			status: "warn", present: []string{"security.shell-execution"},
		},
		{
			name:   "InsecureSkipVerify without space in composite literal fires",
			source: []string{"package main", "", "import \"crypto/tls\"", "", "func config() *tls.Config { return &tls.Config{InsecureSkipVerify:true} }"},
			status: "fail", present: []string{"security.insecure-tls"},
		},
		{
			name:   "InsecureSkipVerify assignment fires",
			source: []string{"package main", "", "import \"crypto/tls\"", "", "func harden(cfg *tls.Config) { cfg.InsecureSkipVerify = true }"},
			status: "fail", present: []string{"security.insecure-tls"},
		},
		{
			name:   "InsecureSkipVerify set from a non-literal value does not fire",
			source: []string{"package main", "", "import \"crypto/tls\"", "", "func harden(cfg *tls.Config, allowInsecure bool) { cfg.InsecureSkipVerify = allowInsecure }"},
			status: "pass", absent: []string{"security.insecure-tls"},
		},
	}
}

func securityGoFallbackCases() []securityGoCase {
	return []securityGoCase{
		{
			name:   "shell call in unparseable file still fires",
			source: []string{"package main", "", "func main() {", "\texec.Command(\"sh\"", "}"},
			status: "warn", present: []string{"security.shell-execution"},
		},
		{
			name:   "insecure TLS without space in unparseable file still fires",
			source: []string{"package main", "", "func broken( {}", "", "var cfg = tls.Config{InsecureSkipVerify:true}"},
			status: "fail", present: []string{"security.insecure-tls"},
		},
		{
			name:   "comment mention in unparseable file does not fire",
			source: []string{"package main", "", "// exec.Command(\"sh\") and InsecureSkipVerify: true are documented here.", "func broken( {}"},
			status: "pass", absent: []string{"security.shell-execution", "security.insecure-tls"},
		},
	}
}
