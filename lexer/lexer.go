// By Navid M (c)
// Date: 2025
// License: GPL3
//
// Contains the lexer for the scar programming language.

package lexer

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

type ImportStmt struct {
	Module string
}

type ExternalImportStmt struct {
	Header string
}

type ModuleInfo struct {
	Name            string
	FilePath        string
	PublicVars      map[string]*VarDeclStmt
	PublicClasses   map[string]*ClassDeclStmt
	PublicFuncs     map[string]*MethodDeclStmt
	PublicMacros    map[string]*MacroDeclStmt
	ExternalImports []string
}

type Program struct {
	Imports    []*ImportStmt
	Statements []*Statement
}

type Statement struct {
	Import               *ImportStmt
	ExternalImport       *ExternalImportStmt
	Print                *PrintStmt
	Sleep                *SleepStmt
	While                *WhileStmt
	For                  *ForStmt
	ReverseFor           *ReverseForStmt
	VerboseFor           *VerboseForStmt
	Put                  *PutStmt
	If                   *IfStmt
	Break                *BreakStmt
	Continue             *ContinueStmt
	VarDecl              *VarDeclStmt
	VarAssign            *VarAssignStmt
	IndexAssign          *IndexAssignStmt
	ListDecl             *ListDeclStmt
	ClassDecl            *ClassDeclStmt
	StructDecl           *StructDeclStmt
	PubStructDecl        *PubStructDeclStmt
	EnumDecl             *EnumDeclStmt
	MethodCall           *MethodCallStmt
	StaticMethodCall     *StaticMethodCallStmt
	ObjectDecl           *ObjectDeclStmt
	Return               *ReturnStmt
	VarDeclMethodCall    *VarDeclMethodCallStmt
	VarAssignMethodCall  *VarAssignMethodCallStmt
	VarDeclInferred      *VarDeclInferredStmt
	PubVarDecl           *PubVarDeclStmt
	PubClassDecl         *PubClassDeclStmt
	PubEnumDecl          *PubEnumDeclStmt
	PubAllocate          *PubAllocateStmt
	TopLevelFuncDecl     *TopLevelFuncDeclStmt
	FunctionCall         *FunctionCallStmt
	TryCatch             *TryCatchStmt
	Throw                *ThrowStmt
	VarDeclRead          *VarDeclReadStmt
	VarDeclWrite         *VarDeclWriteStmt
	RawCode              *RawCodeStmt
	MapDecl              *MapDeclStmt
	ParallelFor          *ParallelForStmt
	ParallelWhile        *ParallelWhileStmt
	ParallelBlock        *ParallelBlockStmt
	PubTopLevelFuncDecl  *PubTopLevelFuncDeclStmt
	PutMap               *PutMapStmt
	GetMap               *GetMapStmt
	Foreach              *ForeachStmt
	CatString            *CatStringStmt
	CatList              *CatListStmt
	Run                  *RunStmt
	ListDeclFunctionCall *ListDeclFunctionCallStmt
	ListOf               *ListOfStmt
	ListOfDecl           *ListOfDeclStmt
	Allocate             *AllocateStmt
	StackAllocate        *StackAllocateStmt
	Free                 *FreeStmt
	Pass                 *PassStmt
	NewExpr              *NewExprStmt
	Platform             *PlatformStmt
	MacroDecl            *MacroDeclStmt
	PubMacroDecl         *PubMacroDeclStmt
	MacroCall            *MacroCallStmt
}

type ListOfDeclStmt struct {
	Type  string
	Name  string
	Value string
}

type AllocateStmt struct {
	Type string
	Name string
	Size string
}

type StackAllocateStmt struct {
	Type string
	Name string
	Size string
}

type FreeStmt struct {
	Variable string
}

type NewExprStmt struct {
	ClassName string
	Args      []string
}

type PlatformStmt struct {
	Platform string
	Body     []*Statement
}

type MacroDeclStmt struct {
	Name       string
	Parameters []string
	Body       []string
}

type PubMacroDeclStmt struct {
	Name       string
	Parameters []string
	Body       []string
}

type MacroCallStmt struct {
	Name string
	Args []string
}

type CatListStmt struct {
	Target string
	Lists  []string
}

type RunStmt struct {
	FunctionCall string
}

type CatStringStmt struct {
	Target string
	Value  string
}

