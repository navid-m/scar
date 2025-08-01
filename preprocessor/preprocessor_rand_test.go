package preprocessor

import (
	"fmt"
	"testing"
)

func TestReplaceRandCalls(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple rand call",
			input:    "int x = rand(1, 10);",
			expected: "int x = rand__internal(1, 10);",
		},
		{
			name:     "multiple rand calls",
			input:    "int x = rand(1, 10); int y = rand(50, 100);",
			expected: "int x = rand__internal(1, 10); int y = rand__internal(50, 100);",
		},
		{
			name:     "rand with spaces",
			input:    "int x = rand( 1 , 10 );",
			expected: "int x = rand__internal( 1 , 10 );",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := replaceRandCalls(tt.input)
			if result != tt.expected {
				t.Errorf("replaceRandCalls() = %v, want %v", result, tt.expected)
			}
			fmt.Printf("Input: %s\nOutput: %s\nExpected: %s\n\n", tt.input, result, tt.expected)
		})
	}
}
