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
	Put                 *PutStmt
	If                  *IfStmt
	Break               *BreakStmt
	Continue            *ContinueStmt
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
	PutMap              *PutMapStmt
}

type PutStmt struct {
	Put       string
	Format    string
	Variables []string
}

type PutMapStmt struct {
	MapName string
	Key     string
	Value   string
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

type ContinueStmt struct {
	Continue string
}

type VarDeclStmt struct {
	Type  string
	Name  string
	Value string
	IsRef bool
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
	Type     string
	IsList   bool
	ListType string
	Name     string
	IsRef    bool
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

// Converts type casting functions like float(expr) to C-style casts (float)(expr)
func handleTypeCasting(symbolName string) string {
	typeCasts := []string{"float", "int", "double", "char"}

	result := symbolName
	for _, typecast := range typeCasts {
		pattern := typecast + "("
		originalPattern := pattern
		for {
			startIdx := strings.LastIndex(result, pattern)
			if startIdx == -1 {
				break
			}
			if startIdx > 0 {
				prevChar := result[startIdx-1]
				if (prevChar >= 'a' && prevChar <= 'z') || (prevChar >= 'A' && prevChar <= 'Z') || (prevChar >= '0' && prevChar <= '9') || prevChar == '_' {
					marker := fmt.Sprintf("__TEMP_MARKER_%s_%d__", typecast, startIdx)
					result = result[:startIdx] + marker + result[startIdx+len(pattern):]
					continue
				}
			}

			var (
				openParen  = startIdx + len(typecast)
				parenCount = 1
				closeParen = openParen + 1
			)

			for closeParen < len(result) && parenCount > 0 {
				switch result[closeParen] {
				case '(':
					parenCount++
				case ')':
					parenCount--
				}
				closeParen++
			}

			if parenCount == 0 {
				var (
					before = result[:startIdx]
					after  = result[closeParen:]
					expr   = result[openParen+1 : closeParen-1]
				)
				result = before + "(" + typecast + ")(" + expr + ")" + after
			} else {
				break
			}
		}

		for i := 0; i < len(result); i++ {
			markerPrefix := fmt.Sprintf("__TEMP_MARKER_%s_", typecast)
			if strings.Contains(result, markerPrefix) {
				for {
					markerStart := strings.Index(result, markerPrefix)
					if markerStart == -1 {
						break
					}
					markerEnd := strings.Index(result[markerStart:], "__") + markerStart + 2
					if markerEnd > markerStart+2 {
						result = result[:markerStart] + originalPattern + result[markerEnd:]
					} else {
						break
					}
				}
				break
			}
		}
	}

	return result
}

// Resolves a symbol.
// Handles type casting functions like float(), int(), etc.
//
// Uses regex and careful parsing to find module.symbol patterns
// without destroying the expression structure
func ResolveSymbol(symbolName string, currentModule string) string {
	result := handleTypeCasting(symbolName)
	if strings.Contains(result, ".") {
		for moduleName, module := range LoadedModules {
			for symbolName := range module.PublicVars {
				pattern := moduleName + "." + symbolName
				replacement := fmt.Sprintf("%s_%s", moduleName, symbolName)
				result = strings.ReplaceAll(result, pattern, replacement)
			}
			for symbolName := range module.PublicClasses {
				pattern := moduleName + "." + symbolName
				replacement := fmt.Sprintf("%s_%s", moduleName, symbolName)
				result = strings.ReplaceAll(result, pattern, replacement)
			}
		}

		if !strings.ContainsAny(result, " *+-/()[]<>=!") {
			parts := strings.SplitN(result, ".", 2)
			if len(parts) == 2 {
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
		}
	}

	return result
}

func GenerateUniqueSymbol(originalName string, moduleName string) string {
	if moduleName == "" {
		return originalName
	}
	return fmt.Sprintf("%s_%s", moduleName, originalName)
}

var vdt = []string{"int", "float", "double", "char", "string", "bool", "map"}

func isValidType(s string) bool {
	if slices.Contains(vdt, s) {
		return true
	}
	if strings.HasPrefix(s, "list[") && strings.HasSuffix(s, "]") {
		innerType := strings.TrimPrefix(strings.TrimSuffix(s, "]"), "list[")
		return slices.Contains(vdt, innerType)
	}
	return false
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
