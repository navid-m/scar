package main

import (
	"os"
	"path/filepath"
	"scar/lexer"
	"scar/renderer"
	"strings"
	"testing"
)

func TestForeverLoop(t *testing.T) {
	input := `print "This will print forever: "
sleep 3
while 1:
    print "Hello"`

	program, err := lexer.ParseWithIndentation(input)
	if err != nil {
		t.Fatalf("Failed to parse input: %v", err)
	}

	result := renderer.RenderC(program, ".", false)
	assertContainsAll(t, result, []string{
		"int main(",
		"printf(\"This will print forever:",
		"sleep(3);",
		"while (1)",
		"printf(\"Hello",
		"return 0;",
	})
}

func TestForLoop(t *testing.T) {
	input := `print "start..."

for i = 0 to 3: 
    print "looping" 
    sleep 1 

print "done..."`

	program, err := lexer.ParseWithIndentation(input)
	if err != nil {
		t.Fatalf("Failed to parse input: %v", err)
	}

	result := renderer.RenderC(program, ".", false)
	assertContainsAll(t, result, []string{
		"int main(",
		"printf(\"start...",
		"for (int i = 0; i <= 3; i++)",
		"printf(\"looping",
		"sleep(1);",
		"printf(\"done...",
		"return 0;",
	})
}

func TestNestedStructures(t *testing.T) {
	input := `print "Starting nested test"
for i = 1 to 2:
    print "Outer loop"
    while 1:
        print "Inner while"
        sleep 1
    print "After while"
print "All done"`

	program, err := lexer.ParseWithIndentation(input)
	if err != nil {
		t.Fatalf("Failed to parse input: %v", err)
	}

	result := renderer.RenderC(program, ".", false)
	assertContainsAll(t, result, []string{
		"int main(",
		"for (int i = 1; i <= 2; i++)",
		"while (1)",
		"sleep(1);",
		`printf("After while\n")`,
		`printf("All done\n");`,
		"return 0;",
	})
}

func TestIndentationHandling(t *testing.T) {
	input := `print "before"
for i = 1 to 1:
    print "inside"
print "after"`
	program, err := lexer.ParseWithIndentation(input)
	if err != nil {
		t.Fatalf("Failed to parse input: %v", err)
	}
	if len(program.Statements) != 3 {
		t.Errorf("Expected 3 top-level statements, got %d", len(program.Statements))
	}
	if program.Statements[0].Print == nil {
		t.Error("First statement should be print")
	}
	if program.Statements[1].For == nil {
		t.Error("Second statement should be for loop")
	}
	if len(program.Statements[1].For.Body) != 1 {
		t.Errorf("For loop body should have 1 statement, got %d", len(program.Statements[1].For.Body))
	}
	if program.Statements[2].Print == nil {
		t.Error("Third statement should be print")
	}
	if program.Statements[2].Print.Print != "after" {
		t.Error("Third statement should be 'after'")
	}
}

func TestEmptyLinesAndComments(t *testing.T) {
	input := `print "start"
# This is a comment

for i = 1 to 1:
    # Comment in block
    print "inside"
    
print "end"`

	program, err := lexer.ParseWithIndentation(input)
	if err != nil {
		t.Fatalf("Failed to parse input: %v", err)
	}
	if len(program.Statements) != 3 {
		t.Errorf("Expected 3 top-level statements, got %d", len(program.Statements))
	}
}

func TestWhileConditions(t *testing.T) {
	testCases := []struct {
		input     string
		condition string
	}{
		{"while 1:", "1"},
		{"while x:", "x"},
		{"while count > 0:", "count > 0"},
		{"while i < 10:", "i < 10"},
	}

	for _, tc := range testCases {
		input := tc.input + "\n    print \"test\""
		program, err := lexer.ParseWithIndentation(input)
		if err != nil {
			t.Fatalf("Failed to parse '%s': %v", tc.input, err)
		}

		if len(program.Statements) != 1 {
			t.Errorf("Expected 1 statement for '%s'", tc.input)
			continue
		}

		whileStmt := program.Statements[0].While
		if whileStmt == nil {
			t.Errorf("Expected while statement for '%s'", tc.input)
			continue
		}

		if whileStmt.Condition != tc.condition {
			t.Errorf("Expected condition '%s', got '%s' for input '%s'",
				tc.condition, whileStmt.Condition, tc.input)
		}
	}
}

func TestCompileAllTestFiles(t *testing.T) {
	files, err := filepath.Glob("tests/prims/*.scar")
	if err != nil {
		t.Fatalf("Failed to list test files: %v", err)
	}

	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			content, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("Failed to read file %s: %v", file, err)
			}

			program, err := lexer.ParseWithIndentation(string(content))
			if err != nil {
				if strings.Contains(err.Error(), "$raw blocks are only allowed in the standard library") {
					return
				}
				t.Fatalf("Failed to parse file %s: %v", file, err)
			}

			output := renderer.RenderC(program, ".", false)
			if len(output) == 0 {
				t.Errorf("Rendered output is empty for file %s", file)
			}
		})
	}
}

func normalizeWhitespace(s string) string {
	lines := strings.Split(s, "\n")
	var normalized []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}

	return strings.Join(normalized, "\n")
}

func assertContainsAll(t *testing.T, haystack string, needles []string) {
	t.Helper()
	norm := normalizeWhitespace(haystack)
	for _, n := range needles {
		if !strings.Contains(norm, strings.TrimSpace(n)) {
			t.Errorf("Expected generated code to contain: %q\n--- Begin Generated ---\n%s\n--- End Generated ---", n, haystack)
		}
	}
}
