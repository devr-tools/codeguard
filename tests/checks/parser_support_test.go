package checks_test

import (
	"testing"

	supportpkg "github.com/devr-tools/codeguard/internal/codeguard/checks/support"
)

func TestDependencyGraphReachablePath(t *testing.T) {
	graph := supportpkg.NewDependencyGraph(map[string]supportpkg.DependencyNode{
		"app.service": {
			ID: "app.service",
			Edges: []supportpkg.DependencyEdge{
				{To: "app.web"},
			},
		},
		"app.web": {
			ID: "app.web",
			Edges: []supportpkg.DependencyEdge{
				{To: "app.cli"},
			},
		},
		"app.cli": {ID: "app.cli"},
	})

	path := graph.ReachablePath("app.service", func(id string) bool {
		return id == "app.cli"
	})
	if len(path) != 3 {
		t.Fatalf("path length = %d, want 3 (%v)", len(path), path)
	}
	if path[0] != "app.service" || path[1] != "app.web" || path[2] != "app.cli" {
		t.Fatalf("path = %v, want app.service -> app.web -> app.cli", path)
	}
}

func TestDependencyGraphStronglyConnectedComponents(t *testing.T) {
	graph := supportpkg.NewDependencyGraph(map[string]supportpkg.DependencyNode{
		"app.repo": {
			ID: "app.repo",
			Edges: []supportpkg.DependencyEdge{
				{To: "app.service"},
			},
		},
		"app.service": {
			ID: "app.service",
			Edges: []supportpkg.DependencyEdge{
				{To: "app.repo"},
			},
		},
	})

	components := graph.StronglyConnectedComponents()
	if len(components) != 1 {
		t.Fatalf("component count = %d, want 1", len(components))
	}
	if len(components[0]) != 2 {
		t.Fatalf("component size = %d, want 2 (%v)", len(components[0]), components[0])
	}
}

func TestParsePythonFunctionsHandlesMultilineSignatures(t *testing.T) {
	source := `class Example:
    @decorator(value={"x": [1, 2]})
    async def build(
        self,
        config,
        *,
        retries=(1, 2),
    ):
        if config:
            return retries

    def helper():
        return 1
`

	functions := supportpkg.ParsePythonFunctions(source)
	if len(functions) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(functions))
	}
	if functions[0].Name != "build" {
		t.Fatalf("expected first function to be build, got %q", functions[0].Name)
	}
	if functions[0].StartLine != 3 || functions[0].EndLine != 11 {
		t.Fatalf("expected build lines 3-11, got %d-%d", functions[0].StartLine, functions[0].EndLine)
	}
}

func TestParseRustFunctionsSkipsDeclarationsWithoutBodies(t *testing.T) {
	source := `trait Worker {
    fn execute(&self, input: String);
}

impl Worker for Job {
    pub async fn execute(&self, input: String, options: Vec<(String, String)>) -> Result<()> {
        let template = r#"fn fake() {}"#;
        if input.is_empty() || options.is_empty() {
            return Ok(());
        }
        Ok(())
    }
}
`

	functions := supportpkg.ParseRustFunctions(source)
	if len(functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(functions))
	}
	if functions[0].Name != "execute" {
		t.Fatalf("expected execute, got %q", functions[0].Name)
	}
}

func TestParseJavaFunctionsSkipsAnnotationsAndAnonymousClasses(t *testing.T) {
	source := `class Example {
    @Route(path = "/x")
    public String render(String value, java.util.Map<String, Integer> lookup) {
        if (value.isEmpty() || lookup.isEmpty()) {
            return value;
        }
        Runnable r = new Runnable() {
            @Override
            public void run() {}
        };
        return value;
    }
}
`

	functions := supportpkg.ParseJavaFunctions(source)
	if len(functions) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(functions))
	}
	if functions[0].Name != "render" {
		t.Fatalf("expected first function to be render, got %q", functions[0].Name)
	}
	if functions[1].Name != "run" {
		t.Fatalf("expected second function to be run, got %q", functions[1].Name)
	}
}
