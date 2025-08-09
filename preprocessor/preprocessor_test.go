package preprocessor

import (
	"scar/lexer"
	"testing"
)

func TestInsertMacros(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no macros",
			input:    "int main() { return 0; }",
			expected: "int main() { return 0; }",
		},
		{
			name:     "len macro",
			input:    "int arr[] = {1, 2, 3}; int l = len(arr);",
			expected: "#define len(x) (sizeof(x) / sizeof((x)[0]))\nint arr[] = {1, 2, 3}; int l = len(arr);",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := InsertMacros(tt.input); got != tt.expected {
				t.Errorf("InsertMacros() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestRemoveComments(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no comments",
			input:    "print(\"Hello\")\nprint(\"World\")",
			expected: "print(\"Hello\")\nprint(\"World\")",
		},
		{
			name:     "single line comment",
			input:    "# This is a comment\nprint(\"Hello\")",
			expected: "\nprint(\"Hello\")",
		},
		{
			name:     "comment inside string",
			input:    `print("# This is not a comment")`,
			expected: `print("# This is not a comment")`,
		},
		{
			name:     "inline comment",
			input:    `hash = ((hash << 5) + hash) + ord(data[i])  # hash * 33 + c`,
			expected: `hash = ((hash << 5) + hash) + ord(data[i])  `,
		},
		{
			name:     "multiple inline comments",
			input:    "int x = 5 # comment 1\nint y = 10 # comment 2",
			expected: "int x = 5 \nint y = 10 ",
		},
		{
			name:     "mixed comments",
			input:    "# full line comment\nint x = 5 # inline comment\nprint(\"test\")",
			expected: "\nint x = 5 \nprint(\"test\")",
		},
		{
			name:     "raw block with preprocessor directives",
			input:    "$raw (\n    #include <stdio.h>\n    #define MAX 100\n)",
			expected: "$raw (\n    #include <stdio.h>\n    #define MAX 100\n)",
		},
		{
			name:     "mixed raw block and scar comments",
			input:    "# scar comment\n$raw (\n    #include <stdio.h> // C comment\n)\nint x = 5 # another scar comment",
			expected: "\n$raw (\n    #include <stdio.h> // C comment\n)\nint x = 5 ",
		},
		{
			name:     "nested parentheses in raw block",
			input:    "$raw (\n    #define FUNC(x) ((x) + 1)\n    printf(\"test\");\n)",
			expected: "$raw (\n    #define FUNC(x) ((x) + 1)\n    printf(\"test\");\n)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lexer.RemoveComments(tt.input); got != tt.expected {
				t.Errorf("RemoveComments() = %q, want %q", got, tt.expected)
			}
		})
	}
}
