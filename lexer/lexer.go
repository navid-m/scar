// By Navid M (c)
// Date: 2025
// License: GPL3
//
// Contains the lexer for the scar programming language.

package lexer

import (
	"fmt"
	"slices"
	"strings"
)

type ImportStmt struct {
	Module string
}

type ModuleInfo struct {
	Name          string
	FilePath      string
	PublicVars    map[string]*VarDeclStmt
	PublicClasses map[string]*ClassDeclStmt
	PublicFuncs   map[string]*MethodDeclStmt
}

type Program struct {
	Imports    []*ImportStmt
	Statements []*Statement
}

type Statement struct {
	Import              *ImportStmt
	Print               *PrintStmt
	Sleep               *SleepStmt
	While               *WhileStmt
	For                 *ForStmt
	If                  *IfStmt
	Break               *BreakStmt
	VarDecl             *VarDeclStmt
	VarAssign           *VarAssignStmt
	ListDecl            *ListDeclStmt
	ClassDecl           *ClassDeclStmt
	MethodCall          *MethodCallStmt
	ObjectDecl          *ObjectDeclStmt
	Return              *ReturnStmt
	VarDeclMethodCall   *VarDeclMethodCallStmt
	VarAssignMethodCall *VarAssignMethodCallStmt
	VarDeclInferred     *VarDeclInferredStmt
	PubVarDecl          *PubVarDeclStmt
	PubClassDecl        *PubClassDeclStmt
	TopLevelFuncDecl    *TopLevelFuncDeclStmt
	FunctionCall        *FunctionCallStmt
	TryCatch            *TryCatchStmt
	Throw               *ThrowStmt
	VarDeclRead         *VarDeclReadStmt
	VarDeclWrite        *VarDeclWriteStmt
	RawCode             *RawCodeStmt
	MapDecl             *MapDeclStmt
	ParallelFor         *ParallelForStmt
	PubTopLevelFuncDecl *PubTopLevelFuncDeclStmt
}

type PubTopLevelFuncDeclStmt struct {
	Name       string
	Parameters []*MethodParameter
	ReturnType string
	Body       []*Statement
}

type ParallelForStmt struct {
	Var   string
	Start string
	End   string
	Body  []*Statement
}

type PubVarDeclStmt struct {
	Type  string
	Name  string
	Value string
}

type PubClassDeclStmt struct {
	Name        string
	Constructor *ConstructorStmt
	Methods     []*MethodDeclStmt
}

type VarDeclMethodCallStmt struct {
	Type   string
	Name   string
	Object string
	Method string
	Args   []string
}

type VarAssignMethodCallStmt struct {
	Name   string
	Object string
	Method string
	Args   []string
}

type MapDeclStmt struct {
	KeyType   string
	ValueType string
	Name      string
	Pairs     []MapPair
}

type MapPair struct {
	Key   string
	Value string
}

type VarDeclInferredStmt struct {
	Name  string
	Value string
}

type VarDeclWriteStmt struct {
	Content  string
	FilePath string
	Mode     string
}

type ReturnStmt struct {
	Value string
}

type PrintStmt struct {
	Print     string
	Format    string
	Variables []string
}

type SleepStmt struct {
	Duration string
}

type WhileStmt struct {
	Condition string
	Body      []*Statement
}

type RawCodeStmt struct {
	Code string
}

type ForStmt struct {
	Var   string
	Start string
	End   string
	Body  []*Statement
}

type IfStmt struct {
	Condition string
	Body      []*Statement
	ElseIfs   []*ElifStmt
	Else      *ElseStmt
}

type ElifStmt struct {
	Condition string
	Body      []*Statement
}

type ElseStmt struct {
	Body []*Statement
}

type BreakStmt struct {
	Break string
}

type VarDeclStmt struct {
	Type  string
	Name  string
	Value string
}

type VarAssignStmt struct {
	Name  string
	Value string
}

type ListDeclStmt struct {
	Type     string
	Name     string
	Elements []string
}

type ClassDeclStmt struct {
	Name        string
	Constructor *ConstructorStmt
	Methods     []*MethodDeclStmt
}

type ConstructorStmt struct {
	Parameters []*MethodParameter
	Fields     []*Statement
}

type MethodParameter struct {
	Type string
	Name string
}

type MethodDeclStmt struct {
	Name       string
	Parameters []*MethodParameter
	ReturnType string
	Body       []*Statement
}

type MethodCallStmt struct {
	Object string
	Method string
	Args   []string
}

