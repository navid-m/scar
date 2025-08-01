package preprocessor

import (
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RemoveComments(tt.input); got != tt.expected {
				t.Errorf("RemoveComments() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestProcessGetExpressions(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple replacement",
			input:    "x = get!(y, 1)",
			expected: "x = y_values[1]",
		},
		{
			name:     "no replacement in string literal",
			input:    `print("hello get!(world, 1)")`,
			expected: `print("hello get!(world, 1)")`,
		},
		{
			name:     "nested get expressions",
			input:    "x = get!(y, get!(z, 2))",
			expected: "x = y_values[z_values[2]]",
		},
		{
			name:     "multiple replacements",
			input:    "a = get!(b, 0); c = get!(d, 3)",
			expected: "a = b_values[0]; c = d_values[3]",
		},
		{
			name:     "no get expressions",
			input:    "x = y + 1",
			expected: "x = y + 1",
		},
		{
			name:     "complex arguments",
			input:    "x = get!(my_map, some_var + 2)",
			expected: "x = my_map_values[some_var + 2]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ProcessGetExpressions(tt.input); got != tt.expected {
				t.Errorf("ProcessGetExpressions() = %v, want %v", got, tt.expected)
			}
		})
	}
}
