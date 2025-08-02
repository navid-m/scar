package renderer

import (
	"fmt"
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

func TestTopLevelStringLiteralQuotes(t *testing.T) {
	input := `string code = "++++++++[>++++[>++>+++>+++>+<<<<-]>+>+>->>+[<]<-]>>.>---.+++++++..+++.>>.<-.<.+++.------.--------.>>+.>++."`

	program, err := lexer.ParseWithIndentation(input)
	if err != nil {
		t.Fatalf("Failed to parse input: %v", err)
	}
	var (
		result       = RenderC(program, ".")
		expectedDecl = `char code[256];`
		expectedInit = `strcpy(code, "++++++++[>++++[>++>+++>+++>+<<<<-]>+>+>->>+[<]<-]>>.>---.+++++++..+++.>>.<-.<.+++.------.--------.>>+.>++.");`
	)
	if !strings.Contains(result, expectedDecl) {
		t.Errorf("Expected string declaration '%s' not found in generated code", expectedDecl)
	}
	if !strings.Contains(result, expectedInit) {
		t.Errorf("Expected string initialization '%s' not found in generated code, got:\n%s", expectedInit, result)
	}
}

func TestRenderCWithImports(t *testing.T) {
	mathModule := &lexer.ModuleInfo{
		Name:     "math",
		FilePath: "math.scar",
		PublicVars: map[string]*lexer.VarDeclStmt{
			"PI": {
				Type:  "int",
				Name:  "PI",
				Value: "3",
			},
		},
		PublicClasses: map[string]*lexer.ClassDeclStmt{
			"Calculator": {
				Name: "Calculator",
				Constructor: &lexer.ConstructorStmt{
					Parameters: []*lexer.MethodParameter{},
					Fields:     []*lexer.Statement{},
				},
				Methods: []*lexer.MethodDeclStmt{
					{
						Name: "add",
						Parameters: []*lexer.MethodParameter{
							{Type: "int", Name: "a"},
							{Type: "int", Name: "b"},
						},
						ReturnType: "int",
						Body: []*lexer.Statement{
							{
								Return: &lexer.ReturnStmt{
									Value: "a + b",
								},
							},
						},
					},
				},
			},
		},
		PublicFuncs: map[string]*lexer.MethodDeclStmt{},
	}

	lexer.LoadedModules["math"] = mathModule

	program := &lexer.Program{
		Imports: []*lexer.ImportStmt{
			{Module: "math"},
		},
		Statements: []*lexer.Statement{
			{
				VarDecl: &lexer.VarDeclStmt{
					Type:  "int",
					Name:  "area",
					Value: "math.PI * 5 * 5",
				},
			},
			{
				ObjectDecl: &lexer.ObjectDeclStmt{
					Name: "calc",
					Type: "math.Calculator",
					Args: []string{"math", "Calculator"},
				},
			},
			{
				VarDeclMethodCall: &lexer.VarDeclMethodCallStmt{
					Type:   "int",
					Name:   "result",
					Object: "calc",
					Method: "add",
					Args:   []string{"10", "20"},
				},
			},
			{
				Print: &lexer.PrintStmt{
					Format:    "Area: %d",
					Variables: []string{"area"},
				},
			},
			{
				Print: &lexer.PrintStmt{
					Format:    "Result: %d",
					Variables: []string{"result"},
				},
			},
		},
	}

	cCode := RenderC(program, "")

	expectedVarDecl := "int area = math_PI * 5 * 5;"
	if !strings.Contains(cCode, expectedVarDecl) {
		t.Errorf("Expected imported symbol resolution '%s' not found in generated code", expectedVarDecl)
	}
	expectedStructDef := "typedef struct math_Calculator {"
	if !strings.Contains(cCode, expectedStructDef) {
		t.Errorf("Expected imported class struct definition '%s' not found", expectedStructDef)
	}
	expectedObjectCreation := "math_Calculator* calc = math_Calculator_new();"
	if !strings.Contains(cCode, expectedObjectCreation) {
		t.Errorf("Expected clean object creation '%s' not found", expectedObjectCreation)
	}
	expectedMethodCall := "int result = math_Calculator_add(calc, 10, 20);"
	if !strings.Contains(cCode, expectedMethodCall) {
		t.Errorf("Expected method call '%s' not found", expectedMethodCall)
	}
	expectedPublicVar := "extern int math_PI;"
	if !strings.Contains(cCode, expectedPublicVar) {
		t.Errorf("Expected public variable declaration '%s' not found", expectedPublicVar)
	}
	expectedPublicVarDef := "int math_PI = 3;"
	if !strings.Contains(cCode, expectedPublicVarDef) {
		t.Errorf("Expected public variable definition '%s' not found", expectedPublicVarDef)
	}
	invalidPatterns := []string{
		"math.PI",
		"math, Calculator",
		"new math.Calculator",
	}

	for _, pattern := range invalidPatterns {
		if strings.Contains(cCode, pattern) {
			t.Errorf("Found invalid C code pattern '%s' in generated code", pattern)
		}
	}

	delete(lexer.LoadedModules, "math")
}

func TestThisPointerSyntax(t *testing.T) {
	input := `pub class Soprano:
    init:
        string  this.name = "vito"
        int     this.annoyingness_level = 10
        int     this.singing_ability = 100
        string  this.singing_style = "opera"
        int     this.is_singing = 0
        int     this.is_annoying = 1
        int     this.is_awesome = 0
    fn sing() -> void:
        if this.is_annoying == 1:
            print "LAAAAAA OOOOO EEEEE AAAA OOO!!!!"
        this.is_singing = 1
        while this.singing_ability > 50:
            this.annoyingness_level = this.annoyingness_level + 1
            break
    fn get_name() -> string:
        return this.name
var tony = new Soprano()
tony.sing()`

	program, err := lexer.ParseWithIndentation(input)
	if err != nil {
		t.Fatalf("Failed to parse input: %v", err)
	}
	result := RenderC(program, ".")
	expectedIfCondition := "if (this->is_annoying == 1)"
	if !strings.Contains(result, expectedIfCondition) {
		t.Errorf("Expected if condition with pointer syntax '%s' not found in generated code", expectedIfCondition)
	}
	expectedWhileCondition := "while (this->singing_ability > 50)"
	if !strings.Contains(result, expectedWhileCondition) {
		t.Errorf("Expected while condition with pointer syntax '%s' not found in generated code", expectedWhileCondition)
	}
	expectedAssignment := "this->is_singing = 1;"
	if !strings.Contains(result, expectedAssignment) {
		t.Errorf("Expected assignment with pointer syntax '%s' not found in generated code", expectedAssignment)
	}
	expectedComplexAssignment := "this->annoyingness_level = this->annoyingness_level + 1;"
	if !strings.Contains(result, expectedComplexAssignment) {
		t.Errorf("Expected complex assignment with pointer syntax '%s' not found in generated code", expectedComplexAssignment)
	}
	expectedReturn := "return this->name;"
	if !strings.Contains(result, expectedReturn) {
		t.Errorf("Expected return with pointer syntax '%s' not found in generated code", expectedReturn)
	}
	invalidPatterns := []string{
		"this.is_annoying",
		"this.singing_ability",
		"this.is_singing",
		"this.annoyingness_level",
		"this.name",
	}
	for _, pattern := range invalidPatterns {
		if strings.Contains(result, pattern) {
			t.Errorf("Found invalid dot syntax '%s' in generated code, should be converted to pointer syntax", pattern)
		}
	}
}

func TestGlobalVariablesRendering(t *testing.T) {
	input := `pub float PI = 3.14159265359
pub float E = 2.71828182846
pub int MAX_INT = 2147483647
pub int MIN_INT = -2147483648
pub string GREETING = "Hello World"

pub fn calculate_area(float radius) -> float:
    return PI * radius * radius

var result = calculate_area(5.0)
print "Area: {}", result`

	program, err := lexer.ParseWithIndentation(input)
	if err != nil {
		t.Fatalf("Failed to parse input: %v", err)
	}

	result := RenderC(program, ".")
	expectedDeclarations := []string{
		"float PI = 3.14159265359;",
		"float E = 2.71828182846;",
		"int MAX_INT = 2147483647;",
		"int MIN_INT = -2147483648;",
		"char GREETING[256];",
	}

	for _, expected := range expectedDeclarations {
		if !strings.Contains(result, expected) {
			t.Errorf("Expected global variable declaration '%s' not found in generated code", expected)
		}
	}

	expectedStringInit := "void init_GREETING() { strcpy(GREETING, \"Hello World\"); }"
	if !strings.Contains(result, expectedStringInit) {
		t.Errorf("Expected string variable initialization '%s' not found in generated code", expectedStringInit)
	}
	expectedMainInit := "init_GREETING();"
	if !strings.Contains(result, expectedMainInit) {
		t.Errorf("Expected string initialization call '%s' not found in main function", expectedMainInit)
	}
	expectedFunctionDecl := "float calculate_area(float radius);"
	if !strings.Contains(result, expectedFunctionDecl) {
		t.Errorf("Expected function declaration '%s' not found", expectedFunctionDecl)
	}
	expectedPIUsage := "return PI * radius * radius;"
	if !strings.Contains(result, expectedPIUsage) {
		t.Errorf("Expected PI usage '%s' not found in function implementation", expectedPIUsage)
	}
	invalidPatterns := []string{
		"pub float PI",
		"pub int MAX",
		"pub string",
	}
	for _, pattern := range invalidPatterns {
		if strings.Contains(result, pattern) {
			t.Errorf("Found invalid pattern '%s' in generated C code", pattern)
		}
	}
	if !strings.Contains(result, "int main() {") {
		t.Error("Expected main function declaration not found")
	}
	if !strings.Contains(result, "return 0;") {
		t.Error("Expected main function return statement not found")
	}
}

func TestMixedGlobalVariableTypes(t *testing.T) {
	input := `pub int counter = 0
pub bool is_active = true
pub float temperature = 98.6
pub string status = "running"

pub fn update_status() -> void:
    counter = counter + 1
    if counter > 10:
        is_active = false
        status = "stopped"

update_status()
print "Counter: {}, Active: {}, Temp: {}, Status: {}", counter, is_active, temperature, status`

	program, err := lexer.ParseWithIndentation(input)
	if err != nil {
		t.Fatalf("Failed to parse input: %v", err)
	}

	result := RenderC(program, ".")
	fmt.Println(result)
	expectedDeclarations := []string{
		"int counter = 0;",
		"int is_active = true;",
		"float temperature = 98.6;",
		"char status[256];",
	}

	for _, expected := range expectedDeclarations {
		if !strings.Contains(result, expected) {
			t.Errorf("Expected declaration '%s' not found in generated code", expected)
		}
	}

	expectedStringInit := "void init_status() { strcpy(status, \"running\"); }"
	if !strings.Contains(result, expectedStringInit) {
		t.Errorf("Expected string initialization '%s' not found", expectedStringInit)
	}

	expectedAssignment := "counter = counter + 1;"
	if !strings.Contains(result, expectedAssignment) {
		t.Errorf("Expected global variable assignment '%s' not found", expectedAssignment)
	}

	expectedCondition := "if (counter > 10)"
	if !strings.Contains(result, expectedCondition) {
		t.Errorf("Expected global variable condition '%s' not found", expectedCondition)
	}
}
