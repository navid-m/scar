package renderer

import (
	"scar/lexer"
	"strings"
	"testing"
)

func TestRenderC(t *testing.T) {
	program := &lexer.Program{
		Statements: []*lexer.Statement{
			{
				Print: &lexer.PrintStmt{
					Print: "Hello, World!",
				},
			},
		},
	}

	cCode := RenderC(program, "")
	expected := `printf("Hello, World!\n");`

	if !strings.Contains(cCode, expected) {
		t.Errorf("Expected C code to contain '%s', but it didn't", expected)
	}
}
