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
	t.Run("map with elements", func(t *testing.T) {
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

		var (
			cCode              = RenderC(program, "")
			expectedKeyDecl    = `char myMap_keys[2][256]`
			expectedValueDecl  = `int myMap_values[2]`
			expectedSize       = `int myMap_size = 2`
			expectedKeyInit1   = `strcpy(myMap_keys[0], "one")`
			expectedValueInit1 = `myMap_values[0] = 1`
			expectedKeyInit2   = `strcpy(myMap_keys[1], "two")`
			expectedValueInit2 = `myMap_values[1] = 2`
		)
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
	})

	t.Run("empty map", func(t *testing.T) {
		program := &lexer.Program{
			Statements: []*lexer.Statement{
				{
					MapDecl: &lexer.MapDeclStmt{
						Name:      "emptyMap",
						KeyType:   "string",
						ValueType: "int",
						Pairs:     []lexer.MapPair{},
					},
				},
			},
		}

		var (
			cCode        = RenderC(program, "")
			expectedCode = []string{
				`char emptyMap_keys[10][256];`,
				`int emptyMap_values[10];`,
				`int emptyMap_size = 0;`,
			}
			unexpectedCode = []string{
				`strcpy(emptyMap_keys[`,
			}
		)

		for _, code := range expectedCode {
			if !strings.Contains(cCode, code) {
				t.Errorf("Expected C code to contain '%s', but it didn't", code)
			}
		}

		for _, code := range unexpectedCode {
			if strings.Contains(cCode, code) {
				if !strings.Contains(cCode, code+"]") &&
					!strings.Contains(cCode, code+" ") &&
					!strings.Contains(cCode, code+";") {
					t.Errorf("Expected C code to not contain '%s' for empty map, but it did", code)
				}
			}
		}
	})
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

	expectedAreaCalc := "area = math_PI * 5 * 5;"
	if !strings.Contains(cCode, expectedAreaCalc) {
		t.Errorf("Expected area calculation '%s' not found in generated code", expectedAreaCalc)
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

func TestSchedulerExample(t *testing.T) {
	input := `class Task:
    init(string name, int priority, int duration):
        this.name = name
        this.priority = priority
        this.duration = duration
        this.completed = false
        this.start_time = 0

    fn execute() -> void:
        this.start_time = 1
        for i = 1 to this.duration:
            i = i + 1
        this.completed = true

class TaskScheduler:
    init():
        this.total_tasks = 0
        this.completed_tasks = 0

    fn add_task(Task task) -> void:
        this.total_tasks = this.total_tasks + 1

    fn run_scheduler() -> void:
        var task1 = new Task("Task 1", 1, 1)
        this.add_task(task1)
        task1.execute()
`

	program, err := lexer.ParseWithIndentation(input)
	if err != nil {
		t.Fatalf("Failed to parse input: %v", err)
	}

	result := RenderC(program, ".")

	expectedPatterns := []string{
		"this->name",
		"this->priority",
		"this->duration",
		"this->completed",
		"this->start_time",
		"this->total_tasks",
		"this->completed_tasks",
	}

	for _, pattern := range expectedPatterns {
		if !strings.Contains(result, pattern) {
			t.Errorf("Expected pattern '%s' not found in generated code", pattern)
		}
	}

	if strings.Contains(result, "unknown_add_task") {
		t.Error("Found 'unknown_add_task' in generated code, method resolution failed")
	}
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

func TestMatrixExample(t *testing.T) {
	program := &lexer.Program{
		Statements: []*lexer.Statement{
			{
				ClassDecl: &lexer.ClassDeclStmt{
					Name: "Matrix",
					Constructor: &lexer.ConstructorStmt{
						Parameters: []*lexer.MethodParameter{
							{Name: "rows", Type: "int"},
							{Name: "cols", Type: "int"},
						},
						Fields: []*lexer.Statement{
							{
								VarAssign: &lexer.VarAssignStmt{
									Name:  "this.rows",
									Value: "rows",
								},
							},
							{
								VarAssign: &lexer.VarAssignStmt{
									Name:  "this.cols",
									Value: "cols",
								},
							},
							{
								VarAssign: &lexer.VarAssignStmt{
									Name:  "this.size",
									Value: "rows * cols",
								},
							},
						},
					},
					Methods: []*lexer.MethodDeclStmt{
						{
							Name: "init",
							Parameters: []*lexer.MethodParameter{
								{Name: "rows", Type: "int"},
								{Name: "cols", Type: "int"},
							},
							Body: []*lexer.Statement{
								{
									VarAssign: &lexer.VarAssignStmt{
										Name:  "this.rows",
										Value: "rows",
									},
								},
								{
									VarAssign: &lexer.VarAssignStmt{
										Name:  "this.cols",
										Value: "cols",
									},
								},
								{
									VarAssign: &lexer.VarAssignStmt{
										Name:  "this.size",
										Value: "rows * cols",
									},
								},
							},
						},
						{
							Name: "set_value",
							Parameters: []*lexer.MethodParameter{
								{Name: "row", Type: "int"},
								{Name: "col", Type: "int"},
								{Name: "value", Type: "float"},
							},
							ReturnType: "void",
							Body: []*lexer.Statement{
								{
									VarDecl: &lexer.VarDeclStmt{
										Type:  "int",
										Name:  "index",
										Value: "row * this.cols + col",
									},
								},
							},
						},
						{
							Name: "get_value",
							Parameters: []*lexer.MethodParameter{
								{Name: "row", Type: "int"},
								{Name: "col", Type: "int"},
							},
							ReturnType: "float",
							Body: []*lexer.Statement{
								{
									VarDecl: &lexer.VarDeclStmt{
										Type:  "int",
										Name:  "index",
										Value: "row * this.cols + col",
									},
								},
								{
									Return: &lexer.ReturnStmt{
										Value: "float(row + col)",
									},
								},
							},
						},
						{
							Name:       "print_matrix",
							ReturnType: "void",
							Body: []*lexer.Statement{
								{
									For: &lexer.ForStmt{
										Var:   "row",
										Start: "0",
										End:   "this.rows - 1",
										Body: []*lexer.Statement{
											{
												For: &lexer.ForStmt{
													Var:   "col",
													Start: "0",
													End:   "this.cols - 1",
													Body: []*lexer.Statement{
														{
															VarDecl: &lexer.VarDeclStmt{
																Type:  "float",
																Name:  "val",
																Value: "this.get_value(row, col)",
															},
														},
														{
															Print: &lexer.PrintStmt{
																Format:    "%.1f ",
																Variables: []string{"val"},
															},
														},
													},
												},
											},
											{
												Print: &lexer.PrintStmt{
													Print: "\"\n"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	cCode := RenderC(program, "")

	if !strings.Contains(cCode, "this->rows") || !strings.Contains(cCode, "this->cols") {
		t.Error("Expected 'this->' pointer access in generated C code")
	}

	if !strings.Contains(cCode, "Matrix_get_value(this") {
		t.Error("Expected method calls on 'this' to be resolved to class methods")
	}

	if !strings.Contains(cCode, "row <= (this->rows - 1)") || !strings.Contains(cCode, "col <= (this->cols - 1)") {
		t.Error("Expected for loop conditions to use 'this->' pointer access")
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

func TestMethodCallOnThis(t *testing.T) {
	program := &lexer.Program{
		Statements: []*lexer.Statement{
			{
				ClassDecl: &lexer.ClassDeclStmt{
					Name: "TestClass",
					Methods: []*lexer.MethodDeclStmt{
						{
							Name: "method1",
							Parameters: []*lexer.MethodParameter{
								{Name: "self", Type: "TestClass*"},
							},
							Body: []*lexer.Statement{
								{
									VarDeclMethodCall: &lexer.VarDeclMethodCallStmt{
										Type:   "int",
										Name:   "result",
										Object: "this",
										Method: "internal_method",
										Args:   []string{"42"},
									},
								},
							},
						},
						{
							Name: "internal_method",
							Parameters: []*lexer.MethodParameter{
								{Name: "self", Type: "TestClass*"},
								{Name: "value", Type: "int"},
							},
							ReturnType: "int",
							Body: []*lexer.Statement{
								{
									Return: &lexer.ReturnStmt{
										Value: "value",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	cCode := RenderC(program, "")
	tests := []string{
		"TestClass_internal_method",
		"result = TestClass_internal_method",
	}

	for _, test := range tests {
		if !strings.Contains(cCode, test) {
			t.Errorf("Expected C code to contain '%s', but it didn't. Full code:\n%s", test, cCode)
		}
	}
}

// func TestClassMemberAndForLoop(t *testing.T) {
// 	input := `pub class GameOfLife:
//     init(int width, int height):
//         this.width = width
//         this.height = height
//         this.current_generation = 0
//         int size = width * height
//         list[int] grid = [0] * size
//         list[int] next_grid = [0] * size
//         for int i = 0 to (size - 1):
//             grid[i] = 0
//             next_grid[i] = 0
//         this.grid = grid
//         this.next_grid = next_grid

//     fn index(int x, int y) -> int:
//         return y * this.width + x

//     fn get_cell(int x, int y) -> int:
//         int i = this.index(x, y)
//         return this.grid[i]

//     fn set_cell(int x, int y, int value) -> void:
//         int i = this.index(x, y)
//         this.grid[i] = value

//     fn set_next_cell(int x, int y, int value) -> void:
//         int i = this.index(x, y)
//         this.next_grid[i] = value

//     fn initialize_random() -> void:
//         print "Initializing random grid %dx%d..." | this.width, this.height
//         for int y = 0 to (this.height - 1):
//             for int x = 0 to (this.width - 1):
//                 int val = rand(0, 1)
//                 this.set_cell(x, y, val)
//                 print "%d " | val
//             print ""
//         print "Generation %d initialized" | this.current_generation

//     fn count_neighbors(int x, int y) -> int:
//         int count = 0
//         for int dy = -1 to 1:
//             for int dx = -1 to 1:
//                 if dx == 0 && dy == 0:
//                     continue
//                 int nx = x + dx
//                 int ny = y + dy
//                 if nx >= 0 && nx < this.width && ny >= 0 && ny < this.height:
//                     if this.get_cell(nx, ny) == 1:
//                         count = count + 1
//         return count

//     fn next_generation() -> void:
//         print "Computing generation %d..." | (this.current_generation + 1)
//         for int y = 0 to (this.height - 1):
//             for int x = 0 to (this.width - 1):
//                 int neighbors = this.count_neighbors(x, y)
//                 int current = this.get_cell(x, y)
//                 if current == 1:
//                     if neighbors < 2 || neighbors > 3:
//                         this.set_next_cell(x, y, 0)
//                     else:
//                         this.set_next_cell(x, y, 1)
//                 else:
//                     if neighbors == 3:
//                         this.set_next_cell(x, y, 1)
//                     else:
//                         this.set_next_cell(x, y, 0)

//         for int i = 0 to ((this.width * this.height) - 1):
//             this.grid[i] = this.next_grid[i]

//         this.current_generation = this.current_generation + 1

//     fn print_grid() -> void:
//         print "Current grid (Generation %d):" | this.current_generation
//         for int y = 0 to (this.height - 1):
//             for int x = 0 to (this.width - 1):
//                 print "%d " | this.get_cell(x, y)
//             print ""

//     fn run_simulation(int generations) -> void:
//         this.initialize_random()
//         this.print_grid()
//         for int gen = 1 to generations:
//             this.next_generation()
//             this.print_grid()`

// 	program, err := lexer.ParseWithIndentation(input)
// 	if err != nil {
// 		t.Fatalf("Failed to parse input: %v", err)
// 	}

// 	result := RenderC(program, ".")
// 	expectedStructFields := []string{
// 		"int width;",
// 		"int height;",
// 		"int current_generation;",
// 		"int* grid;",
// 		"int* next_grid;",
// 	}

// 	for _, field := range expectedStructFields {
// 		if !strings.Contains(result, field) {
// 			t.Errorf("Expected struct field '%s' not found in generated code", field)
// 		}
// 	}
// 	expectedInitializations := []string{
// 		"this->width = width;",
// 		"this->height = height;",
// 		"this->current_generation = 0;",
// 	}

// 	for _, init := range expectedInitializations {
// 		if !strings.Contains(result, init) {
// 			t.Errorf("Expected initialization '%s' not found in generated code", init)
// 		}
// 	}

// 	expectedForLoops := []string{
// 		"for (int i = 0; i <= ((size - 1)); i++)",
// 		"for (int y = 0; y <= ((this->height - 1)); y++)",
// 		"for (int x = 0; x <= ((this->width - 1)); x++)",
// 		"for (int dy = -1; dy <= 1; dy++)",
// 		"for (int dx = -1; dx <= 1; dx++)",
// 		"for (int gen = 1; gen <= generations; gen++)",
// 	}

// 	for _, loop := range expectedForLoops {
// 		if !strings.Contains(result, loop) {
// 			t.Errorf("Expected for loop '%s' not found in generated code", loop)
// 		}
// 	}

// 	expectedMemberAccess := []string{
// 		"this->width",
// 		"this->height",
// 		"this->grid[i]",
// 		"this->next_grid[i]",
// 		"this->current_generation",
// 	}

// 	for _, access := range expectedMemberAccess {
// 		if !strings.Contains(result, access) {
// 			t.Errorf("Expected member access '%s' not found in generated code", access)
// 		}
// 	}

// 	if strings.Contains(result, "int int") {
// 		t.Error("Found duplicate 'int' keywords in for-loop declarations")
// 	}
// 	tmpFile, err := os.CreateTemp("", "generated_code_*.c")
// 	if err != nil {
// 		t.Fatalf("Failed to create temp file: %v", err)
// 	}
// 	defer os.Remove(tmpFile.Name())

// 	if _, err := tmpFile.WriteString(result); err != nil {
// 		t.Fatalf("Failed to write to temp file: %v", err)
// 	}
// 	t.Logf("Full generated code saved to: %s", tmpFile.Name())

// 	invalidPatterns := []string{
// 		"this.width",
// 		"this.height",
// 		"this.grid",
// 		"this.next_grid",
// 		"this.current_generation",
// 	}

// 	for _, pattern := range invalidPatterns {
// 		if strings.Contains(result, pattern) {
// 			var positions []int
// 			start := 0
// 			for {
// 				idx := strings.Index(result[start:], pattern)
// 				if idx == -1 {
// 					break
// 				}
// 				pos := start + idx
// 				positions = append(positions, pos)
// 				start = pos + len(pattern)
// 			}

// 			for _, pos := range positions {
// 				contextStart := max(pos-50, 0)
// 				contextEnd := pos + len(pattern) + 50
// 				if contextEnd > len(result) {
// 					contextEnd = len(result)
// 				}
// 				context := result[contextStart:contextEnd]

// 				t.Logf("\n=== Found invalid dot notation '%s' at position %d ===\n%s\n%s\n%s\n",
// 					pattern, pos,
// 					strings.Repeat("-", 20),
// 					context,
// 					strings.Repeat("-", 20))
// 			}

// 			t.Errorf("Found %d occurrences of invalid dot notation '%s' in generated code, should use pointer syntax",
// 				len(positions), pattern)
// 		}
// 	}
// }

func TestThisMethodCall(t *testing.T) {
	program := &lexer.Program{
		Statements: []*lexer.Statement{
			{
				ClassDecl: &lexer.ClassDeclStmt{
					Name: "GameOfLife",
					Methods: []*lexer.MethodDeclStmt{
						{
							Name: "count_neighbors",
							Parameters: []*lexer.MethodParameter{
								{Name: "x", Type: "int"},
								{Name: "y", Type: "int"},
							},
							ReturnType: "int",
							Body: []*lexer.Statement{
								{
									VarDecl: &lexer.VarDeclStmt{
										Name:  "count",
										Type:  "int",
										Value: "0",
									},
								},
								{
									If: &lexer.IfStmt{
										Condition: "this.get_cell(1, 2) == 1",
										Body: []*lexer.Statement{
											{
												VarAssign: &lexer.VarAssignStmt{
													Name:  "count",
													Value: "count + 1",
												},
											},
										},
									},
								},
								{
									Return: &lexer.ReturnStmt{
										Value: "count",
									},
								},
							},
						},
					},
				},
			},
		},
	}
	currentClassName = "GameOfLife"
	defer func() { currentClassName = "" }()
	code := RenderC(program, "./testdata")
	expected := "GameOfLife_get_cell(this, 1, 2)"
	if !strings.Contains(code, expected) {
		t.Errorf("Expected method call to be converted to '%s', but got:\n%s", expected, code)
	}
}

func TestMethodCallWithComplexExpression(t *testing.T) {
	program := &lexer.Program{
		Statements: []*lexer.Statement{
			{
				ClassDecl: &lexer.ClassDeclStmt{
					Name: "GameOfLife",
					Methods: []*lexer.MethodDeclStmt{
						{
							Name: "test_method",
							Parameters: []*lexer.MethodParameter{
								{Name: "this", Type: "GameOfLife"},
							},
							Body: []*lexer.Statement{
								{
									If: &lexer.IfStmt{
										Condition: "this.get_cell(1, 2) == 1",
										Body: []*lexer.Statement{
											{
												VarDecl: &lexer.VarDeclStmt{
													Name:  "result",
													Type:  "bool",
													Value: "true",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	cCode := RenderC(program, "")
	expected := "GameOfLife_get_cell(this, 1, 2) == 1"
	if !strings.Contains(cCode, expected) {
		t.Errorf("Expected method call to be converted to '%s', got: %s", expected, cCode)
	}
	if strings.Contains(cCode, ")") && strings.Count(cCode, ")") > strings.Count(cCode, "(") {
		t.Errorf("Found mismatched parentheses in generated code: %s", cCode)
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

	expectedDeclarations := []string{
		"int counter = 0;",
		"bool is_active = true;",
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

func TestListOfInlineAndStandalone(t *testing.T) {
	program := &lexer.Program{
		Statements: []*lexer.Statement{
			{
				ListOfDecl: &lexer.ListOfDeclStmt{
					Type:  "string",
					Name:  "single_line",
					Value: "current_line",
				},
			},
			{
				CatList: &lexer.CatListStmt{
					Target: "lines",
					Lists:  []string{"lines", "single_line"},
				},
			},
			{
				CatList: &lexer.CatListStmt{
					Target: "lines2",
					Lists:  []string{"lines", "list_of!(current_line)"},
				},
			},
			{
				CatList: &lexer.CatListStmt{
					Target: "",
					Lists:  []string{"lines", "list_of!(another_line)"},
				},
			},
			{
				ListOfDecl: &lexer.ListOfDeclStmt{
					Type:  "int",
					Name:  "single_num",
					Value: "42",
				},
			},
			{
				CatList: &lexer.CatListStmt{
					Target: "numbers",
					Lists:  []string{"existing_numbers", "list_of!(99)"},
				},
			},
		},
	}

	globalArrays = map[string]string{
		"lines":            "string",
		"existing_numbers": "int",
	}

	result := RenderC(program, "")

	expectedStandaloneString := []string{
		"char single_line[1000][256];",
		"int single_line_len = 1;",
		"strcpy(single_line[0], current_line);",
	}

	for _, expected := range expectedStandaloneString {
		if !strings.Contains(result, expected) {
			t.Errorf("Expected standalone string list_of code '%s' not found in:\n%s", expected, result)
		}
	}

	expectedStandaloneInt := []string{
		"int single_num[1000];",
		"int single_num_len = 1;",
		"single_num[0] = 42;",
	}

	for _, expected := range expectedStandaloneInt {
		if !strings.Contains(result, expected) {
			t.Errorf("Expected standalone int list_of code '%s' not found in:\n%s", expected, result)
		}
	}
	expectedInlineWithTarget := []string{
		"char lines2[1000][256]; // Concatenated list",
		"int lines2_len = 0;",
		"// Add single element from list_of!(current_line)",
		"if (lines2_len < 1000) {",
		"strcpy(lines2[lines2_len], current_line);",
		"lines2_len++;",
	}
	for _, expected := range expectedInlineWithTarget {
		if !strings.Contains(result, expected) {
			t.Errorf("Expected inline list_of with target code '%s' not found in:\n%s", expected, result)
		}
	}
	expectedInlineWithoutTarget := []string{
		"// Add single element from list_of!(another_line)",
		"if (lines_len < 1000) {",
		"strcpy(lines[lines_len], another_line);",
		"lines_len++;",
	}

	for _, expected := range expectedInlineWithoutTarget {
		if !strings.Contains(result, expected) {
			t.Errorf("Expected inline list_of without target code '%s' not found in:\n%s", expected, result)
		}
	}
	expectedInlineNumeric := []string{
		"int numbers[1000]; // Concatenated list",
		"// Add single element from list_of!(99)",
		"if (numbers_len < 1000) {",
		"numbers[numbers_len] = 99;",
		"numbers_len++;",
	}

	for _, expected := range expectedInlineNumeric {
		if !strings.Contains(result, expected) {
			t.Errorf("Expected inline numeric list_of code '%s' not found in:\n%s", expected, result)
		}
	}
	malformedPatterns := []string{
		"list*of",
		"list_of!(current_line)_len",
		"list_of!(current_line)[",
	}

	for _, pattern := range malformedPatterns {
		if strings.Contains(result, pattern) {
			t.Errorf("Found malformed list_of pattern '%s' in generated code:\n%s", pattern, result)
		}
	}
	globalArrays = make(map[string]string)
}

func TestListOfWithVariableResolution(t *testing.T) {
	program := &lexer.Program{
		Statements: []*lexer.Statement{
			{
				CatList: &lexer.CatListStmt{
					Target: "result",
					Lists:  []string{"existing", "list_of!(this.current_line)"},
				},
			},
			{
				CatList: &lexer.CatListStmt{
					Target: "messages",
					Lists:  []string{"old_messages", "list_of!(\"hello world\")"},
				},
			},
		},
	}

	globalArrays = map[string]string{
		"existing":     "string",
		"old_messages": "string",
	}
	currentModule = "test_module"
	result := RenderC(program, "")
	expectedThisRef := []string{
		"// Add single element from list_of!(this->current_line)",
		"strcpy(result[result_len], this->current_line);",
	}
	for _, expected := range expectedThisRef {
		if !strings.Contains(result, expected) {
			t.Errorf("Expected this. reference handling '%s' not found in:\n%s", expected, result)
		}
	}
	expectedQuotedString := []string{
		"// Add single element from list_of!(\"hello world\")",
		"strcpy(messages[messages_len], \"hello world\");",
	}
	for _, expected := range expectedQuotedString {
		if !strings.Contains(result, expected) {
			t.Errorf("Expected quoted string handling '%s' not found in:\n%s", expected, result)
		}
	}
	globalArrays = make(map[string]string)
	currentModule = ""
}

func TestRecursiveMethodCall(t *testing.T) {
	program := &lexer.Program{
		Statements: []*lexer.Statement{
			{
				ClassDecl: &lexer.ClassDeclStmt{
					Name: "FactorialCalculator",
					Methods: []*lexer.MethodDeclStmt{
						{
							Name: "factorial_recursive",
							Parameters: []*lexer.MethodParameter{
								{Name: "n", Type: "int"},
							},
							ReturnType: "int",
							Body: []*lexer.Statement{
								{
									If: &lexer.IfStmt{
										Condition: "n <= 1",
										Body: []*lexer.Statement{
											{
												Return: &lexer.ReturnStmt{
													Value: "1",
												},
											},
										},
									},
								},
								{
									Return: &lexer.ReturnStmt{
										Value: "n * this.factorial_recursive(n - 1)",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	cCode := RenderC(program, "")
	expected := "FactorialCalculator_factorial_recursive(this, n - 1)"

	if !strings.Contains(cCode, expected) {
		t.Errorf("Expected C code to contain recursive call '%s', but it didn't. Got:\n%s", expected, cCode)
	}
}

func TestPutStatement(t *testing.T) {
	program := &lexer.Program{
		Statements: []*lexer.Statement{
			{
				Put: &lexer.PutStmt{
					Put: "Hello world",
				},
			},
			{
				Put: &lexer.PutStmt{
					Format:    "Value: %d",
					Variables: []string{"x"},
				},
			},
			{
				Put: &lexer.PutStmt{
					Format:    "Name: %s, Age: %d",
					Variables: []string{"name", "age"},
				},
			},
		},
	}

	code := RenderC(program, "./testdata")

	expected1 := `printf("Hello world");`
	if !strings.Contains(code, expected1) {
		t.Errorf("Expected simple put statement to be '%s', but got:\n%s", expected1, code)
	}

	expected2 := `printf("Value: %d", x);`
	if !strings.Contains(code, expected2) {
		t.Errorf("Expected formatted put statement to be '%s', but got:\n%s", expected2, code)
	}

	expected3 := `printf("Name: %s, Age: %d", name, age);`
	if !strings.Contains(code, expected3) {
		t.Errorf("Expected multi-variable put statement to be '%s', but got:\n%s", expected3, code)
	}

	if strings.Contains(code, `printf("Hello world\n");`) {
		t.Errorf("Put statement should not add newlines, but found newline in output")
	}
}

func TestMethodCallInExpression(t *testing.T) {
	program := &lexer.Program{
		Statements: []*lexer.Statement{
			{
				ClassDecl: &lexer.ClassDeclStmt{
					Name: "TestClass",
					Methods: []*lexer.MethodDeclStmt{
						{
							Name: "get_limit",
							Parameters: []*lexer.MethodParameter{
								{Name: "n", Type: "int"},
							},
							ReturnType: "int",
							Body: []*lexer.Statement{
								{
									Return: &lexer.ReturnStmt{
										Value: "n * 2",
									},
								},
							},
						},
						{
							Name: "bar",
							Parameters: []*lexer.MethodParameter{
								{Name: "count", Type: "int"},
							},
							ReturnType: "void",
							Body: []*lexer.Statement{
								{
									For: &lexer.ForStmt{
										Var:   "i",
										Start: "0",
										End:   "this.get_limit(count)",
										Body: []*lexer.Statement{
											{
												Print: &lexer.PrintStmt{
													Variables: []string{"i"},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	cCode := RenderC(program, "")
	expected := "for (int i = 0; i <= (TestClass_get_limit(this, count)); i++)"

	if !strings.Contains(cCode, expected) {
		t.Errorf("Expected C code to contain for loop with method call '%s', but it didn't. Got:\n%s", expected, cCode)
	}
}

// func TestReferenceTypes(t *testing.T) {
// 	program := &lexer.Program{
// 		Statements: []*lexer.Statement{
// 			{
// 				ClassDecl: &lexer.ClassDeclStmt{
// 					Name: "Node",
// 					Constructor: &lexer.ConstructorStmt{
// 						Parameters: []*lexer.MethodParameter{
// 							{Name: "value", Type: "int"},
// 						},
// 						Fields: []*lexer.Statement{
// 							{
// 								VarDecl: &lexer.VarDeclStmt{
// 									Name:  "this.value",
// 									Type:  "int",
// 									Value: "value",
// 								},
// 							},
// 							{
// 								VarDecl: &lexer.VarDeclStmt{
// 									Name:  "this.next",
// 									Type:  "Node",
// 									Value:  "0",
// 									IsRef: true,
// 								},
// 							},
// 						},
// 					},
// 					Methods: []*lexer.MethodDeclStmt{
// 						{
// 							Name: "set_next",
// 							Parameters: []*lexer.MethodParameter{
// 								{Name: "n", Type: "Node", IsRef: true},
// 							},
// 							ReturnType: "void",
// 							Body: []*lexer.Statement{
// 								{
// 									VarAssign: &lexer.VarAssignStmt{
// 										Name:  "this.next",
// 										Value: "n",
// 									},
// 								},
// 							},
// 						},
// 						{
// 							Name: "get_next_value",
// 							ReturnType: "int",
// 							Body: []*lexer.Statement{
// 								{
// 									If: &lexer.IfStmt{
// 										Condition: "this.next != 0",
// 										Body: []*lexer.Statement{
// 											{
// 												Return: &lexer.ReturnStmt{
// 													Value: "this.next.value",
// 												},
// 											},
// 										},
// 										Else: &lexer.ElseStmt{
// 											Body: []*lexer.Statement{
// 												{
// 													Return: &lexer.ReturnStmt{
// 														Value: "-1",
// 													},
// 												},
// 											},
// 										},
// 									},
// 								},
// 							},
// 						},
// 					},
// 				},
// 			},
// 		},
// 	}

// 	cCode := RenderC(program, "")

// 	expectedStruct := `typedef struct Node {
//     int value;
//     struct Node *next;
// } Node;`

// 	expectedSetNext := `void Node_set_next(Node *this, Node *n) {
//     this->next = n;
// }`

// 	expectedGetNextValue := `int Node_get_next_value(Node *this) {
//     if (this->next != 0) {
//         return this->next->value;
//     } else {
//         return -1;
//     }
// }`

// 	if !strings.Contains(cCode, expectedStruct) {
// 		t.Error("Expected C code to contain Node struct definition")
// 	}

// 	if !strings.Contains(cCode, expectedSetNext) {
// 		t.Error("Expected C code to contain set_next method implementation")
// 	}

// 	if !strings.Contains(cCode, expectedGetNextValue) {
// 		t.Error("Expected C code to contain get_next_value method implementation")
// 	}
// }

func TestGetInPrintStatement(t *testing.T) {
	program := &lexer.Program{
		Statements: []*lexer.Statement{
			{
				MapDecl: &lexer.MapDeclStmt{
					Name:      "myMap",
					KeyType:   "string",
					ValueType: "int",
					Pairs: []lexer.MapPair{
						{Key: "one", Value: "1"},
						{Key: "two", Value: "2"},
					},
				},
			},
			{
				Print: &lexer.PrintStmt{
					Format: "The value is: %d",
					Variables: []string{
						"get!(myMap, \"one\")",
					},
				},
			},
		},
	}

	cCode := RenderC(program, "")
	if !strings.Contains(cCode, "__get_myMap_value") {
		t.Error("Expected helper function __get_myMap_value not found in generated code")
	}

	expectedPrint := `printf("The value is: %d\n", __get_myMap_value("one"))`
	if !strings.Contains(cCode, expectedPrint) {
		t.Errorf("Expected print statement not found in generated code. Expected to find: %s", expectedPrint)
	}
}

func TestListParameterInFunction(t *testing.T) {
	program := &lexer.Program{
		Statements: []*lexer.Statement{
			{
				TopLevelFuncDecl: &lexer.TopLevelFuncDeclStmt{
					Name: "processNumbers",
					Parameters: []*lexer.MethodParameter{
						{
							Name:   "numbers",
							Type:   "list[int]",
							IsList: true,
						},
					},
					Body: []*lexer.Statement{
						{
							Print: &lexer.PrintStmt{
								Format:    "First number: %d, Length: %d",
								Variables: []string{"numbers[0]", "numbers_len"},
							},
						},
					},
				},
			},
		},
	}

	cCode := RenderC(program, "")

	// Check that the function prototype has both array and length parameters
	expectedPrototype := "void processNumbers(int numbers[], int numbers_len)"
	if !strings.Contains(cCode, expectedPrototype) {
		t.Errorf("Expected function prototype '%s' not found in generated code", expectedPrototype)
	}

	// Check that the function implementation uses the parameters correctly
	expectedPrint := `printf("First number: %d, Length: %d\n", numbers[0], numbers_len)`
	if !strings.Contains(cCode, expectedPrint) {
		t.Errorf("Expected print statement not found in generated code. Expected to find: %s", expectedPrint)
	}
}

func TestFunctionHoisting(t *testing.T) {
	program := &lexer.Program{
		Imports: []*lexer.ImportStmt{},
		Statements: []*lexer.Statement{
			{
				TopLevelFuncDecl: &lexer.TopLevelFuncDeclStmt{
					Name:       "main",
					Parameters: []*lexer.MethodParameter{},
					ReturnType: "void",
					Body: []*lexer.Statement{
						{
							FunctionCall: &lexer.FunctionCallStmt{
								Name: "print",
								Args: []string{"calculate(5)"},
							},
						},
					},
				},
			},
			{
				TopLevelFuncDecl: &lexer.TopLevelFuncDeclStmt{
					Name: "calculate",
					Parameters: []*lexer.MethodParameter{
						{
							Type: "int",
							Name: "x",
						},
					},
					ReturnType: "int",
					Body: []*lexer.Statement{
						{
							Return: &lexer.ReturnStmt{
								Value: "x * 2",
							},
						},
					},
				},
			},
		},
	}

	// Generate the C code
	cCode := RenderC(program, "")

	// Verify the output contains the function prototype before the main function
	prototypeIndex := strings.Index(cCode, "int calculate(int x);")
	mainFuncIndex := strings.Index(cCode, "void main()")
	calculateFuncIndex := strings.Index(cCode, "int calculate(int x) {")

	// The prototype should appear before both function implementations
	if prototypeIndex == -1 {
		t.Error("Function prototype not found in generated code")
	} else if prototypeIndex >= mainFuncIndex || prototypeIndex >= calculateFuncIndex {
		t.Error("Function prototype not emitted before function implementations")
	}

	// Both function implementations should be present
	if mainFuncIndex == -1 {
		t.Error("main function not found in generated code")
	}

	if calculateFuncIndex == -1 {
		t.Error("calculate function not found in generated code")
	}

	// The function call in main should be correct
	if !strings.Contains(cCode, "print(calculate(5))") {
		t.Error("Function call in main is incorrect")
	}

	// Verify the function prototype is correct
	expectedPrototype := "int calculate(int x);"
	if !strings.Contains(cCode, expectedPrototype) {
		t.Errorf("Expected prototype '%s' not found in generated code", expectedPrototype)
	}
}

func TestMethodCallTranslation(t *testing.T) {
	input := `pub int PI = 3
pub class Calculator:
    fn add(int a, int b) -> int:
        return a + b
var calc = new Calculator()
print "%d" | calc.add(2, 3)`

	program, err := lexer.ParseWithIndentation(input)
	if err != nil {
		t.Fatalf("Failed to parse input: %v", err)
	}

	result := RenderC(program, ".")

	expectedClassDecl := "typedef struct Calculator Calculator;"
	if !strings.Contains(result, expectedClassDecl) {
		t.Errorf("Expected class typedef '%s' not found in generated code", expectedClassDecl)
	}
	expectedConstructor := "Calculator* Calculator_new();"
	if !strings.Contains(result, expectedConstructor) {
		t.Errorf("Expected constructor declaration '%s' not found in generated code", expectedConstructor)
	}
	expectedMethodDecl := "int Calculator_add(Calculator* this, int a, int b);"
	if !strings.Contains(result, expectedMethodDecl) {
		t.Errorf("Expected method declaration '%s' not found in generated code", expectedMethodDecl)
	}
	expectedInstantiation := "Calculator* calc = Calculator_new();"
	if !strings.Contains(result, expectedInstantiation) {
		t.Errorf("Expected object instantiation '%s' not found in generated code", expectedInstantiation)
	}
	expectedMethodCall := "Calculator_add(calc, 2, 3)"
	if !strings.Contains(result, expectedMethodCall) {
		t.Errorf("Expected method call '%s' not found in generated code", expectedMethodCall)
	}
	incorrectMethodCall := "Calc_add(calc, 2, 3)"
	if strings.Contains(result, incorrectMethodCall) {
		t.Errorf("Found incorrect method call '%s' in generated code - this indicates a regression", incorrectMethodCall)
	}
	expectedPrintCall := `printf("%d\n", Calculator_add(calc, 2, 3));`
	if !strings.Contains(result, expectedPrintCall) {
		t.Errorf("Expected print statement '%s' not found in generated code", expectedPrintCall)
	}
	expectedGlobalVar := "int PI = 3;"
	if !strings.Contains(result, expectedGlobalVar) {
		t.Errorf("Expected global variable '%s' not found in generated code", expectedGlobalVar)
	}
	if t.Failed() {
		t.Logf("Full generated C code:\n%s", result)
	}
}

func TestRenderCWithListFunctionAssignment(t *testing.T) {
	t.Run("simple int list function assignment", func(t *testing.T) {
		program := &lexer.Program{
			Statements: []*lexer.Statement{
				{
					TopLevelFuncDecl: &lexer.TopLevelFuncDeclStmt{
						Name:       "get_numbers",
						Parameters: []*lexer.MethodParameter{},
						ReturnType: "list[int]",
						Body: []*lexer.Statement{
							{
								ListDecl: &lexer.ListDeclStmt{
									Name:     "result",
									Type:     "int",
									Elements: []string{"1", "2", "3"},
								},
							},
							{
								Return: &lexer.ReturnStmt{
									Value: "result",
								},
							},
						},
					},
				},
				{
					ListDeclFunctionCall: &lexer.ListDeclFunctionCallStmt{
						Type:         "int",
						Name:         "my_list",
						FunctionCall: "get_numbers()",
					},
				},
				{
					Print: &lexer.PrintStmt{
						Format:    "First: %d",
						Variables: []string{"my_list[0]"},
					},
				},
			},
		}

		cCode := RenderC(program, "")
		expectedPrototype := "int get_numbers(int _output_array[], int _max_size);"
		if !strings.Contains(cCode, expectedPrototype) {
			t.Errorf("Expected function prototype '%s' not found in generated code", expectedPrototype)
		}
		expectedSignature := "int get_numbers(int _output_array[], int _max_size) {"
		if !strings.Contains(cCode, expectedSignature) {
			t.Errorf("Expected function signature '%s' not found in generated code", expectedSignature)
		}
		expectedCopy := "for (int _i = 0; _i < result_len && _i < _max_size; _i++) {"
		if !strings.Contains(cCode, expectedCopy) {
			t.Errorf("Expected array copy loop '%s' not found in generated code", expectedCopy)
		}
		expectedReturnLength := "return result_len;"
		if !strings.Contains(cCode, expectedReturnLength) {
			t.Errorf("Expected return length statement '%s' not found in generated code", expectedReturnLength)
		}
		expectedListDecl := "int my_list[1000];"
		if !strings.Contains(cCode, expectedListDecl) {
			t.Errorf("Expected list declaration '%s' not found in generated code", expectedListDecl)
		}
		expectedLengthVar := "int my_list_len;"
		if !strings.Contains(cCode, expectedLengthVar) {
			t.Errorf("Expected length variable '%s' not found in generated code", expectedLengthVar)
		}
		expectedFunctionCall := "my_list_len = get_numbers(my_list, 1000);"
		if !strings.Contains(cCode, expectedFunctionCall) {
			t.Errorf("Expected function call '%s' not found in generated code", expectedFunctionCall)
		}
		if t.Failed() {
			t.Logf("Full generated C code:\n%s", cCode)
		}
	})

	t.Run("string list function assignment", func(t *testing.T) {
		program := &lexer.Program{
			Statements: []*lexer.Statement{
				{
					TopLevelFuncDecl: &lexer.TopLevelFuncDeclStmt{
						Name:       "get_names",
						Parameters: []*lexer.MethodParameter{},
						ReturnType: "list[string]",
						Body: []*lexer.Statement{
							{
								ListDecl: &lexer.ListDeclStmt{
									Name:     "result",
									Type:     "string",
									Elements: []string{"\"Alice\"", "\"Bob\"", "\"Charlie\""},
								},
							},
							{
								Return: &lexer.ReturnStmt{
									Value: "result",
								},
							},
						},
					},
				},
				{
					ListDeclFunctionCall: &lexer.ListDeclFunctionCallStmt{
						Type:         "string",
						Name:         "my_names",
						FunctionCall: "get_names()",
					},
				},
			},
		}

		cCode := RenderC(program, "")

		expectedPrototype := "int get_names(char _output_array[][256], int _max_size);"
		if !strings.Contains(cCode, expectedPrototype) {
			t.Errorf("Expected string function prototype '%s' not found in generated code", expectedPrototype)
		}
		expectedStringCopy := "strcpy(_output_array[_i], result[_i]);"
		if !strings.Contains(cCode, expectedStringCopy) {
			t.Errorf("Expected string copy statement '%s' not found in generated code", expectedStringCopy)
		}
		expectedStringListDecl := "char my_names[1000][256];"
		if !strings.Contains(cCode, expectedStringListDecl) {
			t.Errorf("Expected string list declaration '%s' not found in generated code", expectedStringListDecl)
		}
		if t.Failed() {
			t.Logf("Full generated C code:\n%s", cCode)
		}
	})

	t.Run("function with parameters returning list", func(t *testing.T) {
		program := &lexer.Program{
			Statements: []*lexer.Statement{
				{
					TopLevelFuncDecl: &lexer.TopLevelFuncDeclStmt{
						Name: "create_range",
						Parameters: []*lexer.MethodParameter{
							{Type: "int", Name: "start"},
							{Type: "int", Name: "end"},
						},
						ReturnType: "list[int]",
						Body: []*lexer.Statement{
							{
								ListDecl: &lexer.ListDeclStmt{
									Name:     "result",
									Type:     "int",
									Elements: []string{},
								},
							},
							{
								Return: &lexer.ReturnStmt{
									Value: "result",
								},
							},
						},
					},
				},
				{
					ListDeclFunctionCall: &lexer.ListDeclFunctionCallStmt{
						Type:         "int",
						Name:         "range_list",
						FunctionCall: "create_range(1, 5)",
					},
				},
			},
		}

		cCode := RenderC(program, "")

		expectedPrototype := "int create_range(int _output_array[], int _max_size, int start, int end);"

		if !strings.Contains(cCode, expectedPrototype) {
			t.Errorf("Expected function prototype with params '%s' not found in generated code", expectedPrototype)
		}

		expectedFunctionCall := "range_list_len = create_range(range_list, 1000, 1, 5);"

		if !strings.Contains(cCode, expectedFunctionCall) {
			t.Errorf("Expected function call with params '%s' not found in generated code", expectedFunctionCall)
		}

		if t.Failed() {
			t.Logf("Full generated C code:\n%s", cCode)
		}
	})

	t.Run("regression test - regular function calls still work", func(t *testing.T) {
		program := &lexer.Program{
			Statements: []*lexer.Statement{
				{
					TopLevelFuncDecl: &lexer.TopLevelFuncDeclStmt{
						Name:       "get_number",
						Parameters: []*lexer.MethodParameter{},
						ReturnType: "int",
						Body: []*lexer.Statement{
							{
								Return: &lexer.ReturnStmt{
									Value: "42",
								},
							},
						},
					},
				},
				{
					VarDecl: &lexer.VarDeclStmt{
						Type:  "int",
						Name:  "num",
						Value: "get_number()",
					},
				},
			},
		}

		cCode := RenderC(program, "")

		expectedPrototype := "int get_number();"
		if !strings.Contains(cCode, expectedPrototype) {
			t.Errorf("Expected regular function prototype '%s' not found in generated code", expectedPrototype)
		}

		expectedVarDecl := "int num = get_number();"
		if !strings.Contains(cCode, expectedVarDecl) {
			t.Errorf("Expected regular variable declaration '%s' not found in generated code", expectedVarDecl)
		}

		incorrectPrototype := "int get_number(int _output_array[], int _max_size);"
		if strings.Contains(cCode, incorrectPrototype) {
			t.Errorf("Found incorrect list function prototype '%s' - this indicates a regression", incorrectPrototype)
		}

		if t.Failed() {
			t.Logf("Full generated C code:\n%s", cCode)
		}
	})
}
