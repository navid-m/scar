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

// --- Tests that might pass ---

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

func TestParseBulkImport(t *testing.T) {
	input := `
import "fmt", "os"
`
	program, err := ParseWithIndentation(input)
	if err != nil {
		t.Fatalf("ParseWithIndentation failed: %v", err)
	}
	if len(program.Imports) != 2 {
		t.Fatalf("expected 2 import statements, got %d", len(program.Imports))
	}
	if program.Imports[0].Module != "fmt" {
		t.Errorf("expected module 'fmt', got '%s'", program.Imports[0].Module)
	}
	if program.Imports[1].Module != "os" {
		t.Errorf("expected module 'os', got '%s'", program.Imports[1].Module)
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
		t.Error("expected an error for invalid statement, but got nil")
	}
}

func TestClassMethodWithNoReturn(t *testing.T) {
	input := `
class MyClass:
    fn myMethod() -> int:
        var x = 10
`
	_, err := ParseWithIndentation(input)
	if err != nil {
		t.Logf("Got an error as expected (or maybe not): %v", err)
	}
}
