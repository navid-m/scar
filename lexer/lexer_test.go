package lexer

import (
	"testing"
)

func TestParseSimpleVariableDeclaration(t *testing.T) {
	input := `var x = 10`
	program, err := ParseWithIndentation(input)
	if err != nil {
		t.Fatalf("ParseWithIndentation failed: %v", err)
	}
	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(program.Statements))
	}
	stmt := program.Statements[0]
	if stmt.VarDeclInferred == nil {
		t.Fatal("expected a VarDeclInferred statement, got nil")
	}
	if stmt.VarDeclInferred.Name != "x" {
		t.Errorf("expected var name 'x', got '%s'", stmt.VarDeclInferred.Name)
	}
	if stmt.VarDeclInferred.Value != "10" {
		t.Errorf("expected var value '10', got '%s'", stmt.VarDeclInferred.Value)
	}
}

func TestParseSimpleIfStatement(t *testing.T) {
	input := `
if x > 5:
    print "x is greater than 5"
`
	program, err := ParseWithIndentation(input)
	if err != nil {
		t.Fatalf("ParseWithIndentation failed: %v", err)
	}
	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(program.Statements))
	}
	ifStmt := program.Statements[0].If
	if ifStmt == nil {
		t.Fatal("expected an If statement, got nil")
	}
	if ifStmt.Condition != "x > 5" {
		t.Errorf("expected condition 'x > 5', got '%s'", ifStmt.Condition)
	}
	if len(ifStmt.Body) != 1 {
		t.Fatalf("expected 1 statement in if body, got %d", len(ifStmt.Body))
	}
}

func TestParseSimpleForLoop(t *testing.T) {
	input := `
for i = 0 to 10:
    print i
`
	program, err := ParseWithIndentation(input)
	if err != nil {
		t.Fatalf("ParseWithIndentation failed: %v", err)
	}
	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(program.Statements))
	}
	forStmt := program.Statements[0].For
	if forStmt == nil {
		t.Fatal("expected a For statement, got nil")
	}
	if forStmt.Var != "i" {
		t.Errorf("expected var 'i', got '%s'", forStmt.Var)
	}
	if forStmt.Start != "0" {
		t.Errorf("expected start '0', got '%s'", forStmt.Start)
	}
	if forStmt.End != "10" {
		t.Errorf("expected end '10', got '%s'", forStmt.End)
	}
}

func TestParseNestedBlocks(t *testing.T) {
	input := `
for i = 0 to 10:
    if i % 2 == 0:
        print "even"
    else:
        print "odd"
`
	program, err := ParseWithIndentation(input)
	if err != nil {
		t.Fatalf("ParseWithIndentation failed: %v", err)
	}
	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(program.Statements))
	}
	forStmt := program.Statements[0].For
	if forStmt == nil {
		t.Fatal("expected a For statement, got nil")
	}
	if len(forStmt.Body) != 1 {
		t.Fatalf("expected 1 statement in for body, got %d", len(forStmt.Body))
	}
	ifStmt := forStmt.Body[0].If
	if ifStmt == nil {
		t.Fatal("expected an If statement in for body, got nil")
	}
	if len(ifStmt.Body) != 1 {
		t.Errorf("expected 1 statement in if body, got %d", len(ifStmt.Body))
	}
	if ifStmt.Else == nil {
		t.Fatal("expected an Else statement, got nil")
	}
	if len(ifStmt.Else.Body) != 1 {
		t.Errorf("expected 1 statement in else body, got %d", len(ifStmt.Else.Body))
	}
}

func TestMismatchedIndentation(t *testing.T) {
	input := `
if x > 5:
  print "level 1"
    print "level 2"
`
	_, err := ParseWithIndentation(input)
	if err == nil {
		t.Error("expected an error for mismatched indentation, but got nil")
	}
}

func TestInvalidStatement(t *testing.T) {
	input := `let x = 10`
	_, err := ParseWithIndentation(input)
	if err == nil {
		t.Error("expected an error for invalid statement, got nil")
	}
}