type ForeachStmt struct {
	VarType    string
	VarName    string
	Collection string
	Body       []*Statement
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

type GetMapStmt struct {
	MapName string
	Key     string
}

type PubTopLevelFuncDeclStmt struct {
	Name       string
	Parameters []*MethodParameter
	ReturnType string
	Body       []*Statement
}

type ReductionClause struct {
	Operation string
	Variable  string
}

type ParallelForStmt struct {
	Var        string
	Start      string
	End        string
	Step       string
	Reductions []*ReductionClause
	Body       []*Statement
}

type ParallelWhileStmt struct {
	Condition string
	Body      []*Statement
}

type ParallelBlockStmt struct {
	Body []*Statement
}

type PubVarDeclStmt struct {
	Type  string
	Name  string
	Value string
}

type EnumDeclStmt struct {
	IsPublic bool
	Name     string
	Values   []string
}

type PubEnumDeclStmt struct {
	Name   string
	Values []string
}

type PubClassDeclStmt struct {
	Name        string
	Constructor *ConstructorStmt
	Methods     []*MethodDeclStmt
}

type PubAllocateStmt struct {
	Type string
	Name string
	Size string
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

type ListDeclFunctionCallStmt struct {
	Type         string
	Name         string
	FunctionCall string
}

type MapDeclStmt struct {
	KeyType   string
	ValueType string
	Name      string
	Pairs     []MapPair
}

type ListOfStmt struct {
	Type  string
	Value string
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

type ReverseForStmt struct {
	Var   string
	Start string
	End   string
	Body  []*Statement
}

type VerboseForStmt struct {
	VarType   string
	VarName   string
	Init      string
	Condition string
	Increment string
	Body      []*Statement
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

type PassStmt struct {
	Pass string
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

type IndexAssignStmt struct {
	ListName string
	Index    string
	Value    string
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

type StructDeclStmt struct {
	Name   string
	Fields []*StructField
}

type PubStructDeclStmt struct {
	Name   string
	Fields []*StructField
}

type StructField struct {
	Type string
	Name string
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
	IsStatic   bool
}

type MethodCallStmt struct {
	Object string
	Method string
	Args   []string
}

type StaticMethodCallStmt struct {
	Class  string
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
var CurrentSourceFile string
var RegisteredMacros = make(map[string]bool)

func isStandardLibraryFile(filePath string) bool {
	if filePath == "" {
		return false
	}
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return false
	}
	dir := filepath.Dir(absPath)
	if filepath.Base(dir) != "lib" {
		return false
	}
	parentDir := filepath.Dir(dir)
	executableName := "scar"
	if runtime.GOOS == "windows" {
		executableName = "scar.exe"
	}
	executablePath := filepath.Join(parentDir, executableName)
	if _, err := os.Stat(executablePath); err == nil {
		return true
	}
	return false
}

func containsReserved(code string) (bool, string) {
	for _, cmd := range reservedC {
		if strings.Contains(code, cmd+"(") {
			return true, cmd
		}
	}
	return false, ""
}

func ParseWithIndentation(input string) (*Program, error) {
	return InnerParseWithIndentation(input, "")
}

func InnerParseWithIndentation(input string, sourceFile string) (*Program, error) {
	CurrentSourceFile = sourceFile

	var (
		lines           = strings.Split(input, "\n")
		statements, err = parseStatements(lines, 0, 0)
	)
	if err != nil {
		return nil, err
	}
	var (
		imports             []*ImportStmt
		nonImportStatements []*Statement
	)

	importSet := make(map[string]bool)
	addImport := func(imp *ImportStmt) {
		if imp == nil {
			return
		}
		if !importSet[imp.Module] {
			imports = append(imports, imp)
			importSet[imp.Module] = true
		}
	}

	isPlatformActive := func(platformName string) bool {
		p := strings.ToLower(strings.TrimSpace(platformName))
		switch p {
		case "windows":
			return runtime.GOOS == "windows"
		case "posix":
			return runtime.GOOS != "windows"
		case "darwin", "linux":
			return runtime.GOOS == p
		default:
			return runtime.GOOS == p
		}
	}

	var collectImports func(stmts []*Statement)
	collectImports = func(stmts []*Statement) {
		for _, s := range stmts {
			if s == nil {
				continue
			}
			if s.Import != nil {
				addImport(s.Import)
			}
			if s.Platform != nil {
				if isPlatformActive(s.Platform.Platform) {
					collectImports(s.Platform.Body)
				}
			}
		}
	}

	for _, stmt := range statements {
		if stmt.Import != nil {
			addImport(stmt.Import)
			importLines := strings.Split(input, "\n")
			for i, line := range importLines {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "import ") && !strings.HasPrefix(trimmed, "external import") {
					bulkImports, err := parseAllImports(importLines, i)
					if err == nil && len(bulkImports) > 1 {
						for _, bi := range bulkImports {
							addImport(bi)
						}
						break
					}
				}
			}
		} else {
			nonImportStatements = append(nonImportStatements, stmt)
		}
	}

	collectImports(nonImportStatements)

	return &Program{Imports: imports, Statements: nonImportStatements}, nil
}

// Converts type casting functions like float(expr) to C-style casts (float)(expr)
func handleTypeCasting(symbolName string) string {
	var (
		typeCasts = []string{"float", "double", "char"}
		result    = symbolName
	)
	for _, typecast := range typeCasts {
		var (
			pattern         = typecast + "("
			originalPattern = pattern
		)
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
				var (
					moduleName = parts[0]
					symbol     = parts[1]
				)
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
	sanitizedModuleName := strings.ReplaceAll(strings.ReplaceAll(moduleName, "/", "_"), "\\", "_")
	return fmt.Sprintf("%s_%s", sanitizedModuleName, originalName)
}

var (
	vdt          = []string{"int", "float", "double", "char", "string", "bool", "map", "cstring", "lstring"}
	numericTypes = map[string]bool{
		"i8": true, "i16": true, "i32": true, "i64": true,
		"u8": true, "u16": true, "u32": true, "u64": true,
		"f32": true, "f64": true,
	}
)

func isValidType(s string) bool {
	if slices.Contains(vdt, s) {
		return true
	}
	if numericTypes[s] {
		return true
	}
	if strings.HasPrefix(s, "list[") && strings.HasSuffix(s, "]") {
		innerType := strings.TrimPrefix(strings.TrimSuffix(s, "]"), "list[")
		return isValidType(innerType)
	}
	if strings.HasPrefix(s, "map[") && strings.HasSuffix(s, "]") {
		mapTypeContent := strings.TrimPrefix(strings.TrimSuffix(s, "]"), "map[")
		return isValidMapType(mapTypeContent)
	}
	if strings.Contains(s, "::") {
		parts := strings.Split(s, "::")
		if len(parts) == 2 {
			namespace := parts[0]
			typeName := parts[1]
			if len(namespace) > 0 && len(typeName) > 0 {
				for _, r := range namespace {
					if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_') {
						return false
					}
				}
				if len(typeName) > 0 && typeName[0] >= 'A' && typeName[0] <= 'Z' {
					for _, r := range typeName {
						if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_') {
							return false
						}
					}
					return true
				}
			}
		}
		return false
	}
	if strings.Contains(s, "_") {
		parts := strings.Split(s, "_")
		if len(parts) >= 2 {
			lastPart := parts[len(parts)-1]
			if len(lastPart) > 0 && lastPart[0] >= 'A' && lastPart[0] <= 'Z' {
				for i := 0; i < len(parts)-1; i++ {
					part := parts[i]
					if len(part) == 0 {
						return false
					}
					for _, r := range part {
						if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_') {
							return false
						}
					}
				}
				for _, r := range lastPart {
					if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_') {
						return false
					}
				}
				return true
			}
		}
	}
	if len(s) > 0 && s[0] >= 'A' && s[0] <= 'Z' {
		for _, r := range s {
			if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_') {
				return false
			}
		}
		return true
	}
	return false
}

func parseComplexType(typeStr string) (string, bool) {
	if !strings.Contains(typeStr, "[") {
		return typeStr, isValidType(typeStr)
	}
	if strings.HasPrefix(typeStr, "list[") {
		if !strings.HasSuffix(typeStr, "]") {
			return "", false
		}
		innerType := extractInnerType(typeStr, "list[", "]")
		if innerType == "" {
			return "", false
		}
		_, valid := parseComplexType(innerType)
		return typeStr, valid
	}
	if strings.HasPrefix(typeStr, "map[") {
		if !strings.HasSuffix(typeStr, "]") {
			return "", false
		}
		mapContent := extractInnerType(typeStr, "map[", "]")
		if mapContent == "" {
			return "", false
		}
		return typeStr, isValidMapType(mapContent)
	}

	return typeStr, isValidType(typeStr)
}

func extractInnerType(typeStr, prefix, suffix string) string {
	if !strings.HasPrefix(typeStr, prefix) || !strings.HasSuffix(typeStr, suffix) {
		return ""
	}

	content := typeStr[len(prefix) : len(typeStr)-len(suffix)]
	return content
}

func isValidMapType(mapContent string) bool {
	colonPos := findMapColonPosition(mapContent)
	if colonPos == -1 {
		return false
	}

	var (
		keyType   = strings.TrimSpace(mapContent[:colonPos])
		valueType = strings.TrimSpace(mapContent[colonPos+1:])
	)

	if keyType == "" || valueType == "" {
		return false
	}

	_, keyValid := parseComplexType(keyType)
	_, valueValid := parseComplexType(valueType)

	return keyValid && valueValid
}

func findMapColonPosition(mapContent string) int {
	bracketDepth := 0
	for i, char := range mapContent {
		switch char {
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
		case ':':
			if bracketDepth == 0 {
				return i
			}
		}
	}
	return -1
}

func IsOperator(s string) bool {
	return slices.Contains([]string{"+", "-", "*", "/", "%"}, s)
}

var reservedC = []string{
	"printf", "fprintf", "sprintf", "snprintf",
	"strcpy", "strncpy", "strcat", "strncat",
	"gets", "scanf", "fscanf", "sscanf",
	"system", "exec", "popen",
	"malloc", "calloc", "realloc", "free",
	"memcpy", "memmove", "memset",
	"fopen", "fclose", "fread", "fwrite",
	"access", "unlink", "mkdir", "rmdir",
	"stat", "chmod", "chown",
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
