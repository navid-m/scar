package preprocessor

import "testing"

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