func TestFunctionHoisting(t *testing.T) {
	input := `
fn main() -> int:
    # Call functions before they're defined
    result1 = add(5, 3)
    result2 = multiply(4, 6)
    return subtract(result2, result1)

fn add(int a, int b) -> int:
    return a + b

fn multiply(int a, int b) -> int:
    return a * b

fn subtract(int a, int b) -> int:
    return a - b
`

	program, err := ParseWithIndentation(input)
	if err != nil {
		t.Fatalf("ParseWithIndentation failed: %v", err)
	}

	var funcNames []string
	for _, stmt := range program.Statements {
		if stmt.TopLevelFuncDecl != nil {
			funcNames = append(funcNames, stmt.TopLevelFuncDecl.Name)
		}
	}

	expectedFuncs := map[string]bool{
		"main":     true,
		"add":      true,
		"multiply": true,
		"subtract": true,
	}

	for _, name := range funcNames {
		if !expectedFuncs[name] {
			t.Errorf("unexpected function name: %s", name)
		}
		delete(expectedFuncs, name)
	}

	for name := range expectedFuncs {
		t.Errorf("missing expected function: %s", name)
	}

	// The main function should be the first statement after hoisting
	if len(program.Statements) == 0 || program.Statements[0].TopLevelFuncDecl == nil ||
		program.Statements[0].TopLevelFuncDecl.Name != "main" {
		t.Error("expected 'main' function to be the first statement after hoisting")
	}
}

func TestClassMethodWithNoReturn(t *testing.T) {
	input := `
class Test:
    fn method() -> int:
        x = 10`
	_, err := ParseWithIndentation(input)
	if err != nil {
		// TODO: Ensure an error gets thrown here.
		t.Logf("Got an error as expected (or maybe not): %v", err)
	}
}

func TestReturnWithoutValue(t *testing.T) {
	input := `
fn test():
    return
`
	program, err := ParseWithIndentation(input)
	if err != nil {
		t.Fatalf("ParseWithIndentation failed: %v", err)
	}
	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(program.Statements))
	}
	if program.Statements[0].TopLevelFuncDecl == nil {
		t.Fatal("expected a TopLevelFuncDecl statement, got nil")
	}
	if len(program.Statements[0].TopLevelFuncDecl.Body) != 1 {
		t.Fatalf("expected 1 statement in function body, got %d", len(program.Statements[0].TopLevelFuncDecl.Body))
	}
	if program.Statements[0].TopLevelFuncDecl.Body[0].Return == nil {
		t.Fatal("expected a Return statement in function body, got nil")
	}
	if program.Statements[0].TopLevelFuncDecl.Body[0].Return.Value != "" {
		t.Errorf("expected empty return value, got '%s'", program.Statements[0].TopLevelFuncDecl.Body[0].Return.Value)
	}
}

func TestEmptyMapDeclaration(t *testing.T) {
	input := `map[string:int] emptyMap = []`
	program, err := ParseWithIndentation(input)
	if err != nil {
		t.Fatalf("ParseWithIndentation failed: %v", err)
	}
	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(program.Statements))
	}
	stmt := program.Statements[0]
	if stmt.MapDecl == nil {
		t.Fatal("expected a MapDecl statement, got nil")
	}
	if stmt.MapDecl.Name != "emptyMap" {
		t.Errorf("expected map name 'emptyMap', got '%s'", stmt.MapDecl.Name)
	}
	if stmt.MapDecl.KeyType != "string" {
		t.Errorf("expected key type 'string', got '%s'", stmt.MapDecl.KeyType)
	}
	if stmt.MapDecl.ValueType != "int" {
		t.Errorf("expected value type 'int', got '%s'", stmt.MapDecl.ValueType)
	}
	if len(stmt.MapDecl.Pairs) != 0 {
		t.Errorf("expected 0 pairs in empty map, got %d", len(stmt.MapDecl.Pairs))
	}
}

func TestContinueStatement(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		hasError bool
	}{
		{
			name: "simple continue in for loop",
			input: `for i = 0 to 10:
    if i % 2 == 0:
        continue
    print i`,
			hasError: false,
		},
		{
			name: "continue in while loop",
			input: `i = 0
while i < 10:
    i = i + 1
    if i % 2 != 0:
        continue
    print i`,
			hasError: false,
		},
		{
			name: "nested continue",
			input: `for i = 0 to 5:
    for j = 0 to 5:
        if i == j:
            continue
        print i, j`,
			hasError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, err := ParseWithIndentation(tt.input)
			if tt.hasError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseWithIndentation failed: %v", err)
			}

			var foundContinue bool
			var checkStmts func(stmts []*Statement)

			checkStmts = func(stmts []*Statement) {
				for _, stmt := range stmts {
					if stmt.Continue != nil {
						foundContinue = true
					}
					if stmt.If != nil {
						checkStmts(stmt.If.Body)
						for _, elif := range stmt.If.ElseIfs {
							checkStmts(elif.Body)
						}
						if stmt.If.Else != nil {
							checkStmts(stmt.If.Else.Body)
						}
					}
					if stmt.For != nil {
						checkStmts(stmt.For.Body)
					}
					if stmt.While != nil {
						checkStmts(stmt.While.Body)
					}
				}
			}

			checkStmts(program.Statements)

			if !foundContinue {
				t.Error("expected to find a continue statement in the parsed program")
			}
		})
	}
}