type ObjectDeclStmt struct {
	Type string
	Name string
	Args []string
}

type Expression struct {
	Left     string
	Operator string
	Right    string
}

type IndexAccess struct {
	ListName string
	Index    string
}

type TopLevelFuncDeclStmt struct {
	Name       string
	Parameters []*MethodParameter
	ReturnType string
	Body       []*Statement
}

type FunctionCallStmt struct {
	Name string
	Args []string
}

type TryCatchStmt struct {
	TryBody   []*Statement
	CatchBody []*Statement
}

type ThrowStmt struct {
	Value string
}

type VarDeclReadStmt struct {
	Type     string
	Name     string
	FilePath string
}

var LoadedModules = make(map[string]*ModuleInfo)

func ParseWithIndentation(input string) (*Program, error) {
	lines := strings.Split(input, "\n")
	statements, err := parseStatements(lines, 0, 0)
	if err != nil {
		return nil, err
	}

	var imports []*ImportStmt
	var nonImportStatements []*Statement

	for _, stmt := range statements {
		if stmt.Import != nil {
			imports = append(imports, stmt.Import)
			if strings.Contains(input, "import") {
				importLines := strings.Split(input, "\n")
				for i, line := range importLines {
					trimmed := strings.TrimSpace(line)
					if strings.HasPrefix(trimmed, "import") {
						bulkImports, err := parseAllImports(importLines, i)
						if err == nil && len(bulkImports) > 1 {
							imports = bulkImports
							break
						}
					}
				}
			}
		} else {
			nonImportStatements = append(nonImportStatements, stmt)
		}
	}

	return &Program{Imports: imports, Statements: nonImportStatements}, nil
}

func ResolveSymbol(symbolName string, currentModule string) string {
	if strings.Contains(symbolName, ".") && (strings.Contains(symbolName, " ") || strings.Contains(symbolName, "*") || strings.Contains(symbolName, "+") || strings.Contains(symbolName, "-") || strings.Contains(symbolName, "/")) {
		result := symbolName
		words := strings.FieldsFunc(symbolName, func(r rune) bool {
			return r == ' ' || r == '*' || r == '+' || r == '-' || r == '/' || r == '(' || r == ')' || r == '[' || r == ']' || r == ',' || r == '=' || r == '<' || r == '>' || r == '!'
		})

		for _, word := range words {
			if strings.Contains(word, ".") {
				parts := strings.SplitN(word, ".", 2)
				if len(parts) == 2 {
					moduleName := parts[0]
					symbol := parts[1]

					if module, exists := LoadedModules[moduleName]; exists {
						if _, exists := module.PublicVars[symbol]; exists {
							replacement := fmt.Sprintf("%s_%s", moduleName, symbol)
							result = strings.ReplaceAll(result, word, replacement)
						}
						if _, exists := module.PublicClasses[symbol]; exists {
							replacement := fmt.Sprintf("%s_%s", moduleName, symbol)
							result = strings.ReplaceAll(result, word, replacement)
						}
					}
				}
			}
		}

		return result
	}

	if strings.Contains(symbolName, ".") {
		parts := strings.SplitN(symbolName, ".", 2)
		moduleName := parts[0]
		symbol := parts[1]

		if module, exists := LoadedModules[moduleName]; exists {
			if _, exists := module.PublicVars[symbol]; exists {
				return fmt.Sprintf("%s_%s", moduleName, symbol)
			}
			if _, exists := module.PublicClasses[symbol]; exists {
				return fmt.Sprintf("%s_%s", moduleName, symbol)
			}
		}
	}

	return symbolName
}

func GenerateUniqueSymbol(originalName string, moduleName string) string {
	if moduleName == "" {
		return originalName
	}
	return fmt.Sprintf("%s_%s", moduleName, originalName)
}

var vdt = []string{"int", "float", "double", "char", "string", "bool", "map"}

func isValidType(s string) bool {
	return slices.Contains(vdt, s)
}

func IsOperator(s string) bool {
	operators := []string{"+", "-", "*", "/", "%"}
	return slices.Contains(operators, s)
}

func getIndentation(line string) int {
	indent := 0
	shouldBreak := false
	for _, char := range line {
		switch char {
		case ' ':
			indent++
		case '\t':
			indent += 4
		default:
			shouldBreak = true
		}
		if shouldBreak {
			break
		}
	}
	return indent
}

func findEndOfBlock(lines []string, startLine, blockIndent int) int {
	for i := startLine; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if getIndentation(line) < blockIndent {
			return i
		}
	}
	return len(lines)
}
