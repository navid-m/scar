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

func TestRenderCWithForLoop(t *testing.T) {
	program := &lexer.Program{
		Statements: []*lexer.Statement{
			{
				For: &lexer.ForStmt{
					Var:   "i",
					Start: "0",
					End:   "9",
					Body: []*lexer.Statement{
						{
							Print: &lexer.PrintStmt{
								Format:    "i is %d",
								Variables: []string{"i"},
							},
						},
					},
				},
			},
		},
	}

	cCode := RenderC(program, "")
	expected := `for (int i = 0; i <= 9; i++) {
    printf("i is %d\n", i);
    }`

	// Normalize whitespace for comparison
	normalizedCCode := strings.Join(strings.Fields(cCode), " ")
	normalizedExpected := strings.Join(strings.Fields(expected), " ")

	if !strings.Contains(normalizedCCode, normalizedExpected) {
		t.Errorf("Expected C code to contain '%s', but got '%s'", normalizedExpected, normalizedCCode)
	}
}

func TestRenderCWithWhileLoop(t *testing.T) {
	program := &lexer.Program{
		Statements: []*lexer.Statement{
			{
				VarDecl: &lexer.VarDeclStmt{
					Name:  "x",
					Type:  "int",
					Value: "5",
				},
			},
			{
				While: &lexer.WhileStmt{
					Condition: "x > 0",
					Body: []*lexer.Statement{
						{
							Print: &lexer.PrintStmt{
								Format:    "x is %d",
								Variables: []string{"x"},
							},
						},
						{
							VarAssign: &lexer.VarAssignStmt{
								Name:  "x",
								Value: "x - 1",
							},
						},
					},
				},
			},
		},
	}

	cCode := RenderC(program, "")
	expected := `int x = 5;
    while (x > 0) {
    printf("x is %d\n", x);
    x = x - 1;
    }`

	var (
		normalizedCCode    = strings.Join(strings.Fields(cCode), " ")
		normalizedExpected = strings.Join(strings.Fields(expected), " ")
	)

	if !strings.Contains(normalizedCCode, normalizedExpected) {
		t.Errorf("Expected C code to contain '%s', but got '%s'", normalizedExpected, normalizedCCode)
	}
}

func TestRenderCWithStringVariable(t *testing.T) {
	program := &lexer.Program{
		Statements: []*lexer.Statement{
			{
				VarDecl: &lexer.VarDeclStmt{
					Name:  "msg",
					Type:  "string",
					Value: "\"Hello, String!\"",
				},
			},
			{
				Print: &lexer.PrintStmt{
					Format:    "%s",
					Variables: []string{"msg"},
				},
			},
		},
	}

	cCode := RenderC(program, "")
	expectedVarDecl := `char msg[256]`
	expectedStrcpy := `strcpy(msg, "Hello, String!");`
	expectedPrintf := `printf("%s\n", msg);`

	if !strings.Contains(cCode, expectedVarDecl) {
		t.Errorf("Expected C code to contain '%s', but it didn't", expectedVarDecl)
	}
	if !strings.Contains(cCode, expectedStrcpy) {
		t.Errorf("Expected C code to contain '%s', but it didn't", expectedStrcpy)
	}
	if !strings.Contains(cCode, expectedPrintf) {
		t.Errorf("Expected C code to contain '%s', but it didn't", expectedPrintf)
	}
}

func TestRenderCWithList(t *testing.T) {
	program := &lexer.Program{
		Statements: []*lexer.Statement{
			{
				ListDecl: &lexer.ListDeclStmt{
					Name:     "myList",
					Type:     "int",
					Elements: []string{"1", "2", "3"},
				},
			},
			{
				Print: &lexer.PrintStmt{
					Format:    "%d",
					Variables: []string{"myList[1]"},
				},
			},
		},
	}

	cCode := RenderC(program, "")
	expectedListDecl := `int myList[3];`
	expectedListInit1 := `myList[0] = 1;`
	expectedListInit2 := `myList[1] = 2;`
	expectedListInit3 := `myList[2] = 3;`
	expectedPrintf := `printf("%d\n", myList[1]);`

	if !strings.Contains(cCode, expectedListDecl) {
		t.Errorf("Expected C code to contain '%s', but it didn't", expectedListDecl)
	}
	if !strings.Contains(cCode, expectedListInit1) {
		t.Errorf("Expected C code to contain '%s', but it didn't", expectedListInit1)
	}
	if !strings.Contains(cCode, expectedListInit2) {
		t.Errorf("Expected C code to contain '%s', but it didn't", expectedListInit2)
	}
	if !strings.Contains(cCode, expectedListInit3) {
		t.Errorf("Expected C code to contain '%s', but it didn't", expectedListInit3)
	}
	if !strings.Contains(cCode, expectedPrintf) {
		t.Errorf("Expected C code to contain '%s', but it didn't", expectedPrintf)
	}
}

