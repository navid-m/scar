package main

import (
	"scar/lexer"
	"scar/renderer"
	"strings"
	"testing"
)

// Ensures that standalone method calls are properly included in the generated C code (regression test for bug
// where myCat.setAge(10), myCat.setInfo(8, "Whiskers"), etc. were missing).
func TestMethodCallsInGeneratedCode(t *testing.T) {
	input := `class Cat:
    init:
        int     this.age  = 5
        string  this.name = "Fluffy"

    fn setAge(int newAge) -> void:
        reassign this.age = newAge

    fn setInfo(int newAge, string newName) -> void:
        reassign this.age   = newAge
        reassign this.name  = newName

    fn getAge() -> int:
        print "Age is %d" | this.age
        return this.age

Cat myCat = new Cat()
myCat.setAge(10)
int age = myCat.getAge()
myCat.setInfo(8, "Whiskers")
reassign age = myCat.getAge()
print "The age was %d" | age`

	program, err := lexer.ParseWithIndentation(input)
	if err != nil {
		t.Fatalf("Failed to parse input: %v", err)
	}

	result := renderer.RenderC(program, ".")
	expectedCalls := []string{
		"Cat_setAge(myCat, 10);",
		"Cat_setInfo(myCat, 8, \"Whiskers\");",
		"Cat_getAge(myCat)",
	}

	for _, expectedCall := range expectedCalls {
		if !strings.Contains(result, expectedCall) {
			t.Errorf("Expected method call '%s' not found in generated C code.\nGenerated code:\n%s", expectedCall, result)
		}
	}
	mainFunctionStart := strings.Index(result, "int main() {")
	if mainFunctionStart == -1 {
		t.Fatal("main function not found in generated C code")
	}

	mainFunction := result[mainFunctionStart:]

	standaloneCalls := []string{
		"Cat_setAge(myCat, 10);",
		"Cat_setInfo(myCat, 8, \"Whiskers\");",
	}

	for _, call := range standaloneCalls {
		if !strings.Contains(mainFunction, call) {
			t.Errorf("Standalone method call '%s' not found in main function.\nMain function:\n%s", call, mainFunction)
		}
	}
}

// Tests method calls that have no arguments
func TestMethodCallsWithoutArgs(t *testing.T) {
	input := `class TestClass:
    init:
        int this.value = 42

    fn doSomething() -> void:
        print "Doing something"

TestClass obj = new TestClass()
obj.doSomething()`

	program, err := lexer.ParseWithIndentation(input)
	if err != nil {
		t.Fatalf("Failed to parse input: %v", err)
	}

	result := renderer.RenderC(program, ".")
	expectedCall := "TestClass_doSomething(obj);"
	if !strings.Contains(result, expectedCall) {
		t.Errorf("Expected method call '%s' not found in generated C code.\nGenerated code:\n%s", expectedCall, result)
	}
}

// Tests method calls with multiple arguments
func TestMethodCallsWithMultipleArgs(t *testing.T) {
	input := `class Calculator:
    init:
        int this.result = 0

    fn add(int a, int b, int c) -> void:
        reassign this.result = a + b + c

Calculator calc = new Calculator()
calc.add(1, 2, 3)`

	program, err := lexer.ParseWithIndentation(input)
	if err != nil {
		t.Fatalf("Failed to parse input: %v", err)
	}
	result := renderer.RenderC(program, ".")
	expectedCall := "Calculator_add(calc, 1, 2, 3);"
	if !strings.Contains(result, expectedCall) {
		t.Errorf("Expected method call '%s' not found in generated C code.\nGenerated code:\n%s", expectedCall, result)
	}
}
