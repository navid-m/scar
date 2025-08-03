package renderer

import (
	"scar/lexer"
	"strings"
	"testing"
)

func TestRecursiveClassMethods(t *testing.T) {
	program := &lexer.Program{
		Imports: []*lexer.ImportStmt{},
		Statements: []*lexer.Statement{
			{
				ClassDecl: &lexer.ClassDeclStmt{
					Name: "RecursiveTest",
					Methods: []*lexer.MethodDeclStmt{
						{
							Name: "methodA",
							Parameters: []*lexer.MethodParameter{
								{Name: "n", Type: "int"},
							},
							ReturnType: "void",
							Body: []*lexer.Statement{
								{
									If: &lexer.IfStmt{
										Condition: "n <= 0",
										Body: []*lexer.Statement{
											{
												Return: &lexer.ReturnStmt{Value: ""},
											},
										},
									},
								},
								{
									FunctionCall: &lexer.FunctionCallStmt{
										Name: "print",
										Args: []string{"A" + " + str(n)"},
									},
								},
								{
									MethodCall: &lexer.MethodCallStmt{
										Object: "this",
										Method: "methodB",
										Args:   []string{"n - 1"},
									},
								},
							},
						},
						{
							Name: "methodB",
							Parameters: []*lexer.MethodParameter{
								{Name: "n", Type: "int"},
							},
							ReturnType: "void",
							Body: []*lexer.Statement{
								{
									If: &lexer.IfStmt{
										Condition: "n <= 0",
										Body: []*lexer.Statement{
											{
												Return: &lexer.ReturnStmt{Value: ""},
											},
										},
									},
								},
								{
									FunctionCall: &lexer.FunctionCallStmt{
										Name: "print",
										Args: []string{"B" + " + str(n)"},
									},
								},
								{
									MethodCall: &lexer.MethodCallStmt{
										Object: "this",
										Method: "methodA",
										Args:   []string{"n - 1"},
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

	protoA := "void RecursiveTest_methodA(RecursiveTest* this, int n);"
	protoB := "void RecursiveTest_methodB(RecursiveTest* this, int n);"

	protoAIndex := strings.Index(cCode, protoA)
	protoBIndex := strings.Index(cCode, protoB)
	if protoAIndex == -1 {
		t.Error("Method A prototype not found in generated code")
	}

	if protoBIndex == -1 {
		t.Error("Method B prototype not found in generated code")
	}
	implAIndex := strings.Index(cCode, "void RecursiveTest_methodA(RecursiveTest* this, int n) {")
	implBIndex := strings.Index(cCode, "void RecursiveTest_methodB(RecursiveTest* this, int n) {")

	if implAIndex != -1 && protoAIndex >= implAIndex {
		t.Error("Method A prototype should appear before its implementation")
	}

	if implBIndex != -1 && protoBIndex >= implBIndex {
		t.Error("Method B prototype should appear before its implementation")
	}

	if !strings.Contains(cCode, "RecursiveTest_methodB(this, n - 1)") {
		t.Error("Method call to methodB not found or incorrect")
	}

	if !strings.Contains(cCode, "RecursiveTest_methodA(this, n - 1)") {
		t.Error("Method call to methodA not found or incorrect")
	}
}
