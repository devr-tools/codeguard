package support

import "testing"

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

	functions := ParsePythonFunctions(source)
	if len(functions) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(functions))
	}
	if functions[0].Name != "build" {
		t.Fatalf("expected first function to be build, got %q", functions[0].Name)
	}
	if functions[0].StartLine != 3 || functions[0].EndLine != 11 {
		t.Fatalf("expected build lines 3-10, got %d-%d", functions[0].StartLine, functions[0].EndLine)
	}
	if functions[0].Parameters != "\n        self,\n        config,\n        *,\n        retries=(1, 2),\n    " {
		t.Fatalf("unexpected parameters: %q", functions[0].Parameters)
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

	functions := ParseRustFunctions(source)
	if len(functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(functions))
	}
	if functions[0].Name != "execute" {
		t.Fatalf("expected execute, got %q", functions[0].Name)
	}
	if functions[0].StartLine != 6 || functions[0].EndLine != 12 {
		t.Fatalf("expected execute lines 6-12, got %d-%d", functions[0].StartLine, functions[0].EndLine)
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

	functions := ParseJavaFunctions(source)
	if len(functions) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(functions))
	}
	if functions[0].Name != "render" {
		t.Fatalf("expected first function to be render, got %q", functions[0].Name)
	}
	if functions[0].StartLine != 2 || functions[0].EndLine != 12 {
		t.Fatalf("expected render lines 2-12, got %d-%d", functions[0].StartLine, functions[0].EndLine)
	}
	if functions[1].Name != "run" {
		t.Fatalf("expected second function to be run, got %q", functions[1].Name)
	}
}
