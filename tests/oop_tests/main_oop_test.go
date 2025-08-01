package oop_tests

import (
	"fmt"
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
        this.age = newAge

    fn setInfo(int newAge, string newName) -> void:
        this.age   = newAge
        this.name  = newName

    fn getAge() -> int:
        print "Age is %d" | this.age
        return this.age

Cat myCat = new Cat()
myCat.setAge(10)
int age = myCat.getAge()
myCat.setInfo(8, "Whiskers")
age = myCat.getAge()
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

// Tests that constructor init() works correctly, including field assignments and side effects
func TestConstructorInitialization(t *testing.T) {
	input := `class TestClass:
    init(int x, int y):
        this.x = x
        this.y = y
        print "Initializing with x=%d, y=%d" | x, y
        this.z = x + y
        print "Sum is %d" | this.z

TestClass obj = new TestClass(5, 3)
print "Final values - x: %d, y: %d, z: %d" | obj.x, obj.y, obj.z`

	program, err := lexer.ParseWithIndentation(input)
	if err != nil {
		t.Fatalf("Failed to parse input: %v", err)
	}

	result := renderer.RenderC(program, ".")
	expectedConstructor := "TestClass* TestClass_new(int x, int y) {"
	if !strings.Contains(result, expectedConstructor) {
		t.Errorf("Expected constructor signature not found. Expected: %s", expectedConstructor)
	}

	expectedInits := []string{
		"obj->x = x;",
		"obj->y = y;",
		"obj->z = x + y;",
	}

	for _, init := range expectedInits {
		if !strings.Contains(result, init) {
			t.Errorf("Expected field initialization '%s' not found in constructor", init)
		}
	}
	expectedPrints := []string{
		`printf("Initializing with x=%d, y=%d\n", x, y);`,
		`printf("Sum is %d\n", this->z);`,
	}

	for _, printStmt := range expectedPrints {
		fmt.Println(result)
		if !strings.Contains(result, printStmt) {
			t.Errorf("Expected print statement '%s' not found in constructor", printStmt)
		}
	}
	expectedMainCall := "TestClass* obj = TestClass_new(5, 3);"
	if !strings.Contains(result, expectedMainCall) {
		t.Errorf("Expected object creation with arguments not found. Expected: %s", expectedMainCall)
	}
}

// Tests method calls with multiple arguments
func TestMethodCallsWithMultipleArgs(t *testing.T) {
	input := `class Calculator:
    init:
        int this.result = 0

    fn add(int a, int b, int c) -> void:
        this.result = a + b + c

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
