package renderer

import (
	"scar/lexer"
	"strings"
	"testing"
)

func TestStringReturningFunctionDeclarationAndImplementation(t *testing.T) {
	program := &lexer.Program{
		Statements: []*lexer.Statement{
			{
				PubTopLevelFuncDecl: &lexer.PubTopLevelFuncDeclStmt{
					Name:       "readln",
					Parameters: []*lexer.MethodParameter{},
					ReturnType: "string",
					Body: []*lexer.Statement{
						{
							RawCode: &lexer.RawCodeStmt{
								Code: `char buffer[1024];
if (fgets(buffer, sizeof(buffer), stdin) != NULL) {
    size_t len = strlen(buffer);
    if (len > 0 && buffer[len - 1] == '\n') {
        buffer[len - 1] = '\0';
    }
    return buffer;
} else {
    return "";
}`,
							},
						},
					},
				},
			},
			{
				VarDecl: &lexer.VarDeclStmt{
					Name:  "asdf",
					Type:  "string",
					Value: "readln()",
				},
			},
		},
	}

	result := RenderC(program, "")

	if !strings.Contains(result, "void readln(char* _output_buffer);") {
		t.Errorf("Expected function declaration 'void readln(char* _output_buffer);' but got:\n%s", result)
	}

	if strings.Contains(result, "char* readln();") {
		t.Error("Should not have old-style 'char* readln();' declaration")
	}

	if !strings.Contains(result, "void readln(char* _output_buffer) {") {
		t.Error("Expected function implementation with correct signature")
	}

	if strings.Contains(result, "return buffer;") {
		t.Error("Should not have 'return buffer;' - should be transformed")
	}

	if !strings.Contains(result, "strcpy(_output_buffer, buffer);") {
		t.Error("Expected 'strcpy(_output_buffer, buffer);' transformation")
	}

	if !strings.Contains(result, `strcpy(_output_buffer, "");`) {
		t.Error("Expected empty string return to be transformed")
	}

	if !strings.Contains(result, "char asdf[256];") {
		t.Error("Expected string variable declaration")
	}

	if !strings.Contains(result, "readln(asdf);") {
		t.Error("Expected function call with buffer parameter")
	}

	if strings.Contains(result, "strcpy(asdf, readln())") {
		t.Error("Should not use strcpy with function call")
	}
}

func TestStringFunctionWithParameters(t *testing.T) {
	program := &lexer.Program{
		Statements: []*lexer.Statement{
			{
				TopLevelFuncDecl: &lexer.TopLevelFuncDeclStmt{
					Name: "formatString",
					Parameters: []*lexer.MethodParameter{
						{Name: "prefix", Type: "string"},
						{Name: "value", Type: "int"},
					},
					ReturnType: "string",
					Body: []*lexer.Statement{
						{
							RawCode: &lexer.RawCodeStmt{
								Code: `sprintf(buffer, "%s: %d", prefix, value);
return buffer;`,
							},
						},
					},
				},
			},
			{
				VarDecl: &lexer.VarDeclStmt{
					Name:  "result",
					Type:  "string",
					Value: `formatString("Number", 42)`,
				},
			},
		},
	}

	result := RenderC(program, "")

	expectedDecl := "void formatString(char* _output_buffer, char* prefix, int value);"
	if !strings.Contains(result, expectedDecl) {
		t.Errorf("Expected declaration: %s\nGot:\n%s", expectedDecl, result)
	}

	expectedImpl := "void formatString(char* _output_buffer, char* prefix, int value) {"
	if !strings.Contains(result, expectedImpl) {
		t.Error("Expected correct implementation signature")
	}

	if !strings.Contains(result, `formatString(result, "Number", 42);`) {
		t.Errorf("Expected function call with all parameters including buffer. Got:\n%s", result)
	}
}

func TestNonStringFunctionUnchanged(t *testing.T) {
	program := &lexer.Program{
		Statements: []*lexer.Statement{
			{
				TopLevelFuncDecl: &lexer.TopLevelFuncDeclStmt{
					Name:       "getNumber",
					Parameters: []*lexer.MethodParameter{},
					ReturnType: "int",
					Body: []*lexer.Statement{
						{
							Return: &lexer.ReturnStmt{Value: "42"},
						},
					},
				},
			},
			{
				VarDecl: &lexer.VarDeclStmt{
					Name:  "num",
					Type:  "int",
					Value: "getNumber()",
				},
			},
		},
	}

	result := RenderC(program, "")
	if !strings.Contains(result, "int getNumber();") {
		t.Error("Expected normal int function declaration")
	}

	if !strings.Contains(result, "int getNumber() {") {
		t.Error("Expected normal int function implementation")
	}

	if !strings.Contains(result, "int num = getNumber();") {
		t.Error("Expected normal function call assignment")
	}
}

func TestMixedStringAndNonStringFunctions(t *testing.T) {
	program := &lexer.Program{
		Statements: []*lexer.Statement{
			{
				TopLevelFuncDecl: &lexer.TopLevelFuncDeclStmt{
					Name:       "getString",
					Parameters: []*lexer.MethodParameter{},
					ReturnType: "string",
					Body: []*lexer.Statement{
						{
							RawCode: &lexer.RawCodeStmt{Code: `return "hello";`},
						},
					},
				},
			},
			{
				TopLevelFuncDecl: &lexer.TopLevelFuncDeclStmt{
					Name:       "getInt",
					Parameters: []*lexer.MethodParameter{},
					ReturnType: "int",
					Body: []*lexer.Statement{
						{
							Return: &lexer.ReturnStmt{Value: "123"},
						},
					},
				},
			},
			{
				VarDecl: &lexer.VarDeclStmt{
					Name:  "str",
					Type:  "string",
					Value: "getString()",
				},
			},
			{
				VarDecl: &lexer.VarDeclStmt{
					Name:  "num",
					Type:  "int",
					Value: "getInt()",
				},
			},
		},
	}

	result := RenderC(program, "")

	if !strings.Contains(result, "void getString(char* _output_buffer);") {
		t.Error("String function should have modified signature")
	}
	if !strings.Contains(result, "int getInt();") {
		t.Error("Int function should have normal signature")
	}
	if !strings.Contains(result, "getString(str);") {
		t.Error("String variable should call function with buffer")
	}
	if !strings.Contains(result, "int num = getInt();") {
		t.Error("Int variable should use normal assignment")
	}
}