func TestRenderCWithMap(t *testing.T) {
	program := &lexer.Program{
		Statements: []*lexer.Statement{
			{
				MapDecl: &lexer.MapDeclStmt{
					Name:      "myMap",
					KeyType:   "string",
					ValueType: "int",
					Pairs: []lexer.MapPair{
						{Key: "\"one\"", Value: "1"},
						{Key: "\"two\"", Value: "2"},
					},
				},
			},
		},
	}

	cCode := RenderC(program, "")
	expectedKeyDecl := `char myMap_keys[2][256]`
	expectedValueDecl := `int myMap_values[2]`
	expectedSize := `int myMap_size = 2`
	expectedKeyInit1 := `strcpy(myMap_keys[0], "one")`
	expectedValueInit1 := `myMap_values[0] = 1`
	expectedKeyInit2 := `strcpy(myMap_keys[1], "two")`
	expectedValueInit2 := `myMap_values[1] = 2`

	if !strings.Contains(cCode, expectedKeyDecl) {
		t.Errorf("Expected C code to contain '%s', but it didn't", expectedKeyDecl)
	}
	if !strings.Contains(cCode, expectedValueDecl) {
		t.Errorf("Expected C code to contain '%s', but it didn't", expectedValueDecl)
	}
	if !strings.Contains(cCode, expectedSize) {
		t.Errorf("Expected C code to contain '%s', but it didn't", expectedSize)
	}
	if !strings.Contains(cCode, expectedKeyInit1) {
		t.Errorf("Expected C code to contain '%s', but it didn't", expectedKeyInit1)
	}
	if !strings.Contains(cCode, expectedValueInit1) {
		t.Errorf("Expected C code to contain '%s', but it didn't", expectedValueInit1)
	}
	if !strings.Contains(cCode, expectedKeyInit2) {
		t.Errorf("Expected C code to contain '%s', but it didn't", expectedKeyInit2)
	}
	if !strings.Contains(cCode, expectedValueInit2) {
		t.Errorf("Expected C code to contain '%s', but it didn't", expectedValueInit2)
	}
}

func TestRenderCWithObjectConstructor(t *testing.T) {
	program := &lexer.Program{
		Statements: []*lexer.Statement{
			{
				ClassDecl: &lexer.ClassDeclStmt{
					Name: "TestClass",
					Constructor: &lexer.ConstructorStmt{
						Fields: []*lexer.Statement{
							{
								VarDecl: &lexer.VarDeclStmt{
									Type:  "int",
									Name:  "this.value",
									Value: "42",
								},
							},
						},
					},
				},
			},
			{
				ObjectDecl: &lexer.ObjectDeclStmt{
					Type: "TestClass",
					Name: "obj",
				},
			},
		},
	}

	cCode := RenderC(program, "")
	if !strings.Contains(cCode, "TestClass* this = malloc(sizeof(TestClass));") {
		t.Error("Expected constructor to declare 'this' pointer")
	}

	if !strings.Contains(cCode, "this->value = 42;") {
		t.Error("Expected constructor to use 'this' to set member variable")
	}
}

func TestRenderCWithStringList(t *testing.T) {
	program := &lexer.Program{
		Statements: []*lexer.Statement{
			{
				ListDecl: &lexer.ListDeclStmt{
					Name:     "names",
					Type:     "string",
					Elements: []string{"\"Alice\"", "\"Bob\"", "\"Charlie\""},
				},
			},
			{
				VarAssign: &lexer.VarAssignStmt{
					Name:  "names[2]",
					Value: "\"David\"",
				},
			},
			{
				Print: &lexer.PrintStmt{
					Format:    "Name: %s",
					Variables: []string{"names[0]"},
				},
			},
		},
	}

	cCode := RenderC(program, "")
	expectedListDecl := `char names[3][256]`
	if !strings.Contains(cCode, expectedListDecl) {
		t.Errorf("Expected C code to contain '%s', but it didn't", expectedListDecl)
	}
	var (
		expectedInit1 = `strcpy(names[0], "Alice")`
		expectedInit2 = `strcpy(names[1], "Bob")`
		expectedInit3 = `strcpy(names[2], "Charlie")`
	)
	if !strings.Contains(cCode, expectedInit1) {
		t.Errorf("Expected C code to contain '%s', but it didn't", expectedInit1)
	}
	if !strings.Contains(cCode, expectedInit2) {
		t.Errorf("Expected C code to contain '%s', but it didn't", expectedInit2)
	}
	if !strings.Contains(cCode, expectedInit3) {
		t.Errorf("Expected C code to contain '%s', but it didn't", expectedInit3)
	}
	expectedAssign := `strcpy(names[2], "David")`
	if !strings.Contains(cCode, expectedAssign) {
		t.Errorf("Expected C code to contain '%s', but it didn't", expectedAssign)
	}
	expectedPrintf := `printf("Name: %s\n", names[0])`
	if !strings.Contains(cCode, expectedPrintf) {
		t.Errorf("Expected C code to contain '%s', but it didn't", expectedPrintf)
	}
}

func TestConstructorStringLiteralQuotes(t *testing.T) {
	input := `class Cat:
    init():
        string this.name = "Fluffy"

Cat fluffy = new Cat()
`

	program, err := lexer.ParseWithIndentation(input)
	if err != nil {
		t.Fatalf("Failed to parse input: %v", err)
	}

	result := RenderC(program, ".")
	expectedConstructor := "Cat* Cat_new() {"
	if !strings.Contains(result, expectedConstructor) {
		t.Errorf("Expected constructor signature not found. Expected: %s", expectedConstructor)
	}

	expectedInit := `strcpy(this->name, "Fluffy");`
	if !strings.Contains(result, expectedInit) {
		t.Errorf("Expected string field initialization '%s' not found in constructor", expectedInit)
	}

	expectedObjectCreation := `Cat* fluffy = Cat_new();`
	if !strings.Contains(result, expectedObjectCreation) {
		t.Errorf("Expected object creation with quoted string not found. Expected: %s", expectedObjectCreation)
	}
}
