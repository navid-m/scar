// By Navid M (c)
// Date: 2025
// License: GPL3
//
// IL code generator for the scar programming language.

package renderer

import (
	"fmt"
	"os"
	"regexp"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"scar/lexer"
	"scar/logger"
)

var (
	globalClasses     = make(map[string]*ClassInfo)
	globalEnums       = make(map[string]*EnumInfo)
	globalObjects     = make(map[string]*ObjectInfo)
	globalFunctions   = make(map[string]*lexer.TopLevelFuncDeclStmt)
	globalArrays      = make(map[string]string)
	globalVars        = make(map[string]*lexer.PubVarDeclStmt)
	globalAllocations = make(map[string]*lexer.PubAllocateStmt)
	localVars         = make(map[string]string)
	writeCounter      = 0
	currentModule     = ""
	currentClassName  = ""
	currentFunction   *lexer.TopLevelFuncDeclStmt
	useGC             = false
	primitiveTypes    = map[string]string{
		"int":     "int",
		"float":   "float",
		"double":  "double",
		"bool":    "bool",
		"char":    "char",
		"string":  "char*",
		"lstring": "char*",
		"u16":     "uint16_t",
		"u32":     "uint32_t",
		"u64":     "uint64_t",
		"i16":     "int16_t",
		"i32":     "int32_t",
		"i64":     "int64_t",
		"f32":     "float",
		"f64":     "double",
	}
)

func RenderC(program *lexer.Program, baseDir string, gcFlag bool) string {
	useGC = gcFlag
	var b strings.Builder

	for _, importStmt := range program.Imports {
		_, err := lexer.LoadModule(importStmt.Module, baseDir)
		if err != nil {
			fmt.Printf("\033[31mFailed to load module '%s': %v\033[0m\n", importStmt.Module, err)
			os.Exit(1)
		}
	}

	var externalImports []string
	for _, stmt := range program.Statements {
		if stmt.ExternalImport != nil {
			externalImports = append(externalImports, stmt.ExternalImport.Header)
		}
	}

	for _, module := range lexer.LoadedModules {
		externalImports = append(externalImports, module.ExternalImports...)
	}

	for _, stmt := range program.Statements {
		if stmt.ClassDecl != nil {
			collectClassInfo(stmt.ClassDecl)
		}
		if stmt.PubVarDecl != nil {
			globalVars[stmt.PubVarDecl.Name] = stmt.PubVarDecl
		}
		if stmt.PubAllocate != nil {
			globalAllocations[stmt.PubAllocate.Name] = stmt.PubAllocate
		}
		if stmt.PubClassDecl != nil {
			classDecl := &lexer.ClassDeclStmt{
				Name:        stmt.PubClassDecl.Name,
				Constructor: stmt.PubClassDecl.Constructor,
				Methods:     stmt.PubClassDecl.Methods,
			}
			collectClassInfo(classDecl)
		}
		if stmt.ObjectDecl != nil {
			objectInfo := &ObjectInfo{
				Name: stmt.ObjectDecl.Name,
				Type: stmt.ObjectDecl.Type,
			}
			globalObjects[stmt.ObjectDecl.Name] = objectInfo
		}
		if stmt.TopLevelFuncDecl != nil {
			globalFunctions[stmt.TopLevelFuncDecl.Name] = stmt.TopLevelFuncDecl
		}
		if stmt.PubTopLevelFuncDecl != nil {
			topLevelFunc := &lexer.TopLevelFuncDeclStmt{
				Name:       stmt.PubTopLevelFuncDecl.Name,
				Parameters: stmt.PubTopLevelFuncDecl.Parameters,
				ReturnType: stmt.PubTopLevelFuncDecl.ReturnType,
				Body:       stmt.PubTopLevelFuncDecl.Body,
			}
			globalFunctions[stmt.PubTopLevelFuncDecl.Name] = topLevelFunc
		}
		if stmt.EnumDecl != nil {
			enumInfo := &EnumInfo{
				Name:   stmt.EnumDecl.Name,
				Values: stmt.EnumDecl.Values,
			}
			globalEnums[stmt.EnumDecl.Name] = enumInfo
		}
		if stmt.PubEnumDecl != nil {
			enumInfo := &EnumInfo{
				Name:   stmt.PubEnumDecl.Name,
				Values: stmt.PubEnumDecl.Values,
			}
			globalEnums[stmt.PubEnumDecl.Name] = enumInfo
		}
	}

	for _, module := range lexer.LoadedModules {
		for _, classDecl := range module.PublicClasses {
			collectClassInfoWithModule(classDecl, module.Name)
		}
	}

	for _, enumInfo := range globalEnums {
		b.WriteString("typedef enum {\n")
		for i, value := range enumInfo.Values {
			b.WriteString(fmt.Sprintf("    %s_%s", enumInfo.Name, value))
			if i < len(enumInfo.Values)-1 {
				b.WriteString(",\n")
			}
		}
		b.WriteString(fmt.Sprintf("\n} %s;\n\n", enumInfo.Name))
	}

	b.WriteString(`#include <stdio.h>
#include <string.h>
#include <unistd.h>
#include <omp.h>
#include <stdlib.h>
#include <stdbool.h>
#include <stdint.h>

#ifdef _WIN32
#include <windows.h>
#endif
`)

	if useGC {
		b.WriteString(`#include <gc.h>
`)
	}
	for _, header := range externalImports {
		b.WriteString(fmt.Sprintf("#include <%s>\n", header))
	}

	b.WriteString(`
int _exception = 0;
int __global_argc = 0;
char** __global_argv = NULL;

bool __check_string_key_exists(char keys[][256], int size, char* key) {
    for (int i = 0; i < size; i++) {
        if (strcmp(keys[i], key) == 0) {
            return true;
        }
    }
    return false;
}

bool __check_key_exists(int* keys, int size, int key) {
    for (int i = 0; i < size; i++) {
        if (keys[i] == key) {
            return true;
        }
    }
    return false;
}

`)
	for className := range globalClasses {
		fmt.Fprintf(&b, "struct %s;\n", className)
	}
	b.WriteString("\n")
	for className := range globalClasses {
		fmt.Fprintf(&b, "typedef struct %s %s;\n", className, className)
	}
	b.WriteString("\n")
	for className, classInfo := range globalClasses {
		generateStructDefinition(&b, classInfo, className)
		b.WriteString("\n")
	}

	for className := range globalClasses {
		var constructor *lexer.ConstructorStmt
		for _, stmt := range program.Statements {
			if stmt.ClassDecl != nil && stmt.ClassDecl.Name == className {
				constructor = stmt.ClassDecl.Constructor
				break
			}
			if stmt.PubClassDecl != nil && stmt.PubClassDecl.Name == className {
				constructor = stmt.PubClassDecl.Constructor
				break
			}
		}

		if constructor == nil {
			for _, module := range lexer.LoadedModules {
				for originalClassName, classDecl := range module.PublicClasses {
					moduleClassName := lexer.GenerateUniqueSymbol(originalClassName, module.Name)
					if moduleClassName == className {
						constructor = classDecl.Constructor
						break
					}
				}
				if constructor != nil {
					break
				}
			}
		}

		if constructor != nil && len(constructor.Parameters) > 0 {
			fmt.Fprintf(&b, "%s* %s_new(", className, className)
			for i, param := range constructor.Parameters {
				if i > 0 {
					b.WriteString(", ")
				}
				paramType := mapTypeToCType(param.Type)
				if param.Type == "string" {
					paramType = "char*"
				}
				fmt.Fprintf(&b, "%s %s", paramType, param.Name)
			}
			b.WriteString(");\n")
		} else {
			fmt.Fprintf(&b, "%s* %s_new();\n", className, className)
		}

		b.WriteString("\n")
	}
	for _, module := range lexer.LoadedModules {
		for funcName, funcDecl := range module.PublicFuncs {
			topLevelFunc := &lexer.TopLevelFuncDeclStmt{
				Name:       lexer.GenerateUniqueSymbol(funcName, module.Name),
				Parameters: funcDecl.Parameters,
				ReturnType: funcDecl.ReturnType,
				Body:       funcDecl.Body,
			}
			globalFunctions[lexer.GenerateUniqueSymbol(funcName, module.Name)] = topLevelFunc
		}
	}
	for _, funcDecl := range globalFunctions {
		if funcDecl.Name == "main" {
			continue
		}
		prototype := generateFunctionPrototype(funcDecl)
		b.WriteString(fmt.Sprintf("%s;\n", prototype))
	}
	b.WriteString("\n")
	for varName, varDecl := range globalVars {
		var (
			cType = mapTypeToCType(varDecl.Type)
			value = varDecl.Value
		)

		switch varDecl.Type {
		case "string":
			if !strings.HasPrefix(value, "\"") {
				value = fmt.Sprintf("\"%s\"", value)
			}
			fmt.Fprintf(&b, "char %s[256];\n", varName)
			fmt.Fprintf(&b, "void init_%s() { strcpy(%s, %s); }\n", varName, varName, value)
		case "lstring":
			if !strings.HasPrefix(value, "\"") {
				value = fmt.Sprintf("\"%s\"", value)
			}
			fmt.Fprintf(&b, "char %s[10000];\n", varName)
			fmt.Fprintf(&b, "void init_%s() { strcpy(%s, %s); }\n", varName, varName, value)
		default:
			fmt.Fprintf(&b, "%s %s = %s;\n", cType, varName, value)
		}
	}
	b.WriteString("\n")

	for varName, allocDecl := range globalAllocations {
		cType := mapTypeToCType(allocDecl.Type)
		size := allocDecl.Size
		if strings.HasSuffix(cType, "*") {
			fmt.Fprintf(&b, "%s %s;\n", cType, varName)
			if useGC {
				fmt.Fprintf(&b, "void init_%s() { %s = (%s)GC_malloc(%s * sizeof(%s)); }\n", varName, varName, cType, size, strings.TrimSuffix(cType, "*"))
			} else {
				fmt.Fprintf(&b, "void init_%s() { %s = (%s)malloc(%s * sizeof(%s)); }\n", varName, varName, cType, size, strings.TrimSuffix(cType, "*"))
			}
		} else {
			fmt.Fprintf(&b, "%s* %s;\n", cType, varName)
			if useGC {
				fmt.Fprintf(&b, "void init_%s() { %s = (%s*)GC_malloc(%s * sizeof(%s)); }\n", varName, varName, cType, size, cType)
			} else {
				fmt.Fprintf(&b, "void init_%s() { %s = (%s*)malloc(%s * sizeof(%s)); }\n", varName, varName, cType, size, cType)
			}
		}
	}

	b.WriteString("\n")
	for _, module := range lexer.LoadedModules {
		for varName, varDecl := range module.PublicVars {
			cType := mapTypeToCType(varDecl.Type)
			uniqueName := lexer.GenerateUniqueSymbol(varName, module.Name)
			if varDecl.Type == "string" {
				fmt.Fprintf(&b, "extern char %s[256];\n", uniqueName)
			} else if varDecl.Type == "lstring" {
				fmt.Fprintf(&b, "extern char %s[10000];\n", uniqueName)
			} else {
				fmt.Fprintf(&b, "extern %s %s;\n", cType, uniqueName)
			}
		}
	}

	for _, module := range lexer.LoadedModules {
		for varName, varDecl := range module.PublicVars {
			var (
				cType      = mapTypeToCType(varDecl.Type)
				uniqueName = lexer.GenerateUniqueSymbol(varName, module.Name)
				value      = varDecl.Value
			)
			if varDecl.Type == "string" {
				if !strings.HasPrefix(value, "\"") {
					value = fmt.Sprintf("\"%s\"", value)
				}
				fmt.Fprintf(&b, "char %s[256];\n", uniqueName)
				fmt.Fprintf(&b, "void init_%s() { strcpy(%s, %s); }\n", uniqueName, uniqueName, value)
			} else if varDecl.Type == "lstring" {
				if !strings.HasPrefix(value, "\"") {
					value = fmt.Sprintf("\"%s\"", value)
				}
				fmt.Fprintf(&b, "char %s[10000];\n", uniqueName)
				fmt.Fprintf(&b, "void init_%s() { strcpy(%s, %s); }\n", uniqueName, uniqueName, value)
			} else {
				fmt.Fprintf(&b, "%s %s = %s;\n", cType, uniqueName, value)
			}
		}
	}

	for _, stmt := range program.Statements {
		if stmt.ClassDecl != nil {
			generateClassImplementation(&b, stmt.ClassDecl, "", program)
		}
		if stmt.PubClassDecl != nil {
			classDecl := &lexer.ClassDeclStmt{
				Name:        stmt.PubClassDecl.Name,
				Constructor: stmt.PubClassDecl.Constructor,
				Methods:     stmt.PubClassDecl.Methods,
			}
			generateClassImplementation(&b, classDecl, "", program)
		}
	}

	for _, module := range lexer.LoadedModules {
		for _, classDecl := range module.PublicClasses {
			generateClassImplementation(&b, classDecl, module.Name, program)
		}
	}

	for _, funcDecl := range globalFunctions {
		generateTopLevelFunctionImplementation(&b, funcDecl, program)
	}

	b.WriteString("int main(int argc, char** argv) {\n")
	b.WriteString("#ifdef _WIN32\n")
	b.WriteString("    SetConsoleOutputCP(CP_UTF8);\n")
	b.WriteString("#endif\n")
	b.WriteString("    __global_argc = argc;\n")
	b.WriteString("    __global_argv = argv;\n")

	for _, module := range lexer.LoadedModules {
		for varName, varDecl := range module.PublicVars {
			if varDecl.Type == "string" {
				uniqueName := lexer.GenerateUniqueSymbol(varName, module.Name)
				fmt.Fprintf(&b, "    init_%s();\n", uniqueName)
			}
		}
	}
	for varName, varDecl := range globalVars {
		if varDecl.Type == "string" {
			fmt.Fprintf(&b, "    init_%s();\n", varName)
		}
	}
	for varName := range globalAllocations {
		fmt.Fprintf(&b, "    init_%s();\n", varName)
	}

	var mainStatements []*lexer.Statement
	for _, stmt := range program.Statements {
		if stmt.ClassDecl == nil && stmt.PubClassDecl == nil && stmt.PubVarDecl == nil && stmt.PubAllocate == nil && stmt.TopLevelFuncDecl == nil && stmt.PubTopLevelFuncDecl == nil {
			mainStatements = append(mainStatements, stmt)
		}
	}

	renderStatements(&b, mainStatements, "    ", "", program, "")
	b.WriteString("    return 0;\n")
	b.WriteString("}\n")

	for _, stmt := range program.Statements {
		if stmt.PubTopLevelFuncDecl == nil && stmt.ClassDecl == nil && stmt.PubClassDecl == nil && stmt.PubVarDecl == nil && stmt.PubAllocate == nil && stmt.TopLevelFuncDecl == nil {
			mainStatements = append(mainStatements, stmt)
		}
	}
	return b.String()
}

func resolveLenFunctionCalls(expression string) string {
	lenRegex := regexp.MustCompile(`len\(([a-zA-Z_][a-zA-Z0-9_]*)\)`)
	result := lenRegex.ReplaceAllStringFunc(expression, func(match string) string {
		arrayName := lenRegex.FindStringSubmatch(match)[1]
		if _, exists := globalArrays[arrayName]; exists {
			return arrayName + "_len"
		}
		if currentFunction != nil {
			for _, param := range currentFunction.Parameters {
				if param.Name == arrayName && (param.IsList || strings.HasPrefix(param.Type, "list[")) {
					return arrayName + "_len"
				}
			}
		}

		return match
	})
	return result
}

func collectClassInfo(classDecl *lexer.ClassDeclStmt) {
	collectClassInfoWithModule(classDecl, "")
}

func collectClassInfoWithModule(classDecl *lexer.ClassDeclStmt, moduleName string) {
	className := classDecl.Name
	if moduleName != "" {
		className = lexer.GenerateUniqueSymbol(classDecl.Name, moduleName)
	}

	classInfo := &ClassInfo{
		Name:    className,
		Fields:  []FieldInfo{},
		Methods: []MethodInfo{},
	}

	if classDecl.Constructor != nil {
		fieldMap := make(map[string]bool)
		for _, param := range classDecl.Constructor.Parameters {
			if _, exists := fieldMap[param.Name]; !exists {
				fieldType := param.Type
				if after, ok := strings.CutPrefix(fieldType, "ref "); ok {
					fieldType = after
				}
				fieldInfo := FieldInfo{
					Name:  param.Name,
					Type:  mapTypeToCType(fieldType),
					IsRef: param.IsRef,
				}
				classInfo.Fields = append(classInfo.Fields, fieldInfo)
				fieldMap[param.Name] = true
			}
		}
		for _, stmt := range classDecl.Constructor.Fields {
			if stmt.VarDecl != nil {
				fieldName := stmt.VarDecl.Name
				fieldName = strings.TrimPrefix(fieldName, "this.")
				if _, exists := fieldMap[fieldName]; !exists {
					isRef := stmt.VarDecl.IsRef
					fieldType := stmt.VarDecl.Type
					if after, ok := strings.CutPrefix(fieldType, "ref "); ok {
						fieldType = after
					}
					fieldInfo := FieldInfo{
						Name:  fieldName,
						Type:  fieldType,
						IsRef: isRef,
					}
					classInfo.Fields = append(classInfo.Fields, fieldInfo)
					fieldMap[fieldName] = true
				}
			}
			if stmt.VarAssign != nil && strings.HasPrefix(stmt.VarAssign.Name, "this.") {
				fieldName := stmt.VarAssign.Name[5:]
				if _, exists := fieldMap[fieldName]; !exists {
					fieldType := inferTypeFromValue(stmt.VarAssign.Value)
					isRef := strings.HasPrefix(fieldType, "ref ")
					if isRef {
						fieldType = strings.TrimPrefix(fieldType, "ref ")
					}
					logger.Debug("Field %s, Value %s, Inferred Type: %s, IsRef: %v\n", fieldName, stmt.VarAssign.Value, fieldType, isRef)
					fieldInfo := FieldInfo{
						Name:  fieldName,
						Type:  fieldType,
						IsRef: isRef,
					}
					classInfo.Fields = append(classInfo.Fields, fieldInfo)
					fieldMap[fieldName] = true
				}
			}
			if stmt.MapDecl != nil && strings.HasPrefix(stmt.MapDecl.Name, "this.") {
				fieldName := strings.TrimPrefix(stmt.MapDecl.Name, "this.")
				if _, exists := fieldMap[fieldName]; !exists {
					// For map fields, we need to store key/value arrays and size
					keyFieldInfo := FieldInfo{
						Name:  fieldName + "_keys",
						Type:  stmt.MapDecl.KeyType,
						IsRef: false,
					}
					valueFieldInfo := FieldInfo{
						Name:  fieldName + "_values",
						Type:  stmt.MapDecl.ValueType,
						IsRef: false,
					}
					capacityFieldInfo := FieldInfo{
						Name:  fieldName + "_capacity",
						Type:  "int",
						IsRef: false,
					}
					sizeFieldInfo := FieldInfo{
						Name:  fieldName + "_size",
						Type:  "int",
						IsRef: false,
					}
					classInfo.Fields = append(classInfo.Fields, keyFieldInfo, valueFieldInfo, capacityFieldInfo, sizeFieldInfo)
					fieldMap[fieldName] = true
				}
			}
		}
	}

	for _, method := range classDecl.Methods {
		methodInfo := MethodInfo{
			Name:       method.Name,
			Parameters: []string{},
			ReturnType: method.ReturnType,
		}
		for _, param := range method.Parameters {
			methodInfo.Parameters = append(methodInfo.Parameters, param.Name)
		}
		classInfo.Methods = append(classInfo.Methods, methodInfo)
	}

	globalClasses[className] = classInfo
}

func isFunctionCall(value string) bool {
	return strings.Contains(value, "(") && strings.Contains(value, ")")
}

func resolveFunctionCall(value string) string {
	if !isFunctionCall(value) {
		return value
	}
	parenIndex := strings.Index(value, "(")
	if parenIndex == -1 {
		return value
	}
	var (
		funcName         = strings.TrimSpace(value[:parenIndex])
		argsWithParens   = value[parenIndex:]
		resolvedFuncName = lexer.ResolveSymbol(funcName, currentModule)
	)
	if functionReturnsString(resolvedFuncName) {
		tempBufferName := fmt.Sprintf("temp_str_buffer_%d", len(value)*31%1000) // Simple hash for uniqueness

		args := argsWithParens[1 : len(argsWithParens)-1] // Remove parentheses
		if strings.TrimSpace(args) == "" {
			return fmt.Sprintf("({ char %s[256]; %s(%s); %s; })", tempBufferName, resolvedFuncName, tempBufferName, tempBufferName)
		} else {
			return fmt.Sprintf("({ char %s[256]; %s(%s, %s); %s; })", tempBufferName, resolvedFuncName, tempBufferName, args, tempBufferName)
		}
	}

	return resolvedFuncName + argsWithParens
}

func processMethodArguments(args string) string {
	if strings.TrimSpace(args) == "" {
		return args
	}
	var processedArgs []string
	var currentArg strings.Builder
	parenDepth := 0
	inQuotes := false

	for i, char := range args {
		switch char {
		case '"':
			if i == 0 || args[i-1] != '\\' {
				inQuotes = !inQuotes
			}
			currentArg.WriteRune(char)
		case '(':
			if !inQuotes {
				parenDepth++
			}
			currentArg.WriteRune(char)
		case ')':
			if !inQuotes {
				parenDepth--
			}
			currentArg.WriteRune(char)
		case ',':
			if parenDepth == 0 && !inQuotes {
				arg := strings.TrimSpace(currentArg.String())
				processedArgs = append(processedArgs, processStringFunctionArg(arg))
				currentArg.Reset()
			} else {
				currentArg.WriteRune(char)
			}
		default:
			currentArg.WriteRune(char)
		}
	}
	if currentArg.Len() > 0 {
		arg := strings.TrimSpace(currentArg.String())
		processedArgs = append(processedArgs, processStringFunctionArg(arg))
	}

	return strings.Join(processedArgs, ", ")
}

func convertPropertyAccess(expr string) string {
	logger.Debug("convertPropertyAccess called with: '%s'\n", expr)
	if strings.Contains(expr, "__get_") && strings.Contains(expr, "_value") {
		logger.Debug("convertPropertyAccess: DETECTED GET EXPRESSION: '%s'\n", expr)
	}
	if strings.Contains(expr, "- >") {
		logger.Debug("MANGLED ARROW DETECTED! Stack trace:\n")
		debug.PrintStack()
	}
	bitwiseOps := map[string]string{
		"b_or":     "|",
		"b_and":    "&",
		"b_xor":    "^",
		"b_lshift": "<<",
		"b_rshift": ">>",
	}
	result := expr
	for bitwiseOp, cOp := range bitwiseOps {
		oldResult := result
		result = strings.ReplaceAll(result, " "+bitwiseOp+" ", " "+cOp+" ")
		result = strings.ReplaceAll(result, " "+bitwiseOp+"(", " "+cOp+"(")
		result = strings.ReplaceAll(result, ")"+bitwiseOp+" ", ")"+cOp+" ")
		result = strings.ReplaceAll(result, ")"+bitwiseOp+"(", ")"+cOp+"(")
		pattern := regexp.MustCompile(`(\w)\s+` + regexp.QuoteMeta(bitwiseOp) + `\s+`)
		result = pattern.ReplaceAllString(result, "$1 "+cOp+" ")

		if result != oldResult {
			logger.Debug("convertPropertyAccess bitwise conversion %s -> %s: '%s' became '%s'\n", bitwiseOp, cOp, oldResult, result)
		}
	}
	if result != expr {
		return result
	}

	if isNumericLiteral(expr) {
		logger.Debug("convertPropertyAccess - expression is a numeric literal, skipping conversion\n")
		return expr
	}

	if strings.Contains(expr, ".") && !strings.Contains(expr, "(") {
		logger.Debug("convertPropertyAccess - passed dot and paren checks\n")
		dotIndex := strings.Index(expr, ".")
		logger.Debug("convertPropertyAccess - dotIndex: %d\n", dotIndex)
		if dotIndex > 0 {
			logger.Debug("convertPropertyAccess - passed dotIndex > 0 check\n")
			objectName := expr[:dotIndex]
			logger.Debug("convertPropertyAccess - objectName: '%s'\n", objectName)

			if isNumericLiteral(objectName) {
				logger.Debug("convertPropertyAccess - objectName is numeric, checking if full number literal\n")
				numberEnd := dotIndex + 1
				for numberEnd < len(expr) && unicode.IsDigit(rune(expr[numberEnd])) {
					numberEnd++
				}
				if numberEnd < len(expr) && (expr[numberEnd] == ' ' || expr[numberEnd] == '*' || expr[numberEnd] == '/' || expr[numberEnd] == '+' || expr[numberEnd] == '-') {
					potentialNumber := expr[:numberEnd]
					if isNumericLiteral(potentialNumber) {
						logger.Debug("convertPropertyAccess - detected floating point number '%s' in expression, skipping conversion\n", potentialNumber)
						return expr
					}
				}
			}

			if strings.Contains(objectName, "[") && strings.Contains(objectName, "]") {
				logger.Debug("convertPropertyAccess - detected array element access, keeping dot notation\n")
				return expr
			}

			if !strings.Contains(objectName, " ") && !strings.Contains(objectName, "\"") {
				logger.Debug("convertPropertyAccess - passed objectName checks\n")
				originalExpr := expr
				expr = strings.Replace(expr, ".", "->", 1)
				logger.Debug("convertPropertyAccess converted '%s' to '%s'\n", originalExpr, expr)
			} else {
				logger.Debug("convertPropertyAccess - failed objectName checks\n")
			}
		} else {
			logger.Debug("convertPropertyAccess - failed dotIndex > 0 check\n")
		}
	} else {
		logger.Debug("convertPropertyAccess - failed dot or paren checks\n")
	}
	return expr
}

func processNotKeyword(condition string) string {
	logger.Debug("processNotKeyword called with: '%s'\n", condition)
	condition = strings.TrimSpace(condition)
	if strings.HasPrefix(condition, "not ") {
		remaining := strings.TrimSpace(condition[4:])
		result := "!(" + remaining + ")"
		logger.Debug("processNotKeyword converted '%s' to '%s'\n", condition, result)
		return result
	}
	logger.Debug("processNotKeyword - no 'not' prefix found\n")
	return condition
}

func processStringFunctionArg(arg string) string {
	logger.Debug("processStringFunctionArg called with: '%s'\n", arg)
	arg = convertPropertyAccess(arg)

	if isFunctionCall(arg) {
		parenIndex := strings.Index(arg, "(")
		if parenIndex == -1 {
			return arg
		}
		funcName := strings.TrimSpace(arg[:parenIndex])
		resolvedFuncName := lexer.ResolveSymbol(funcName, currentModule)

		logger.Debug("Function call detected - funcName: '%s', resolvedFuncName: '%s'\n", funcName, resolvedFuncName)

		if functionReturnsString(resolvedFuncName) {
			logger.Debug("Function returns string, transforming...\n")
			argsStr := arg[parenIndex+1 : len(arg)-1]
			tempBufferName := fmt.Sprintf("temp_str_buffer_%d", len(arg)*31%1000)
			if strings.TrimSpace(argsStr) == "" {
				return fmt.Sprintf("({ char %s[256]; %s(%s); %s; })", tempBufferName, resolvedFuncName, tempBufferName, tempBufferName)
			} else {
				return fmt.Sprintf("({ char %s[256]; %s(%s, %s); %s; })", tempBufferName, resolvedFuncName, tempBufferName, argsStr, tempBufferName)
			}
		}
	}

	return arg
}

func processCatExpression(expr string) string {
	if !strings.Contains(expr, "cat!(") {
		return expr
	}
	result := expr
	processed := make(map[string]bool)
	for {
		catIndex := strings.Index(result, "cat!(")
		if catIndex == -1 {
			break
		}
		openParen := catIndex + 4
		parenCount := 1
		i := openParen + 1
		var comma int = -1
		for i < len(result) && parenCount > 0 {
			char := result[i]
			if char == '(' {
				parenCount++
			} else if char == ')' {
				parenCount--
			} else if char == ',' && parenCount == 1 && comma == -1 {
				comma = i
			}
			i++
		}
		if parenCount != 0 || comma == -1 {
			break
		}

		closeParen := i - 1
		fullMatch := result[catIndex : closeParen+1]
		if processed[fullMatch] {
			break
		}
		processed[fullMatch] = true
		arg1 := strings.TrimSpace(result[openParen+1 : comma])
		arg2 := strings.TrimSpace(result[comma+1 : closeParen])
		processedArg1 := processStringFunctionArg(arg1)
		processedArg2 := processStringFunctionArg(arg2)
		replacement := fmt.Sprintf("__CAT_PROCESSED__(%s, %s)", processedArg1, processedArg2)
		result = result[:catIndex] + replacement + result[closeParen+1:]
	}
	result = strings.ReplaceAll(result, "__CAT_PROCESSED__", "cat!")
	return result
}

func inferTypeFromValue(value string) string {
	if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
		if len(value) > 256 {
			return "lstring"
		}
		return "string"
	}
	if value == "NULL" {
		return "ref void"
	}
	if strings.HasPrefix(value, "new ") {
		return "object"
	}
	if strings.Contains(value, ".") && !strings.HasPrefix(value, "new ") {
		return "f32"
	}
	if value == "true" || value == "false" {
		return "bool"
	}
	if strings.HasPrefix(value, "this.") {
		return "ref object"
	}
	return "i32"
}

func generateStructDefinition(b *strings.Builder, classInfo *ClassInfo, structName string) {
	fmt.Fprintf(b, "#define MAX_STRING_LENGTH 256\n")
	fmt.Fprintf(b, "#define MAX_LSTRING_LENGTH 10000\n")
	fmt.Fprintf(b, "#define MAX_MAP_SIZE 100\n")

	hasSelfReference := false
	for _, field := range classInfo.Fields {
		if field.Type == structName && field.IsRef {
			hasSelfReference = true
			break
		}
	}

	if hasSelfReference {
		fmt.Fprintf(b, "struct %s;\n", structName)
	}

	fmt.Fprintf(b, "typedef struct %s {\n", structName)

	for _, field := range classInfo.Fields {
		if strings.HasSuffix(field.Name, "_keys") {
			if field.Type == "string" {
				fmt.Fprintf(b, "    char %s[MAX_MAP_SIZE][MAX_STRING_LENGTH];\n", field.Name)
			} else {
				cType := mapTypeToCType(field.Type)
				fmt.Fprintf(b, "    %s %s[MAX_MAP_SIZE];\n", cType, field.Name)
			}
		} else if strings.HasSuffix(field.Name, "_values") {
			if field.Type == "string" {
				fmt.Fprintf(b, "    char %s[MAX_MAP_SIZE][MAX_STRING_LENGTH];\n", field.Name)
			} else {
				cType := mapTypeToCType(field.Type)
				fmt.Fprintf(b, "    %s %s[MAX_MAP_SIZE];\n", cType, field.Name)
			}
		} else if strings.HasSuffix(field.Name, "_size") || strings.HasSuffix(field.Name, "_capacity") {
			fmt.Fprintf(b, "    int %s;\n", field.Name)
		} else if field.IsRef {
			switch field.Type {
			case "int", "float", "double", "bool", "char":
				fmt.Fprintf(b, "    %s* %s;\n", mapTypeToCType(field.Type), field.Name)
			case "string":
				fmt.Fprintf(b, "    char* %s;\n", field.Name)
			default:
				// For custom types, use the type name directly (without 'struct')
				// since we have a forward declaration with 'typedef struct X X;'
				fmt.Fprintf(b, "    %s* %s;\n", field.Type, field.Name)
			}
		} else if field.Type == "string" {
			fmt.Fprintf(b, "    char %s[MAX_STRING_LENGTH];\n", field.Name)
		} else {
			cType := mapTypeToCType(field.Type)
			fmt.Fprintf(b, "    %s %s;\n", cType, field.Name)
		}
	}
	fmt.Fprintf(b, "} %s;\n", structName)
}

func generateClassImplementation(b *strings.Builder, classDecl *lexer.ClassDeclStmt, moduleName string, program *lexer.Program) {
	className := classDecl.Name
	if moduleName != "" {
		className = lexer.GenerateUniqueSymbol(classDecl.Name, moduleName)
	}

	currentClassName = className
	defer func() { currentClassName = "" }()

	if classDecl.Constructor != nil && len(classDecl.Constructor.Parameters) > 0 {
		fmt.Fprintf(b, "%s* %s_new(", className, className)
		for i, param := range classDecl.Constructor.Parameters {
			if i > 0 {
				b.WriteString(", ")
			}
			paramType := mapTypeToCType(param.Type)
			if param.Type == "string" {
				paramType = "char*"
			}
			fmt.Fprintf(b, "%s %s", paramType, param.Name)
		}
		b.WriteString(") {\n")
	} else {
		fmt.Fprintf(b, "%s* %s_new() {\n", className, className)
	}

	if useGC {
		fmt.Fprintf(b, "    %s* this = (%s*)GC_malloc(sizeof(%s));\n", className, className, className)
	} else {
		fmt.Fprintf(b, "    %s* this = malloc(sizeof(%s));\n", className, className)
	}

	if classInfo, exists := globalClasses[className]; exists {
		for _, field := range classInfo.Fields {
			if strings.HasSuffix(field.Name, "_size") || strings.HasSuffix(field.Name, "_capacity") {
				fmt.Fprintf(b, "    this->%s = 0;\n", field.Name)
			} else if strings.HasPrefix(field.Type, "ref ") {
				fmt.Fprintf(b, "    this->%s = NULL;\n", field.Name)
			} else {
				switch field.Type {
				case "int":
					fmt.Fprintf(b, "    this->%s = 0;\n", field.Name)
				case "float", "double":
					fmt.Fprintf(b, "    this->%s = 0.0;\n", field.Name)
				case "string":
					if !strings.HasSuffix(field.Name, "_keys") && !strings.HasSuffix(field.Name, "_values") {
						fmt.Fprintf(b, "    this->%s[0] = '\\0';\n", field.Name)
					}
				case "bool":
					// Skip initialization for array fields like grid_values
					if !strings.HasSuffix(field.Name, "_values") {
						fmt.Fprintf(b, "    this->%s = 0;\n", field.Name)
					}
				}
			}
		}
	}

	if classDecl.Constructor != nil {
		if classInfo, exists := globalClasses[className]; exists {
			for _, param := range classDecl.Constructor.Parameters {
				for _, field := range classInfo.Fields {
					if field.Name == param.Name {
						if strings.HasPrefix(field.Type, "ref ") {
							fmt.Fprintf(b, "    this->%s = %s;\n", param.Name, param.Name)
						} else if param.Type == "string" {
							fmt.Fprintf(b, "    strcpy(this->%s, %s);\n", param.Name, param.Name)
						} else {
							fmt.Fprintf(b, "    this->%s = %s;\n", param.Name, param.Name)
						}
						break
					}
				}
			}
		}

		for _, stmt := range classDecl.Constructor.Fields {
			switch {
			case stmt.MapDecl != nil && strings.HasPrefix(stmt.MapDecl.Name, "this."):
				var (
					fieldName = strings.TrimPrefix(stmt.MapDecl.Name, "this.")
					mapSize   = len(stmt.MapDecl.Pairs)
				)

				// Set initial capacity - use a reasonable default for empty maps
				initialCapacity := 10
				if mapSize > 0 {
					initialCapacity = mapSize * 2 // Allow for growth
				}

				fmt.Fprintf(b, "    this->%s_size = %d;\n", fieldName, mapSize)
				fmt.Fprintf(b, "    this->%s_capacity = %d;\n", fieldName, initialCapacity)

				// Initialize map pairs
				for i, pair := range stmt.MapDecl.Pairs {
					key := pair.Key
					value := pair.Value

					if stmt.MapDecl.KeyType == "string" {
						if !strings.HasPrefix(key, "\"") && !strings.HasSuffix(key, "\"") {
							key = fmt.Sprintf("\"%s\"", key)
						}
						fmt.Fprintf(b, "    strcpy(this->%s_keys[%d], %s);\n", fieldName, i, key)
					} else {
						fmt.Fprintf(b, "    this->%s_keys[%d] = %s;\n", fieldName, i, key)
					}

					if stmt.MapDecl.ValueType == "string" {
						if !strings.HasPrefix(value, "\"") && !strings.HasSuffix(value, "\"") {
							value = fmt.Sprintf("\"%s\"", value)
						}
						fmt.Fprintf(b, "    strcpy(this->%s_values[%d], %s);\n", fieldName, i, value)
					} else {
						fmt.Fprintf(b, "    this->%s_values[%d] = %s;\n", fieldName, i, value)
					}
				}

			case stmt.VarDecl != nil:
				varName := stmt.VarDecl.Name
				value := stmt.VarDecl.Value

				if varName == "this" {
					continue
				}

				if strings.HasPrefix(varName, "this.") {
					fieldName := varName[5:]
					if stmt.VarDecl.IsRef {
						if value == "0" || value == "NULL" {
							fmt.Fprintf(b, "    this->%s = NULL;\n", fieldName)
						} else {
							fmt.Fprintf(b, "    this->%s = %s;\n", fieldName, value)
						}
					} else if stmt.VarDecl.Type == "string" {
						isStringField := stmt.VarDecl.Type == "string"
						logger.Debug("VarDecl field %s, value %s, isStringField %v\n", fieldName, value, isStringField)
						if isStringField {
							if !strings.HasPrefix(value, "\"") && !strings.HasSuffix(value, "\"") && isValidIdentifier(value) {
								value = fmt.Sprintf("\"%s\"", value)
							}
							fmt.Fprintf(b, "    strcpy(this->%s, %s);\n", fieldName, value)
						}
					} else {
						value = strings.ReplaceAll(value, "this.", "this->")
						fmt.Fprintf(b, "    this->%s = %s;\n", fieldName, value)
					}
				} else {
					renderStatements(b, []*lexer.Statement{stmt}, "    ", className, program, "")
				}

			case stmt.VarAssign != nil:
				fieldName := stmt.VarAssign.Name
				value := stmt.VarAssign.Value

				if fieldName == "this" {
					continue
				}

				fieldName = strings.TrimPrefix(fieldName, "this.")

				isStringField := false
				if classInfo, exists := globalClasses[className]; exists {
					for _, field := range classInfo.Fields {
						if field.Name == fieldName && field.Type == "string" {
							isStringField = true
							break
						}
					}
				}

				logger.Debug("VarAssign field %s, value %s, isStringField %v\n", fieldName, value, isStringField)

				if isStringField {
					if !strings.HasPrefix(value, "\"") && !strings.HasSuffix(value, "\"") && isValidIdentifier(value) {
						value = fmt.Sprintf("\"%s\"", value)
					}
					fmt.Fprintf(b, "    strcpy(this->%s, %s);\n", fieldName, value)
				} else {
					value = strings.ReplaceAll(value, "this.", "this->")
					fmt.Fprintf(b, "    this->%s = %s;\n", fieldName, value)
				}

			case stmt.Print != nil:
				if stmt.Print.Format != "" && len(stmt.Print.Variables) > 0 {
					args := make([]string, len(stmt.Print.Variables))
					for i, v := range stmt.Print.Variables {
						v = strings.ReplaceAll(v, "this.", "this->")
						args[i] = v
					}
					fmt.Fprintf(b, "    printf(\"%s\\n\", %s);\n",
						strings.ReplaceAll(stmt.Print.Format, "\"", "\\\""),
						strings.Join(args, ", "))
				} else if stmt.Print.Print != "" {
					fmt.Fprintf(b, "    printf(\"%s\\n\");\n", stmt.Print.Print)
				}
			default:
				renderStatements(b, []*lexer.Statement{stmt}, "    ", className, program, "")
			}
		}
	}

	b.WriteString("    return this;\n}\n\n")

	if classDecl.Constructor != nil {
		for _, stmt := range classDecl.Constructor.Fields {
			if stmt.MapDecl != nil && strings.HasPrefix(stmt.MapDecl.Name, "this.") {
				fieldName := strings.TrimPrefix(stmt.MapDecl.Name, "this.")
				generateInstanceMapAccessHelper(b, className, fieldName, stmt.MapDecl.KeyType, stmt.MapDecl.ValueType)
				generateInstanceMapPutHelper(b, className, fieldName, stmt.MapDecl.KeyType, stmt.MapDecl.ValueType)
			}
		}
	}

	for _, method := range classDecl.Methods {
		returnType := "void"
		if method.ReturnType != "" && method.ReturnType != "void" {
			returnType = mapTypeToCType(method.ReturnType)
		}
		prototype := generateMethodPrototype(className, method.Name, returnType, method.Parameters, method.IsStatic)
		b.WriteString(prototype)
		b.WriteString(";\n")
	}
	b.WriteString("\n")

	for _, method := range classDecl.Methods {
		returnType := "void"
		if method.ReturnType != "" && method.ReturnType != "void" {
			returnType = mapTypeToCType(method.ReturnType)
		}

		if method.IsStatic {
			fmt.Fprintf(b, "%s %s_%s(", returnType, className, method.Name)
		} else {
			fmt.Fprintf(b, "%s %s_%s(%s* this", returnType, className, method.Name, className)
		}

		for i, param := range method.Parameters {
			paramType := mapTypeToCType(param.Type)
			// For ref parameters, don't add extra * since they should be handled as single pointers
			if param.IsRef {
				// For ref parameters, ensure they are treated as single pointers
				if !strings.HasSuffix(paramType, "*") {
					paramType = paramType + "*"
				}
			} else {
				// For non-ref parameters, apply the normal rules
				if _, isPrimitive := primitiveTypes[param.Type]; !isPrimitive && param.Type != "string" {
					paramType = paramType + "*"
				} else if param.Type == "string" {
					paramType = "char*"
				}
			}

			// Add comma for all parameters (instance methods already have 'this' parameter)
			if method.IsStatic && i == 0 {
				fmt.Fprintf(b, "%s %s", paramType, param.Name)
			} else {
				fmt.Fprintf(b, ", %s %s", paramType, param.Name)
			}
		}

		b.WriteString(") {\n")
		renderStatements(b, method.Body, "    ", className, program, method.ReturnType)
		b.WriteString("}\n\n")
	}
}

func generateInstanceMapAccessHelper(b *strings.Builder, className, fieldName, keyType, valueType string) {
	cKeyType := mapTypeToCType(keyType)
	if keyType == "string" {
		cKeyType = "char*"
	}

	cValueType := mapTypeToCType(valueType)
	if valueType == "string" {
		cValueType = "char*"
	}

	helperName := fmt.Sprintf("%s_get_%s_value", className, fieldName)

	fmt.Fprintf(b, "%s %s(%s* this, %s key) {\n", cValueType, helperName, className, cKeyType)
	fmt.Fprintf(b, "    for (int i = 0; i < this->%s_size; i++) {\n", fieldName)

	if keyType == "string" {
		fmt.Fprintf(b, "        if (strcmp(this->%s_keys[i], key) == 0) {\n", fieldName)
	} else {
		fmt.Fprintf(b, "        if (this->%s_keys[i] == key) {\n", fieldName)
	}

	if valueType == "string" {
		fmt.Fprintf(b, "            return this->%s_values[i];\n", fieldName)
	} else {
		fmt.Fprintf(b, "            return this->%s_values[i];\n", fieldName)
	}
	fmt.Fprintf(b, "        }\n")
	fmt.Fprintf(b, "    }\n")

	// Default return value
	switch valueType {
	case "string":
		fmt.Fprintf(b, "    return \"\";\n")
	case "char":
		fmt.Fprintf(b, "    return '\\0';\n")
	default:
		fmt.Fprintf(b, "    return 0;\n")
	}

	fmt.Fprintf(b, "}\n\n")
}

func generateInstanceMapPutHelper(b *strings.Builder, className, fieldName, keyType, valueType string) {
	cKeyType := mapTypeToCType(keyType)
	if keyType == "string" {
		cKeyType = "char*"
	}

	cValueType := mapTypeToCType(valueType)
	if valueType == "string" {
		cValueType = "char*"
	}

	helperName := fmt.Sprintf("%s_put_%s_value", className, fieldName)

	fmt.Fprintf(b, "void %s(%s* this, %s key, %s value) {\n", helperName, className, cKeyType, cValueType)
	fmt.Fprintf(b, "    int found = 0;\n")
	fmt.Fprintf(b, "    for (int i = 0; i < this->%s_size; i++) {\n", fieldName)

	if keyType == "string" {
		fmt.Fprintf(b, "        if (strcmp(this->%s_keys[i], key) == 0) {\n", fieldName)
	} else {
		fmt.Fprintf(b, "        if (this->%s_keys[i] == key) {\n", fieldName)
	}

	if valueType == "string" {
		fmt.Fprintf(b, "            strcpy(this->%s_values[i], value);\n", fieldName)
	} else {
		fmt.Fprintf(b, "            this->%s_values[i] = value;\n", fieldName)
	}
	fmt.Fprintf(b, "            found = 1;\n")
	fmt.Fprintf(b, "            break;\n")
	fmt.Fprintf(b, "        }\n")
	fmt.Fprintf(b, "    }\n")

	fmt.Fprintf(b, "    if (!found && this->%s_size < MAX_MAP_SIZE) {\n", fieldName)

	if keyType == "string" {
		fmt.Fprintf(b, "        strcpy(this->%s_keys[this->%s_size], key);\n", fieldName, fieldName)
	} else {
		fmt.Fprintf(b, "        this->%s_keys[this->%s_size] = key;\n", fieldName, fieldName)
	}

	if valueType == "string" {
		fmt.Fprintf(b, "        strcpy(this->%s_values[this->%s_size], value);\n", fieldName, fieldName)
	} else {
		fmt.Fprintf(b, "        this->%s_values[this->%s_size] = value;\n", fieldName, fieldName)
	}

	fmt.Fprintf(b, "        this->%s_size++;\n", fieldName)
	fmt.Fprintf(b, "    }\n")
	fmt.Fprintf(b, "}\n\n")
}

func functionReturnsString(funcName string) bool {
	if funcDecl, exists := globalFunctions[funcName]; exists {
		return funcDecl.ReturnType == "string"
	}
	for _, module := range lexer.LoadedModules {
		if funcDecl, exists := module.PublicFuncs[funcName]; exists {
			return funcDecl.ReturnType == "string"
		}
	}
	return false
}

func functionReturnsList(funcName string) (bool, string) {
	var returnType string
	if funcDecl, exists := globalFunctions[funcName]; exists {
		returnType = funcDecl.ReturnType
	} else {
		for _, module := range lexer.LoadedModules {
			if funcDecl, exists := module.PublicFuncs[funcName]; exists {
				returnType = funcDecl.ReturnType
				break
			}
		}
	}

	if strings.HasPrefix(returnType, "list[") && strings.HasSuffix(returnType, "]") {
		innerType := strings.TrimPrefix(strings.TrimSuffix(returnType, "]"), "list[")
		return true, innerType
	}
	return false, ""
}

func parseFunctionCall(funcCall string) (string, []string) {
	parenIndex := strings.Index(funcCall, "(")
	if parenIndex == -1 {
		return funcCall, []string{}
	}

	funcName := strings.TrimSpace(funcCall[:parenIndex])
	argsStr := funcCall[parenIndex+1 : len(funcCall)-1]

	var args []string
	if strings.TrimSpace(argsStr) != "" {
		args = strings.Split(argsStr, ",")
		for i := range args {
			args[i] = strings.TrimSpace(args[i])
		}
	}

	return funcName, args
}

func renderStatements(b *strings.Builder, stmts []*lexer.Statement, indent string, className string, program *lexer.Program, currentFunctionReturnType string) {
	if className != "" {
		currentClassName = className
	}

	for _, stmt := range stmts {
		switch {
		case stmt.Platform != nil:
			plat := strings.TrimSpace(stmt.Platform.Platform)
			var open, close string
			switch plat {
			case "Windows":
				open = "#ifdef _WIN32\n"
				close = "#endif\n"
			case "Posix":
				open = "#ifndef _WIN32\n"
				close = "#endif\n"
			default:
				open = fmt.Sprintf("#ifdef %s\n", plat)
				close = "#endif\n"
			}
			fmt.Fprintf(b, "%s%s", indent, open)
			renderStatements(b, stmt.Platform.Body, indent, className, program, currentFunctionReturnType)
			fmt.Fprintf(b, "%s%s", indent, close)
		case stmt.Put != nil:
			if stmt.Put.Format != "" && len(stmt.Put.Variables) > 0 {
				var (
					variables = reconstructMethodCalls(stmt.Put.Variables)
					args      = make([]string, len(variables))
				)
				for i, v := range variables {
					if isMethodCall(v) {
						args[i] = convertMethodCallToC(v)
					} else {
						resolvedVar := lexer.ResolveSymbol(v, currentModule)
						resolvedVar = convertThisReferencesGranular(resolvedVar)
						resolvedVar = resolveLenFunctionCalls(resolvedVar)
						args[i] = resolvedVar
					}
				}
				argsStr := strings.Join(args, ", ")
				escapedFormat := strings.ReplaceAll(stmt.Put.Format, "\"", "\\\"")
				fmt.Fprintf(b, "%sprintf(\"%s\", %s);\n", indent, escapedFormat, argsStr)
			} else if stmt.Put.Put != "" {
				fmt.Fprintf(b, "%sprintf(\"%s\");\n", indent, stmt.Put.Put)
			}
		case stmt.ListDeclFunctionCall != nil:
			fullListType := stmt.ListDeclFunctionCall.Type
			listName := stmt.ListDeclFunctionCall.Name
			functionCall := stmt.ListDeclFunctionCall.FunctionCall
			resolvedCall := strings.ReplaceAll(functionCall, "::", "_")

			// Extract the inner type from list[inner_type]
			var innerType string
			if strings.HasPrefix(fullListType, "list[") && strings.HasSuffix(fullListType, "]") {
				innerType = strings.TrimPrefix(strings.TrimSuffix(fullListType, "]"), "list[")
			} else {
				innerType = fullListType
			}

			// Extract function name and existing arguments
			openParen := strings.Index(resolvedCall, "(")
			closeParen := strings.LastIndex(resolvedCall, ")")
			if openParen == -1 || closeParen == -1 {
				// Invalid function call format
				fmt.Fprintf(b, "%s// Error: Invalid function call format: %s\n", indent, resolvedCall)
				continue
			}

			funcName := resolvedCall[:openParen]
			existingArgs := strings.TrimSpace(resolvedCall[openParen+1 : closeParen])

			// Build new function call with target array and size parameters
			var newCall string
			if existingArgs == "" {
				newCall = fmt.Sprintf("%s(%s, 1000)", funcName, listName)
			} else {
				newCall = fmt.Sprintf("%s(%s, 1000, %s)", funcName, listName, existingArgs)
			}

			if innerType == "string" {
				fmt.Fprintf(b, "%schar %s[1000][256];\n", indent, listName)
				fmt.Fprintf(b, "%sint %s_len;\n", indent, listName)
				fmt.Fprintf(b, "%s%s_len = %s;\n", indent, listName, newCall)
			} else {
				cType := mapTypeToCType(innerType)
				fmt.Fprintf(b, "%s%s %s[1000];\n", indent, cType, listName)
				fmt.Fprintf(b, "%sint %s_len;\n", indent, listName)
				fmt.Fprintf(b, "%s%s_len = %s;\n", indent, listName, newCall)
			}

			globalArrays[listName] = innerType
		case stmt.CatList != nil:
			if stmt.CatList.Target != "" {
				targetVar := lexer.ResolveSymbol(stmt.CatList.Target, currentModule)
				listType := "int"
				firstList := stmt.CatList.Lists[0]
				if strings.HasPrefix(firstList, "list_of!(") && strings.HasSuffix(firstList, ")") {
					strings.TrimSpace(firstList[9 : len(firstList)-1])
					listType = "string"
				} else {
					for listName, arrayType := range globalArrays {
						if listName == firstList {
							listType = arrayType
							break
						}
					}
				}

				firstListType := listType
				if strings.HasPrefix(listType, "list[") && strings.HasSuffix(listType, "]") {
					firstListType = extractListInnerType(listType)
				}

				for i := 1; i < len(stmt.CatList.Lists); i++ {
					otherList := stmt.CatList.Lists[i]
					if strings.HasPrefix(otherList, "list_of!(") && strings.HasSuffix(otherList, ")") {
						continue
					}

					otherListType := "int"
					for listName, arrayType := range globalArrays {
						if listName == otherList {
							otherListType = arrayType
							break
						}
					}

					if strings.HasPrefix(otherListType, "list[") && strings.HasSuffix(otherListType, "]") {
						otherListType = extractListInnerType(otherListType)
					}

					if firstListType != otherListType {
						err := fmt.Errorf("TypeError: Cannot concatenate lists of different types. '%s' is type '%s' but '%s' is type '%s'", firstList, firstListType, otherList, otherListType)
						fmt.Fprintf(b, "%s// %s\n", indent, err.Error())
						fmt.Fprintf(b, "%s#error \"%s\"\n", indent, err.Error())
						continue
					}
				}

				if listType == "string" {
					fmt.Fprintf(b, "%schar %s[1000][256]; // Concatenated list\n", indent, targetVar)
					fmt.Fprintf(b, "%sint %s_len = 0;\n", indent, targetVar)
				} else {
					cType := mapTypeToCType(listType)
					fmt.Fprintf(b, "%s%s %s[1000]; // Concatenated list\n", indent, cType, targetVar)
					fmt.Fprintf(b, "%sint %s_len = 0;\n", indent, targetVar)
				}

				for _, listName := range stmt.CatList.Lists {
					if strings.HasPrefix(listName, "list_of!(") && strings.HasSuffix(listName, ")") {
						value := strings.TrimSpace(listName[9 : len(listName)-1])
						value = lexer.ResolveSymbol(value, currentModule)
						value = convertThisReferencesGranular(value)

						if listType == "string" {
							fmt.Fprintf(b, "%s// Add single element from list_of!(%s)\n", indent, value)
							fmt.Fprintf(b, "%sif (%s_len < 1000) {\n", indent, targetVar)
							if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
								fmt.Fprintf(b, "%s    strcpy(%s[%s_len], %s);\n", indent, targetVar, targetVar, value)
							} else {
								fmt.Fprintf(b, "%s    strcpy(%s[%s_len], %s);\n", indent, targetVar, targetVar, value)
							}
							fmt.Fprintf(b, "%s    %s_len++;\n", indent, targetVar)
							fmt.Fprintf(b, "%s}\n", indent)
						} else {
							fmt.Fprintf(b, "%s// Add single element from list_of!(%s)\n", indent, value)
							fmt.Fprintf(b, "%sif (%s_len < 1000) {\n", indent, targetVar)
							fmt.Fprintf(b, "%s    %s[%s_len] = %s;\n", indent, targetVar, targetVar, value)
							fmt.Fprintf(b, "%s    %s_len++;\n", indent, targetVar)
							fmt.Fprintf(b, "%s}\n", indent)
						}
					} else {
						resolvedListName := lexer.ResolveSymbol(listName, currentModule)

						if listType == "string" {
							fmt.Fprintf(b, "%s// Copy from %s\n", indent, resolvedListName)
							fmt.Fprintf(b, "%sfor (int __i = 0; __i < %s_len && %s_len < 1000; __i++) {\n",
								indent, resolvedListName, targetVar)
							fmt.Fprintf(b, "%s    strcpy(%s[%s_len], %s[__i]);\n",
								indent, targetVar, targetVar, resolvedListName)
							fmt.Fprintf(b, "%s    %s_len++;\n", indent, targetVar)
							fmt.Fprintf(b, "%s}\n", indent)
						} else {
							fmt.Fprintf(b, "%s// Copy from %s\n", indent, resolvedListName)
							fmt.Fprintf(b, "%sfor (int __i = 0; __i < %s_len && %s_len < 1000; __i++) {\n",
								indent, resolvedListName, targetVar)
							fmt.Fprintf(b, "%s    %s[%s_len] = %s[__i];\n",
								indent, targetVar, targetVar, resolvedListName)
							fmt.Fprintf(b, "%s    %s_len++;\n", indent, targetVar)
							fmt.Fprintf(b, "%s}\n", indent)
						}
					}
				}
				globalArrays[stmt.CatList.Target] = listType

			} else {
				if len(stmt.CatList.Lists) < 2 {
					fmt.Fprintf(b, "%s// Error: catlist! requires at least 2 lists\n", indent)
					break
				}

				targetList := lexer.ResolveSymbol(stmt.CatList.Lists[0], currentModule)
				listType := "int"

				for listName, arrayType := range globalArrays {
					if listName == stmt.CatList.Lists[0] {
						listType = arrayType
						break
					}
				}

				firstListType := listType
				if strings.HasPrefix(listType, "list[") && strings.HasSuffix(listType, "]") {
					firstListType = extractListInnerType(listType)
				}

				for i := 1; i < len(stmt.CatList.Lists); i++ {
					otherList := stmt.CatList.Lists[i]
					if strings.HasPrefix(otherList, "list_of!(") && strings.HasSuffix(otherList, ")") {
						continue
					}

					otherListType := "int" // default
					for listName, arrayType := range globalArrays {
						if listName == otherList {
							otherListType = arrayType
							break
						}
					}

					if strings.HasPrefix(otherListType, "list[") && strings.HasSuffix(otherListType, "]") {
						otherListType = extractListInnerType(otherListType)
					}

					if firstListType != otherListType {
						err := fmt.Errorf("TypeError: Cannot concatenate lists of different types. '%s' is type '%s' but '%s' is type '%s'", stmt.CatList.Lists[0], firstListType, otherList, otherListType)
						fmt.Fprintf(b, "%s// %s\n", indent, err.Error())
						fmt.Fprintf(b, "%s#error \"%s\"\n", indent, err.Error())
						continue
					}
				}

				for i := 1; i < len(stmt.CatList.Lists); i++ {
					listName := stmt.CatList.Lists[i]
					if strings.HasPrefix(listName, "list_of!(") && strings.HasSuffix(listName, ")") {
						value := strings.TrimSpace(listName[9 : len(listName)-1])
						value = lexer.ResolveSymbol(value, currentModule)
						value = convertThisReferencesGranular(value)

						if listType == "string" {
							fmt.Fprintf(b, "%s// Add single element from list_of!(%s)\n", indent, value)
							fmt.Fprintf(b, "%sif (%s_len < 1000) {\n", indent, targetList)
							if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
								fmt.Fprintf(b, "%s    strcpy(%s[%s_len], %s);\n", indent, targetList, targetList, value)
							} else {
								fmt.Fprintf(b, "%s    strcpy(%s[%s_len], %s);\n", indent, targetList, targetList, value)
							}
							fmt.Fprintf(b, "%s    %s_len++;\n", indent, targetList)
							fmt.Fprintf(b, "%s}\n", indent)
						} else {
							fmt.Fprintf(b, "%s// Add single element from list_of!(%s)\n", indent, value)
							fmt.Fprintf(b, "%sif (%s_len < 1000) {\n", indent, targetList)
							fmt.Fprintf(b, "%s    %s[%s_len] = %s;\n", indent, targetList, targetList, value)
							fmt.Fprintf(b, "%s    %s_len++;\n", indent, targetList)
							fmt.Fprintf(b, "%s}\n", indent)
						}
					} else {
						sourceList := lexer.ResolveSymbol(listName, currentModule)

						if listType == "string" {
							fmt.Fprintf(b, "%s// Concatenate %s into %s\n", indent, sourceList, targetList)
							fmt.Fprintf(b, "%sfor (int __i = 0; __i < %s_len; __i++) {\n",
								indent, sourceList)
							fmt.Fprintf(b, "%s    if (%s_len < 1000) {\n", indent, targetList)
							fmt.Fprintf(b, "%s        strcpy(%s[%s_len], %s[__i]);\n",
								indent, targetList, targetList, sourceList)
							fmt.Fprintf(b, "%s        %s_len++;\n", indent, targetList)
							fmt.Fprintf(b, "%s    }\n", indent)
							fmt.Fprintf(b, "%s}\n", indent)
						} else {
							fmt.Fprintf(b, "%s// Concatenate %s into %s\n", indent, sourceList, targetList)
							fmt.Fprintf(b, "%sfor (int __i = 0; __i < %s_len; __i++) {\n",
								indent, sourceList)
							fmt.Fprintf(b, "%s    if (%s_len < 1000) {\n", indent, targetList)
							fmt.Fprintf(b, "%s        %s[%s_len] = %s[__i];\n",
								indent, targetList, targetList, sourceList)
							fmt.Fprintf(b, "%s        %s_len++;\n", indent, targetList)
							fmt.Fprintf(b, "%s    }\n", indent)
							fmt.Fprintf(b, "%s}\n", indent)
						}
					}
				}
			}
		case stmt.ListOf != nil:
			value := stmt.ListOf.Value
			if strings.HasPrefix(value, "this.") {
				value = "this->" + value[5:]
			} else {
				value = lexer.ResolveSymbol(value, currentModule)
				value = convertThisReferencesGranular(value)
			}
			tempVar := fmt.Sprintf("_temp_list_%d", len(b.String())%1000)
			fmt.Fprintf(b, "%schar %s[1][256];\n", indent, tempVar)
			fmt.Fprintf(b, "%sint %s_len = 1;\n", indent, tempVar)
			if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
				fmt.Fprintf(b, "%sstrcpy(%s[0], %s);\n", indent, tempVar, value)
			} else {
				fmt.Fprintf(b, "%sstrcpy(%s[0], %s);\n", indent, tempVar, value)
			}

		case stmt.ListOfDecl != nil:
			var (
				listType = stmt.ListOfDecl.Type
				listName = stmt.ListOfDecl.Name
				value    = stmt.ListOfDecl.Value
			)

			if strings.HasPrefix(value, "this.") {
				value = "this->" + value[5:]
			} else {
				value = lexer.ResolveSymbol(value, currentModule)
				value = convertThisReferencesGranular(value)
			}

			if listType == "string" {
				fmt.Fprintf(b, "%schar %s[1000][256];\n", indent, listName)
				fmt.Fprintf(b, "%sint %s_len = 1;\n", indent, listName)
				if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
					fmt.Fprintf(b, "%sstrcpy(%s[0], %s);\n", indent, listName, value)
				} else {
					fmt.Fprintf(b, "%sstrcpy(%s[0], %s);\n", indent, listName, value)
				}
			} else {
				cType := mapTypeToCType(listType)
				fmt.Fprintf(b, "%s%s %s[1000];\n", indent, cType, listName)
				fmt.Fprintf(b, "%sint %s_len = 1;\n", indent, listName)
				fmt.Fprintf(b, "%s%s[0] = %s;\n", indent, listName, value)
			}

			globalArrays[listName] = listType
		case stmt.Print != nil:
			if stmt.Print.Format != "" && len(stmt.Print.Variables) > 0 {
				var (
					variables = reconstructMethodCalls(stmt.Print.Variables)
					args      = make([]string, len(variables))
				)
				for i, v := range variables {
					if strings.Contains(v, "get!") {
						processedVar := processGetExpressions(v, program)
						if isMethodCall(processedVar) {
							args[i] = convertMethodCallToC(processedVar)
						} else {
							args[i] = processedVar
						}
					} else if isMethodCall(v) {
						args[i] = convertMethodCallToC(v)
					} else {
						resolvedVar := lexer.ResolveSymbol(v, currentModule)
						resolvedVar = convertThisReferencesGranular(resolvedVar)
						resolvedVar = resolveLenFunctionCalls(resolvedVar)
						args[i] = resolvedVar
					}
				}
				argsStr := strings.Join(args, ", ")
				escapedFormat := strings.ReplaceAll(stmt.Print.Format, "\"", "\\\"")
				fmt.Fprintf(b, "%sprintf(\"%s\\n\", %s);\n", indent, escapedFormat, argsStr)
			} else if stmt.Print.Print != "" {
				printValue := stmt.Print.Print
				if isMethodCall(printValue) {
					printValue = convertMethodCallToC(printValue)
				}
				if strings.Contains(printValue, "get!") {
					fmt.Fprintf(b, "%sprintf(\"%%s\\n\", %s);\n", indent, printValue)
				} else {
					fmt.Fprintf(b, "%sprintf(\"%s\\n\");\n", indent, printValue)
				}
			}

		case stmt.Sleep != nil:
			fmt.Fprintf(b, "%ssleep(%s);\n", indent, stmt.Sleep.Duration)
		case stmt.Break != nil:
			fmt.Fprintf(b, "%sbreak;\n", indent)
		case stmt.Continue != nil:
			fmt.Fprintf(b, "%scontinue;\n", indent)
		case stmt.Run != nil:
			funcCall := stmt.Run.FunctionCall
			fmt.Fprintf(b, "%s%s;\n", indent, funcCall)

		case stmt.Return != nil:
			if stmt.Return.Value == "" {
				fmt.Fprintf(b, "%sreturn;\n", indent)
			} else {
				value := stmt.Return.Value

				// Handle list return types
				if strings.HasPrefix(currentFunctionReturnType, "list[") && strings.HasSuffix(currentFunctionReturnType, "]") {
					// For list returns, we need to copy the returned array to the output array
					// and return the length
					innerType := strings.TrimPrefix(strings.TrimSuffix(currentFunctionReturnType, "]"), "list[")

					if innerType == "string" {
						// For string arrays, copy each string
						fmt.Fprintf(b, "%sfor (int _i = 0; _i < %s_len && _i < _max_size; _i++) {\n", indent, value)
						fmt.Fprintf(b, "%s    strcpy(_output_array[_i], %s[_i]);\n", indent, value)
						fmt.Fprintf(b, "%s}\n", indent)
						fmt.Fprintf(b, "%sreturn %s_len;\n", indent, value)
					} else {
						// For other types, copy the array
						fmt.Fprintf(b, "%sfor (int _i = 0; _i < %s_len && _i < _max_size; _i++) {\n", indent, value)
						fmt.Fprintf(b, "%s    _output_array[_i] = %s[_i];\n", indent, value)
						fmt.Fprintf(b, "%s}\n", indent)
						fmt.Fprintf(b, "%sreturn %s_len;\n", indent, value)
					}
					break
				}

				// Process get! and has! expressions first (before this. conversion)
				value = processGetExpressions(value, program)
				value = processHasExpressions(value, program)
				value = convertNewToConstructor(value)
				value = convertPropertyAccess(value)

				if isMethodCall(value) {
					value = convertMethodCallToC(value)
				} else if strings.HasPrefix(value, "this.") {
					value = "this->" + value[5:]
				} else {
					value = lexer.ResolveSymbol(value, currentModule)
				}

				// Convert this references after macro processing
				value = convertThisReferencesGranular(value)

				if strings.Contains(value, " | ") {
					parts := strings.Split(value, " | ")
					if strings.HasPrefix(parts[0], "\"") && strings.HasSuffix(parts[0], "\"") {
						format := parts[0][1 : len(parts[0])-1] // Remove quotes
						specifierCount := strings.Count(format, "%")
						if specifierCount == len(parts)-1 {
							args := strings.Join(parts[1:], ", ")
							tempVar := "_temp_ret_" + strconv.Itoa(len(b.String())%1000)
							fmt.Fprintf(b, "%schar %s[256];\n", indent, tempVar)
							fmt.Fprintf(b, "%ssprintf(%s, \"%s\", %s);\n", indent, tempVar, format, args)
							fmt.Fprintf(b, "%sreturn %s;\n", indent, tempVar)
							break
						}
					}
				}

				if currentFunctionReturnType == "string" && className == "" {
					if value == `""` {
						fmt.Fprintf(b, "%sstrcpy(_output_buffer, \"\");\n", indent)
					} else {
						fmt.Fprintf(b, "%sstrcpy(_output_buffer, %s);\n", indent, value)
					}
					fmt.Fprintf(b, "%sreturn;\n", indent)
				} else {
					fmt.Fprintf(b, "%sreturn %s;\n", indent, value)
				}
			}
		case stmt.GetMap != nil:
			mapAccess := renderMapAccess(stmt.GetMap.MapName, stmt.GetMap.Key, program)
			fmt.Fprintf(b, "%s%s;\n", indent, mapAccess)
		case stmt.Throw != nil:
			value := lexer.ResolveSymbol(stmt.Throw.Value, currentModule)
			fmt.Fprintf(b, "%s_exception = %s;\n", indent, value)
			fmt.Fprintf(b, "%sgoto catch_label;\n", indent)
		case stmt.TryCatch != nil:
			fmt.Fprintf(b, "%s{\n", indent)
			fmt.Fprintf(b, "%s    int _prev_exception = _exception;\n", indent)
			fmt.Fprintf(b, "%s    _exception = 0;\n", indent)
			renderStatements(b, stmt.TryCatch.TryBody, indent+"    ", className, program, currentFunctionReturnType)
			fmt.Fprintf(b, "%s    if (_exception != 0) {\n", indent)
			fmt.Fprintf(b, "%scatch_label:\n", indent)
			renderStatements(b, stmt.TryCatch.CatchBody, indent+"    ", className, program, currentFunctionReturnType)
			fmt.Fprintf(b, "%s    }\n", indent)
			fmt.Fprintf(b, "%s    _exception = _prev_exception;\n", indent)
			fmt.Fprintf(b, "%s}\n", indent)
		case stmt.While != nil:
			condition := stmt.While.Condition
			condition = processNotKeyword(condition)
			condition = lexer.ResolveSymbol(condition, currentModule)
			condition = convertThisReferencesGranular(condition)
			fmt.Fprintf(b, "%swhile (%s) {\n", indent, condition)
			renderStatements(b, stmt.While.Body, indent+"    ", className, program, currentFunctionReturnType)
			fmt.Fprintf(b, "%s}\n", indent)
		case stmt.CatString != nil:
			target := lexer.ResolveSymbol(stmt.CatString.Target, currentModule)
			value := stmt.CatString.Value
			if strings.HasPrefix(target, "this.") {
				target = "this->" + target[5:]
			}
			if !strings.HasPrefix(value, "\"") && !strings.HasSuffix(value, "\"") {
				if isValidIdentifier(value) {
					value = lexer.ResolveSymbol(value, currentModule)
					if strings.HasPrefix(value, "this.") {
						value = "this->" + value[5:]
					}
				} else {
					value = fmt.Sprintf("\"%s\"", value)
				}
			}
			fmt.Fprintf(b, "%sstrcat(%s, %s);\n", indent, target, value)
		case stmt.Foreach != nil:
			var (
				collection = stmt.Foreach.Collection
				varType    = mapTypeToCType(stmt.Foreach.VarType)
				varName    = stmt.Foreach.VarName
			)

			var mapName, accessType string
			if strings.HasSuffix(collection, ".keys") {
				mapName = collection[:len(collection)-5]
				accessType = "keys"
			} else if strings.HasSuffix(collection, ".values") {
				mapName = collection[:len(collection)-7]
				accessType = "values"
			} else {
				resolvedStringName := lexer.ResolveSymbol(collection, currentModule)
				fmt.Fprintf(b, "%sfor (int __i = 0; __i < strlen(%s); __i++) {\n", indent, resolvedStringName)
				fmt.Fprintf(b, "%s    %s %s = %s[__i];\n", indent, varType, varName, resolvedStringName)
				renderStatements(b, stmt.Foreach.Body, indent+"    ", className, program, currentFunctionReturnType)
				fmt.Fprintf(b, "%s}\n", indent)
				break
			}
			resolvedMapName := lexer.ResolveSymbol(mapName, currentModule)
			fmt.Fprintf(b, "%sfor (int __i = 0; __i < %s_size; __i++) {\n", indent, resolvedMapName)
			if accessType == "keys" {
				if stmt.Foreach.VarType == "string" {
					fmt.Fprintf(b, "%s    %s* %s = %s_keys[__i];\n", indent, varType, varName, resolvedMapName)
				} else {
					fmt.Fprintf(b, "%s    %s %s = %s_keys[__i];\n", indent, varType, varName, resolvedMapName)
				}
			} else {
				if stmt.Foreach.VarType == "string" {
					fmt.Fprintf(b, "%s    %s* %s = %s_values[__i];\n", indent, varType, varName, resolvedMapName)
				} else {
					fmt.Fprintf(b, "%s    %s %s = %s_values[__i];\n", indent, varType, varName, resolvedMapName)
				}
			}
			renderStatements(b, stmt.Foreach.Body, indent+"    ", className, program, currentFunctionReturnType)

			fmt.Fprintf(b, "%s}\n", indent)
		case stmt.For != nil:
			var (
				varName = stmt.For.Var
				start   = lexer.ResolveSymbol(stmt.For.Start, currentModule)
				end     = stmt.For.End
			)
			end = convertThisReferencesGranular(end)
			end = lexer.ResolveSymbol(end, currentModule)
			end = resolveLenFunctionCalls(end)

			if after, ok := strings.CutPrefix(varName, "int "); ok {
				varName = after
			}
			varName = strings.ReplaceAll(varName, "*", "")
			varName = strings.ReplaceAll(varName, "_", "")

			endCond := end
			if strings.ContainsAny(end, "+-*/><=!&|^%(") {
				endCond = fmt.Sprintf("(%s)", end)
			}

			fmt.Fprintf(b, "%sfor (int %s = %s; %s <= %s; %s++) {\n",
				indent, varName, start, varName, endCond, varName)
			renderStatements(b, stmt.For.Body, indent+"    ", className, program, currentFunctionReturnType)
			fmt.Fprintf(b, "%s}\n", indent)
		case stmt.ReverseFor != nil:
			var (
				varName = stmt.ReverseFor.Var
				start   = lexer.ResolveSymbol(stmt.ReverseFor.Start, currentModule)
				end     = stmt.ReverseFor.End
			)
			start = convertThisReferencesGranular(start)
			start = lexer.ResolveSymbol(start, currentModule)
			start = resolveLenFunctionCalls(start)

			end = convertThisReferencesGranular(end)
			end = lexer.ResolveSymbol(end, currentModule)
			end = resolveLenFunctionCalls(end)

			if after, ok := strings.CutPrefix(varName, "int "); ok {
				varName = after
			}
			varName = strings.ReplaceAll(varName, "*", "")
			varName = strings.ReplaceAll(varName, "_", "")

			startCond := start
			if strings.ContainsAny(start, "+-*/><=!&|^%(") {
				startCond = fmt.Sprintf("(%s)", start)
			}

			endCond := end
			if strings.ContainsAny(end, "+-*/><=!&|^%(") {
				endCond = fmt.Sprintf("(%s)", end)
			}

			fmt.Fprintf(b, "%sfor (int %s = %s; %s >= %s; %s--) {\n",
				indent, varName, startCond, varName, endCond, varName)
			renderStatements(b, stmt.ReverseFor.Body, indent+"    ", className, program, currentFunctionReturnType)
			fmt.Fprintf(b, "%s}\n", indent)
		case stmt.VerboseFor != nil:
			var (
				varType   = mapTypeToCType(stmt.VerboseFor.VarType)
				varName   = stmt.VerboseFor.VarName
				init      = lexer.ResolveSymbol(stmt.VerboseFor.Init, currentModule)
				condition = stmt.VerboseFor.Condition
				increment = stmt.VerboseFor.Increment
			)

			init = convertThisReferencesGranular(init)
			condition = convertThisReferencesGranular(condition)
			condition = lexer.ResolveSymbol(condition, currentModule)
			increment = convertThisReferencesGranular(increment)
			increment = lexer.ResolveSymbol(increment, currentModule)

			varName = strings.ReplaceAll(varName, "*", "")
			varName = strings.ReplaceAll(varName, "_", "")

			fmt.Fprintf(b, "%sfor (%s %s = %s; %s; %s) {\n",
				indent, varType, varName, init, condition, increment)
			renderStatements(b, stmt.VerboseFor.Body, indent+"    ", className, program, currentFunctionReturnType)
			fmt.Fprintf(b, "%s}\n", indent)
		case stmt.If != nil:
			condition := stmt.If.Condition
			condition = processNotKeyword(condition)
			condition = processGetExpressions(condition, program)
			condition = processHasExpressions(condition, program)
			condition = convertPropertyAccess(condition)
			condition = resolveLenFunctionCalls(condition)

			if isMethodCall(condition) {
				condition = convertMethodCallToC(condition)
			} else {
				condition = lexer.ResolveSymbol(condition, currentModule)
			}

			// Convert this references after macro processing
			condition = convertThisReferencesGranular(condition)
			condition = resolveImportedSymbols(condition, program.Imports)
			fmt.Fprintf(b, "%sif (%s) {\n", indent, condition)
			renderStatements(b, stmt.If.Body, indent+"    ", className, program, currentFunctionReturnType)
			fmt.Fprintf(b, "%s}\n", indent)

			for _, elif := range stmt.If.ElseIfs {
				elifCondition := elif.Condition
				elifCondition = processNotKeyword(elifCondition)
				elifCondition = processGetExpressions(elifCondition, program)
				elifCondition = processHasExpressions(elifCondition, program)
				elifCondition = convertPropertyAccess(elifCondition)
				elifCondition = resolveLenFunctionCalls(elifCondition)

				if isMethodCall(elifCondition) {
					elifCondition = convertMethodCallToC(elifCondition)
				} else {
					elifCondition = lexer.ResolveSymbol(elifCondition, currentModule)
				}

				// Convert this references after macro processing
				elifCondition = convertThisReferencesGranular(elifCondition)
				elifCondition = resolveImportedSymbols(elifCondition, program.Imports)
				fmt.Fprintf(b, "%selse if (%s) {\n", indent, elifCondition)
				renderStatements(b, elif.Body, indent+"    ", className, program, currentFunctionReturnType)
				fmt.Fprintf(b, "%s}\n", indent)
			}

			if stmt.If.Else != nil {
				fmt.Fprintf(b, "%selse {\n", indent)
				renderStatements(b, stmt.If.Else.Body, indent+"    ", className, program, currentFunctionReturnType)
				fmt.Fprintf(b, "%s}\n", indent)
			}
		case stmt.VarDecl != nil:
			var (
				varType = stmt.VarDecl.Type
				varName = lexer.ResolveSymbol(stmt.VarDecl.Name, currentModule)
				value   = stmt.VarDecl.Value
			)

			localVars[varName] = varType

			var classNames []string
			for className := range globalClasses {
				classNames = append(classNames, className)
			}
			if _, isClassType := globalClasses[varType]; isClassType {
				objectInfo := &ObjectInfo{
					Name: stmt.VarDecl.Name,
					Type: varType,
				}
				globalObjects[stmt.VarDecl.Name] = objectInfo
				logger.Debug("Tracked object '%s' of type '%s'\n", stmt.VarDecl.Name, varType)
			}

			if stmt.VarDecl.IsRef {
				if strings.HasPrefix(varName, "this.") {
					fieldName := varName[5:]
					if value == "0" || value == "NULL" {
						fmt.Fprintf(b, "%sthis->%s = NULL;\n", indent, fieldName)
					} else {
						value = convertThisReferencesGranular(value)
						value = convertNewToConstructor(value)
						fmt.Fprintf(b, "%sthis->%s = %s;\n", indent, fieldName, value)
					}
				} else {
					innerType := strings.TrimPrefix(varType, "ref ")
					switch innerType {
					case "int", "float", "double", "bool", "char":
						fmt.Fprintf(b, "%s%s* %s = ", indent, mapTypeToCType(innerType), varName)
					case "string":
						fmt.Fprintf(b, "%schar* %s = ", indent, varName)
					default:
						fmt.Fprintf(b, "%s%s* %s = ", indent, innerType, varName)
					}

					if value == "0" || value == "NULL" || value == "nil" {
						fmt.Fprintf(b, "NULL;\n")
					} else {
						value = convertThisReferencesGranular(value)
						value = convertNewToConstructor(value)
						fmt.Fprintf(b, "%s;\n", value)
					}
				}
				// Reference variable declaration is complete, continue to next statement
				continue
			}

			if isMethodCall(value) {
				value = convertMethodCallToC(value)
			} else {
				value = convertThisReferencesGranular(value)
			}

			value = processGetExpressions(value, program)
			value = processHasExpressions(value, program)
			value = convertPropertyAccess(value)
			value = fixFloatCastGranular(value)
			value = resolveImportedSymbols(value, program.Imports)
			value = resolveLenFunctionCalls(value)

			if err := checkTypeCompatibility(varName, varType, value); err != nil {
				fmt.Fprintf(b, "%s// %s\n", indent, err.Error())
				fmt.Fprintf(b, "%s#error \"%s\"\n", indent, err.Error())
				continue
			}

			if strings.HasPrefix(varName, "this.") {
				fieldName := varName[5:]
				if stmt.VarDecl.Type == "string" {
					if isFunctionCall(value) {
						funcName, args := parseFunctionCall(value)
						resolvedFuncName := lexer.ResolveSymbol(funcName, currentModule)
						if functionReturnsString(resolvedFuncName) {
							if len(args) == 0 {
								fmt.Fprintf(b, "%s%s(this->%s);\n", indent, resolvedFuncName, fieldName)
							} else {
								resolvedArgs := make([]string, len(args))
								for i, arg := range args {
									resolvedArgs[i] = lexer.ResolveSymbol(arg, currentModule)
								}
								fmt.Fprintf(b, "%s%s(this->%s, %s);\n", indent, resolvedFuncName, fieldName, strings.Join(resolvedArgs, ", "))
							}
						} else {
							resolvedCall := resolveFunctionCall(value)
							fmt.Fprintf(b, "%sstrcpy(this->%s, %s);\n", indent, fieldName, resolvedCall)
						}
					} else {
						if !strings.HasPrefix(value, "\"") && !strings.HasSuffix(value, "\"") {
							value = fmt.Sprintf("\"%s\"", value)
						}
						fmt.Fprintf(b, "%sstrcpy(this->%s, %s);\n", indent, fieldName, value)
					}
				} else {
					value = strings.ReplaceAll(value, "this.", "this->")
					if isFunctionCall(value) {
						value = resolveFunctionCall(value)
					}
					fmt.Fprintf(b, "%sthis->%s = %s;\n", indent, fieldName, value)
				}
			} else {
				if stmt.VarDecl.Type == "string" {
					fmt.Fprintf(b, "%schar %s[256];\n", indent, varName)
					if value == "" || value == "\"\"" {
						fmt.Fprintf(b, "%sstrcpy(%s, \"\");\n", indent, varName)
					} else if isFunctionCall(value) {
						funcName, args := parseFunctionCall(value)
						resolvedFuncName := lexer.ResolveSymbol(funcName, currentModule)

						if functionReturnsString(resolvedFuncName) {
							if len(args) == 0 {
								fmt.Fprintf(b, "%s%s(%s);\n", indent, resolvedFuncName, varName)
							} else {
								resolvedArgs := make([]string, len(args))
								for i, arg := range args {
									resolvedArgs[i] = lexer.ResolveSymbol(arg, currentModule)
								}
								fmt.Fprintf(b, "%s%s(%s, %s);\n", indent, resolvedFuncName, varName, strings.Join(resolvedArgs, ", "))
							}
						} else {
							resolvedCall := resolveFunctionCall(value)
							fmt.Fprintf(b, "%sstrcpy(%s, %s);\n", indent, varName, resolvedCall)
						}
					} else {
						if !strings.HasPrefix(value, "\"") && !strings.HasSuffix(value, "\"") {
							if _, isLocal := localVars[value]; isLocal {
								fmt.Fprintf(b, "%sstrcpy(%s, %s);\n", indent, varName, value)
							} else if _, isGlobal := globalVars[value]; isGlobal {
								fmt.Fprintf(b, "%sstrcpy(%s, %s);\n", indent, varName, value)
							} else if strings.Contains(value, "[") && strings.Contains(value, "]") {
								fmt.Fprintf(b, "%sstrcpy(%s, %s);\n", indent, varName, value)
							} else {
								value = fmt.Sprintf("\"%s\"", value)
								fmt.Fprintf(b, "%sstrcpy(%s, %s);\n", indent, varName, value)
							}
						} else {
							fmt.Fprintf(b, "%sstrcpy(%s, %s);\n", indent, varName, value)
						}
					}
				} else {
					if isFunctionCall(value) {
						value = resolveFunctionCall(value)
					}
					value = convertNewToConstructor(value)
					cType := mapTypeToCType(varType)
					fmt.Fprintf(b, "%s%s %s = %s;\n", indent, cType, varName, value)
				}
			}
		case stmt.VarAssign != nil:
			var (
				varName = lexer.ResolveSymbol(stmt.VarAssign.Name, currentModule)
				value   = stmt.VarAssign.Value
			)

			value = lexer.ResolveSymbol(value, currentModule)
			value = fixFloatCastGranular(value)
			value = convertThisReferencesGranular(value)
			value = convertNewToConstructor(value) // Convert 'new ClassName(args)' to 'ClassName_new(args)'
			value = convertMethodCallToC(value)    // Convert method calls like 'obj.method()' to 'Class_method(obj)'
			value = convertPropertyAccess(value)   // Convert property access from dot to arrow notation

			var varType string
			if localType, exists := localVars[varName]; exists {
				varType = localType
			} else {
				for _, classInfo := range globalClasses {
					for _, field := range classInfo.Fields {
						if field.Name == varName || ("this->"+field.Name) == varName {
							varType = field.Type
							break
						}
					}
				}
			}
			if varType != "" {
				if err := checkTypeCompatibility(varName, varType, value); err != nil {
					fmt.Fprintf(os.Stderr, "\033[31mCompilation error: %s\033[0m\n", err.Error())
					os.Exit(1)
				}
			}

			if strings.HasPrefix(varName, "this.") {
				varName = "this->" + varName[5:]
			} else if strings.Contains(varName, ".") {
				re := regexp.MustCompile(`([a-zA-Z_][a-zA-Z0-9_]*\[[^\]]+\])\.([a-zA-Z_][a-zA-Z0-9_]*)`)
				if re.MatchString(varName) {
				} else {
					re := regexp.MustCompile(`([a-zA-Z_][a-zA-Z0-9_]*)\.([a-zA-Z_][a-zA-Z0-9_]*)`)
					varName = re.ReplaceAllString(varName, "$1->$2")
				}
			}

			isMapToMapAssignment := false
			var targetFieldName, sourceMapName string
			if strings.HasPrefix(varName, "this->") {
				targetFieldName = varName[6:] // Remove "this->"
				sourceMapName = value
				for _, s := range program.Statements {
					if (s.ClassDecl != nil || s.PubClassDecl != nil) && s.ClassDecl != nil && s.ClassDecl.Constructor != nil {
						for _, field := range s.ClassDecl.Constructor.Fields {
							if field.MapDecl != nil && strings.HasPrefix(field.MapDecl.Name, "this.") {
								declFieldName := strings.TrimPrefix(field.MapDecl.Name, "this.")
								if declFieldName == targetFieldName {
									isMapToMapAssignment = true
									break
								}
							}
						}
					}
					if (s.PubClassDecl != nil) && s.PubClassDecl.Constructor != nil {
						for _, field := range s.PubClassDecl.Constructor.Fields {
							if field.MapDecl != nil && strings.HasPrefix(field.MapDecl.Name, "this.") {
								declFieldName := strings.TrimPrefix(field.MapDecl.Name, "this.")
								if declFieldName == targetFieldName {
									isMapToMapAssignment = true
									break
								}
							}
						}
					}
					if isMapToMapAssignment {
						break
					}
				}
			}

			if isMapToMapAssignment {
				fmt.Fprintf(b, "%s// Copy map from %s to this->%s\n", indent, sourceMapName, targetFieldName)
				fmt.Fprintf(b, "%sthis->%s_size = %s_size;\n", indent, targetFieldName, sourceMapName)
				fmt.Fprintf(b, "%sfor (int i = 0; i < %s_size; i++) {\n", indent, sourceMapName)
				fmt.Fprintf(b, "%s    strcpy(this->%s_keys[i], %s_keys[i]);\n", indent, targetFieldName, sourceMapName)
				fmt.Fprintf(b, "%s    this->%s_values[i] = %s_values[i];\n", indent, targetFieldName, sourceMapName)
				fmt.Fprintf(b, "%s}\n", indent)
			} else if strings.Contains(varName, "[") && strings.Contains(varName, "]") {
				arrayName := varName[:strings.Index(varName, "[")]
				if arrayType, exists := globalArrays[arrayName]; exists && arrayType == "string" {
					if isFunctionCall(value) {
						value = resolveFunctionCall(value)
						fmt.Fprintf(b, "%sstrcpy(%s, %s);\n", indent, varName, value)
					} else {
						if !strings.HasPrefix(value, "\"") && !strings.HasSuffix(value, "\"") {
							value = fmt.Sprintf("\"%s\"", value)
						}
						fmt.Fprintf(b, "%sstrcpy(%s, %s);\n", indent, varName, value)
					}
				} else {
					if isFunctionCall(value) {
						value = resolveFunctionCall(value)
					}
					fmt.Fprintf(b, "%s%s = %s;\n", indent, varName, value)
				}
			} else {
				var varType string

				// First check if it's a local variable
				if localType, exists := localVars[varName]; exists {
					varType = localType
				} else {
					// Fallback to checking class fields
					for _, classInfo := range globalClasses {
						for _, field := range classInfo.Fields {
							if field.Name == varName || ("this->"+field.Name) == varName {
								varType = field.Type
								break
							}
						}
					}
				}

				if varType == "string" {
					value = processCatExpression(value)

					if isFunctionCall(value) {
						value = resolveFunctionCall(value)
						fmt.Fprintf(b, "%sstrcpy(%s, %s);\n", indent, varName, value)
					} else {
						if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
							// Already a string literal with quotes, use as-is
							fmt.Fprintf(b, "%sstrcpy(%s, %s);\n", indent, varName, value)
						} else {
							// Variable name or expression, don't add quotes - use as-is
							fmt.Fprintf(b, "%sstrcpy(%s, %s);\n", indent, varName, value)
						}
					}
				} else {
					if isFunctionCall(value) {
						if _, isListVar := globalArrays[varName]; isListVar {
							var (
								funcName, args   = parseFunctionCall(value)
								resolvedFuncName = lexer.ResolveSymbol(funcName, currentModule)
							)
							if returnsListType, _ := functionReturnsList(resolvedFuncName); returnsListType {
								resolvedArgs := make([]string, len(args))
								for i, arg := range args {
									resolvedArgs[i] = lexer.ResolveSymbol(arg, currentModule)
								}

								// For functions that return lists, we need to pass:
								//
								// 1. Output array (same as varName)
								// 2. Max size (1000)
								// 3. All resolved args with their lengths if they are lists
								var callArgs []string
								callArgs = append(callArgs, varName)
								callArgs = append(callArgs, "1000")
								for _, arg := range resolvedArgs {
									callArgs = append(callArgs, arg)
									if _, isListArg := globalArrays[arg]; isListArg {
										callArgs = append(callArgs, arg+"_len")
									}
								}

								fmt.Fprintf(b, "%s%s_len = %s(%s);\n", indent, varName, resolvedFuncName, strings.Join(callArgs, ", "))
								break
							}
						}
						value = resolveFunctionCall(value)
					}
					fmt.Fprintf(b, "%s%s = %s;\n", indent, varName, value)
				}
			}
		case stmt.IndexAssign != nil:
			var (
				listName = lexer.ResolveSymbol(stmt.IndexAssign.ListName, currentModule)
				index    = stmt.IndexAssign.Index
				value    = stmt.IndexAssign.Value
			)

			index = lexer.ResolveSymbol(index, currentModule)
			value = lexer.ResolveSymbol(value, currentModule)
			value = fixFloatCastGranular(value)
			value = convertThisReferencesGranular(value)

			if listType, exists := globalArrays[stmt.IndexAssign.ListName]; exists {
				innerType := listType
				if strings.HasPrefix(listType, "list[") && strings.HasSuffix(listType, "]") {
					innerType = extractListInnerType(listType)
				}
				err := checkTypeCompatibility(fmt.Sprintf("%s[%s]", listName, index), innerType, value)
				if err != nil {
					fmt.Fprintf(b, "%s// %s\n", indent, err.Error())
					fmt.Fprintf(b, "%s#error \"%s\"\n", indent, err.Error())
					continue
				}
			}

			fmt.Fprintf(b, "%s%s[%s] = %s;\n", indent, listName, index, value)

		case stmt.ListDecl != nil:
			listType := stmt.ListDecl.Type
			listName := lexer.ResolveSymbol(stmt.ListDecl.Name, currentModule)
			globalArrays[stmt.ListDecl.Name] = stmt.ListDecl.Type

			if isComplexCollectionType(listType) {
				renderComplexListDecl(b, stmt.ListDecl, indent, currentModule)
			} else {
				cListType := mapTypeToCType(listType)
				if len(stmt.ListDecl.Elements) == 1 && !strings.Contains(stmt.ListDecl.Elements[0], ",") &&
					!strings.HasPrefix(stmt.ListDecl.Elements[0], "\"") && !strings.HasSuffix(stmt.ListDecl.Elements[0], "\"") &&
					!isNumericOrBoolean(stmt.ListDecl.Elements[0]) {
					// This is likely a variable assignment (e.g., list[int] sorted_list = input_list)
					sourceVar := lexer.ResolveSymbol(stmt.ListDecl.Elements[0], currentModule)

					if listType == "string" {
						fmt.Fprintf(b, "%s%s %s[1000][256];\n", indent, "char", listName)
						fmt.Fprintf(b, "%sint %s_len = %s_len;\n", indent, listName, sourceVar)
						fmt.Fprintf(b, "%sfor (int i = 0; i < %s_len; i++) {\n", indent, sourceVar)
						fmt.Fprintf(b, "%s    strcpy(%s[i], %s[i]);\n", indent, listName, sourceVar)
						fmt.Fprintf(b, "%s}\n", indent)
					} else {
						fmt.Fprintf(b, "%s%s %s[1000];\n", indent, cListType, listName)
						fmt.Fprintf(b, "%sint %s_len = %s_len;\n", indent, listName, sourceVar)
						fmt.Fprintf(b, "%sfor (int i = 0; i < %s_len; i++) {\n", indent, sourceVar)
						fmt.Fprintf(b, "%s    %s[i] = %s[i];\n", indent, listName, sourceVar)
						fmt.Fprintf(b, "%s}\n", indent)
					}
				} else {
					// Traditional list declaration with elements
					if listType == "string" {
						fmt.Fprintf(b, "%s%s %s[%d][256];\n", indent, "char", listName, len(stmt.ListDecl.Elements))
					} else {
						fmt.Fprintf(b, "%s%s %s[%d];\n", indent, cListType, listName, len(stmt.ListDecl.Elements))
					}
					for i, elem := range stmt.ListDecl.Elements {
						elem = lexer.ResolveSymbol(elem, currentModule)
						elem = convertNewToConstructor(elem)
						if listType == "string" {
							if !strings.HasPrefix(elem, "\"") && !strings.HasSuffix(elem, "\"") {
								elem = fmt.Sprintf("\"%s\"", elem)
							}
							fmt.Fprintf(b, "%sstrcpy(%s[%d], %s);\n", indent, listName, i, elem)
						} else {
							fmt.Fprintf(b, "%s%s[%d] = %s;\n", indent, listName, i, elem)
						}
					}
					fmt.Fprintf(b, "%sint %s_len = %d;\n", indent, listName, len(stmt.ListDecl.Elements))
				}
			}
		case stmt.ObjectDecl != nil:
			var (
				varName      = lexer.ResolveSymbol(stmt.ObjectDecl.Name, currentModule)
				typeName     = stmt.ObjectDecl.Type
				args         = stmt.ObjectDecl.Args
				resolvedType = typeName
			)

			if strings.Contains(typeName, ".") {
				parts := strings.Split(typeName, ".")
				resolvedType = lexer.GenerateUniqueSymbol(parts[1], parts[0])
			} else if moduleName, exists := isImportedType(typeName, program.Imports); exists {
				resolvedType = lexer.GenerateUniqueSymbol(typeName, moduleName)
			}

			objectInfo := &ObjectInfo{
				Name: stmt.ObjectDecl.Name,
				Type: typeName,
			}
			globalObjects[stmt.ObjectDecl.Name] = objectInfo
			constructorArgs := make([]string, 0)
			for _, arg := range args {
				if strings.Contains(typeName, ".") {
					parts := strings.Split(typeName, ".")
					if arg == parts[0] || arg == parts[1] {
						continue
					}
				}
				if arg != typeName && arg != resolvedType {
					if strings.HasPrefix(arg, "\"") && strings.HasSuffix(arg, "\"") {
						constructorArgs = append(constructorArgs, arg)
					} else {
						constructorArgs = append(constructorArgs, lexer.ResolveSymbol(arg, currentModule))
					}
				}
			}

			argsStr := strings.Join(constructorArgs, ", ")
			fmt.Fprintf(b, "%s%s* %s = %s_new(%s);\n", indent, resolvedType, varName, resolvedType, argsStr)

		case stmt.VarDeclMethodCall != nil:
			var (
				varType           = mapTypeToCType(stmt.VarDeclMethodCall.Type)
				varName           = lexer.ResolveSymbol(stmt.VarDeclMethodCall.Name, currentModule)
				objectName        = lexer.ResolveSymbol(stmt.VarDeclMethodCall.Object, currentModule)
				methodName        = stmt.VarDeclMethodCall.Method
				args              = make([]string, len(stmt.VarDeclMethodCall.Args))
				resolvedClassName string
			)

			if stmt.VarDeclMethodCall.Object == "this" {
				if className == "" {
					fmt.Println("\033[91mError: 'this' used outside of class context\033[0m")
					os.Exit(1)
				}
				resolvedClassName = className
			} else {
				for _, obj := range globalObjects {
					if obj.Name == stmt.VarDeclMethodCall.Object {
						resolvedClassName = obj.Type
						if strings.Contains(resolvedClassName, ".") {
							parts := strings.Split(resolvedClassName, ".")
							resolvedClassName = lexer.GenerateUniqueSymbol(parts[1], parts[0])
						} else if moduleName, exists := isImportedType(resolvedClassName, program.Imports); exists {
							resolvedClassName = lexer.GenerateUniqueSymbol(resolvedClassName, moduleName)
						}
						break
					}
				}
			}

			for i, arg := range stmt.VarDeclMethodCall.Args {
				args[i] = lexer.ResolveSymbol(arg, currentModule)
				args[i] = convertPropertyAccess(args[i])
			}
			argsStr := strings.Join(args, ", ")
			if resolvedClassName == "" {
				resolvedClassName = "unknown"
				fmt.Println("\033[91mUnknown class name for method call:\033[0m", stmt.VarDeclMethodCall.Object)
				fmt.Println("\033[91mObject name:\033[0m", stmt.VarDeclMethodCall.Object)
				fmt.Println("\033[91mMethod name:\033[0m", stmt.VarDeclMethodCall.Method)
				fmt.Println("\033[91mArgs:\033[0m", stmt.VarDeclMethodCall.Args)
				fmt.Println("\033[91mCompilation failed.\033[0m")
				os.Exit(1)
			}
			if argsStr == "" {
				cType := mapTypeToCType(varType)
				fmt.Fprintf(b, "%s%s %s = %s_%s(%s);\n", indent, cType, varName, resolvedClassName, methodName, objectName)
			} else {
				cType := mapTypeToCType(varType)
				fmt.Fprintf(b, "%s%s %s = %s_%s(%s, %s);\n", indent, cType, varName, resolvedClassName, methodName, objectName, argsStr)
			}
		case stmt.VarAssignMethodCall != nil:
			varName := lexer.ResolveSymbol(stmt.VarAssignMethodCall.Name, currentModule)
			objectName := lexer.ResolveSymbol(stmt.VarAssignMethodCall.Object, currentModule)
			methodName := stmt.VarAssignMethodCall.Method
			args := make([]string, len(stmt.VarAssignMethodCall.Args))
			for i, arg := range stmt.VarAssignMethodCall.Args {
				args[i] = lexer.ResolveSymbol(arg, currentModule)
			}
			argsStr := strings.Join(args, ", ")

			// Check if this is actually a complex expression that was misparsed
			// If the objectName contains operators, treat it as a regular assignment
			if strings.Contains(objectName, "+") || strings.Contains(objectName, "-") ||
				strings.Contains(objectName, "*") || strings.Contains(objectName, "/") ||
				strings.Contains(objectName, " ") {
				// This is a misparsed complex expression, reconstruct the value and treat as VarAssign
				var reconstructedValue string
				if argsStr == "" {
					reconstructedValue = fmt.Sprintf("%s.%s()", objectName, methodName)
				} else {
					reconstructedValue = fmt.Sprintf("%s.%s(%s)", objectName, methodName, argsStr)
				}

				reconstructedValue = fixFloatCastGranular(reconstructedValue)
				reconstructedValue = convertThisReferencesGranular(reconstructedValue)
				reconstructedValue = convertNewToConstructor(reconstructedValue)
				reconstructedValue = convertMethodCallToC(reconstructedValue)
				reconstructedValue = convertPropertyAccess(reconstructedValue)

				fmt.Fprintf(b, "%s%s = %s;\n", indent, varName, reconstructedValue)
				continue
			}

			var resolvedClassName string
			if stmt.VarAssignMethodCall.Object == "this" {
				if className == "" {
					fmt.Println("\033[91mError: 'this' used outside of class context\033[0m")
					os.Exit(1)
				}
				resolvedClassName = className
			} else {
				for _, obj := range globalObjects {
					if obj.Name == stmt.VarAssignMethodCall.Object {
						resolvedClassName = obj.Type
						if strings.Contains(resolvedClassName, ".") {
							parts := strings.Split(resolvedClassName, ".")
							resolvedClassName = lexer.GenerateUniqueSymbol(parts[1], parts[0])
						} else if moduleName, exists := isImportedType(resolvedClassName, program.Imports); exists {
							resolvedClassName = lexer.GenerateUniqueSymbol(resolvedClassName, moduleName)
						}
						break
					}
				}
			}
			if resolvedClassName == "" {
				resolvedClassName = "unknown"
			}
			if argsStr == "" {
				fmt.Fprintf(b, "%s%s = %s_%s(%s);\n", indent, varName, resolvedClassName, methodName, objectName)
			} else {
				fmt.Fprintf(b, "%s%s = %s_%s(%s, %s);\n", indent, varName, resolvedClassName, methodName, objectName, argsStr)
			}
		case stmt.VarDeclInferred != nil:
			var (
				varName = lexer.ResolveSymbol(stmt.VarDeclInferred.Name, currentModule)
				value   = lexer.ResolveSymbol(stmt.VarDeclInferred.Value, currentModule)
				varType = inferTypeFromValue(stmt.VarDeclInferred.Value)
				cType   = mapTypeToCType(varType)
			)
			if isFunctionCall(value) {
				funcName, _ := parseFunctionCall(value)
				resolvedFuncName := lexer.ResolveSymbol(funcName, currentModule)
				if functionReturnsString(resolvedFuncName) {
					varType = "string"
					cType = "char"
				}
			}

			if varType == "string" {
				fmt.Fprintf(b, "%s%s %s[256];\n", indent, cType, varName)
			} else if varType == "lstring" {
				fmt.Fprintf(b, "%s%s %s[10000];\n", indent, cType, varName)
			}

			if varType == "string" || varType == "lstring" {
				if value != "" {
					if isFunctionCall(value) {
						funcName, args := parseFunctionCall(value)
						resolvedFuncName := lexer.ResolveSymbol(funcName, currentModule)
						if functionReturnsString(resolvedFuncName) {
							if len(args) == 0 {
								fmt.Fprintf(b, "%s%s(%s);\n", indent, resolvedFuncName, varName)
							} else {
								resolvedArgs := make([]string, len(args))
								for i, arg := range args {
									resolvedArgs[i] = lexer.ResolveSymbol(arg, currentModule)
								}
								fmt.Fprintf(b, "%s%s(%s, %s);\n", indent, resolvedFuncName, varName, strings.Join(resolvedArgs, ", "))
							}
						} else {
							resolvedCall := resolveFunctionCall(value)
							fmt.Fprintf(b, "%sstrcpy(%s, %s);\n", indent, varName, resolvedCall)
						}
					} else {
						if !strings.HasPrefix(value, "\"") && !strings.HasSuffix(value, "\"") {
							value = fmt.Sprintf("\"%s\"", value)
						}
						fmt.Fprintf(b, "%sstrcpy(%s, %s);\n", indent, varName, value)
					}
				}
			} else {
				if isFunctionCall(value) {
					value = resolveFunctionCall(value)
				}
				fmt.Fprintf(b, "%s%s %s = %s;\n", indent, cType, varName, value)
			}
		case stmt.VarDeclRead != nil:
			var (
				varName   = lexer.ResolveSymbol(stmt.VarDeclRead.Name, currentModule)
				filePath  = stmt.VarDeclRead.FilePath
				fpVarName = fmt.Sprintf("fp_read_%s", varName)
			)
			fmt.Fprintf(b, "%schar* %s = NULL;\n", indent, varName)
			fmt.Fprintf(b, "%sFILE* %s = fopen(%s, \"r\");\n", indent, fpVarName, filePath)
			fmt.Fprintf(b, "%sif (%s != NULL) {\n", indent, fpVarName)
			fmt.Fprintf(b, "%sfseek(%s, 0, SEEK_END);\n", indent+"    ", fpVarName)
			fmt.Fprintf(b, "%slong size = ftell(%s);\n", indent+"    ", fpVarName)
			fmt.Fprintf(b, "%sfseek(%s, 0, SEEK_SET);\n", indent+"    ", fpVarName)
			if useGC {
				fmt.Fprintf(b, "%s%s = GC_malloc(size + 1);\n", indent+"    ", varName)
			} else {
				fmt.Fprintf(b, "%s%s = malloc(size + 1);\n", indent+"    ", varName)
			}
			fmt.Fprintf(b, "%sfread(%s, 1, size, %s);\n", indent+"    ", varName, fpVarName)
			fmt.Fprintf(b, "%s%s[size] = '\\0';\n", indent+"    ", varName)
			fmt.Fprintf(b, "%sfclose(%s);\n", indent+"    ", fpVarName)
			fmt.Fprintf(b, "%s}\n", indent)
		case stmt.VarDeclWrite != nil:
			var (
				content   = lexer.ResolveSymbol(stmt.VarDeclWrite.Content, currentModule)
				filePath  = fmt.Sprintf("\"%s\"", stmt.VarDeclWrite.FilePath)
				mode      = stmt.VarDeclWrite.Mode
				fpVarName = fmt.Sprintf("fp_write_%d", writeCounter)
				fileMode  string
			)
			writeCounter++
			switch mode {
			case "append!":
				fileMode = "\"a\""
			case "overwrite!":
				fileMode = "\"w\""
			}

			fmt.Fprintf(b, "%sFILE* %s = fopen(%s, %s);\n", indent, fpVarName, filePath, fileMode)
			fmt.Fprintf(b, "%sif (%s != NULL) {\n", indent, fpVarName)

			if strings.HasPrefix(content, "\"") && strings.HasSuffix(content, "\"") {
				fmt.Fprintf(b, "%s    fprintf(%s, \"%%s\", %s);\n", indent, fpVarName, content)
			} else {
				fmt.Fprintf(b, "%s    fprintf(%s, \"%%s\", %s);\n", indent, fpVarName, content)
			}

			fmt.Fprintf(b, "%s    fclose(%s);\n", indent, fpVarName)
			fmt.Fprintf(b, "%s}\n", indent)
		case stmt.MethodCall != nil:
			var (
				objectName        = stmt.MethodCall.Object
				methodName        = stmt.MethodCall.Method
				rawArgs           = stmt.MethodCall.Args
				reconstructedArgs = make([]string, 0, len(rawArgs))
				i                 = 0
			)

			for i < len(rawArgs) {
				arg := rawArgs[i]

				if strings.Contains(arg, "(") && !strings.Contains(arg, ")") {
					parenPos := strings.Index(arg, "(")
					if parenPos > 0 {
						beforeParen := arg[:parenPos]
						if strings.Contains(beforeParen, "::") || isValidIdentifier(beforeParen) {
							reconstructed := arg
							j := i + 1
							for j < len(rawArgs) && !strings.Contains(reconstructed, ")") {
								reconstructed += ", " + rawArgs[j]
								j++
							}

							reconstructedArgs = append(reconstructedArgs, reconstructed)
							i = j
							continue
						}
					}
				}

				reconstructedArgs = append(reconstructedArgs, arg)
				i++
			}

			args := make([]string, len(reconstructedArgs))
			for i, arg := range reconstructedArgs {
				resolvedArg := lexer.ResolveSymbol(arg, currentModule)
				args[i] = processStringFunctionArg(resolvedArg)
			}
			argsStr := strings.Join(args, ", ")

			logger.Debug("Method call - object: '%s', method: '%s', args: %v\n", objectName, methodName, stmt.MethodCall.Args)
			logger.Debug("Current class name: '%s'\n", className)

			if objectName == "this" {
				resolvedClassName := className
				if resolvedClassName == "" {
					for _, s := range program.Statements {
						if s.ClassDecl != nil {
							resolvedClassName = s.ClassDecl.Name
							break
						}
						if s.PubClassDecl != nil {
							resolvedClassName = s.PubClassDecl.Name
							break
						}
					}
					// TODO: This is terrible.
					if resolvedClassName == "" {
						resolvedClassName = "Matrix"
					}
				}

				methodExists := false
				for _, s := range program.Statements {
					var class *lexer.ClassDeclStmt
					if s.ClassDecl != nil && s.ClassDecl.Name == resolvedClassName {
						class = s.ClassDecl
					} else if s.PubClassDecl != nil && s.PubClassDecl.Name == resolvedClassName {
						class = &lexer.ClassDeclStmt{
							Name:    s.PubClassDecl.Name,
							Methods: s.PubClassDecl.Methods,
						}
					}

					if class != nil {
						for _, method := range class.Methods {
							if method.Name == methodName {
								methodExists = true
								break
							}
						}
						if methodExists {
							break
						}
					}
				}

				if !methodExists {
					fmt.Printf("Warning: Method '%s' not found in class '%s'\n", methodName, resolvedClassName)
				}

				if argsStr == "" {
					fmt.Fprintf(b, "%s%s_%s(this);\n", indent, resolvedClassName, methodName)
				} else {
					fmt.Fprintf(b, "%s%s_%s(this, %s);\n", indent, resolvedClassName, methodName, argsStr)
				}
			} else {
				if strings.HasPrefix(objectName, "new_") && strings.Contains(objectName, "(") {
					newPrefix := objectName[4:]
					parenPos := strings.Index(newPrefix, "(")
					if parenPos != -1 {
						className := newPrefix[:parenPos]
						constructorArgs := newPrefix[parenPos+1 : len(newPrefix)-1]
						var constructorCall string
						if constructorArgs == "" {
							constructorCall = fmt.Sprintf("%s_new()", className)
						} else {
							constructorCall = fmt.Sprintf("%s_new(%s)", className, constructorArgs)
						}

						if argsStr == "" {
							fmt.Fprintf(b, "%s%s_%s(%s);\n", indent, className, methodName, constructorCall)
						} else {
							fmt.Fprintf(b, "%s%s_%s(%s, %s);\n", indent, className, methodName, constructorCall, argsStr)
						}
						continue
					}
				}

				objectName = lexer.ResolveSymbol(objectName, currentModule)
				var resolvedClassName string
				for _, obj := range globalObjects {
					if obj.Name == stmt.MethodCall.Object {
						resolvedClassName = obj.Type
						if strings.Contains(resolvedClassName, ".") {
							parts := strings.Split(resolvedClassName, ".")
							resolvedClassName = lexer.GenerateUniqueSymbol(parts[1], parts[0])
						} else if moduleName, exists := isImportedType(resolvedClassName, program.Imports); exists {
							resolvedClassName = lexer.GenerateUniqueSymbol(resolvedClassName, moduleName)
						}
						break
					}
				}
				if resolvedClassName == "" {
					for className, classInfo := range globalClasses {
						for _, method := range classInfo.Methods {
							if method.Name == methodName {
								resolvedClassName = className
								logger.Debug("Inferred object '%s' as type '%s' based on method '%s'\n", stmt.MethodCall.Object, className, methodName)
								break
							}
						}
						if resolvedClassName != "" {
							break
						}
					}
					if resolvedClassName == "" {
						resolvedClassName = "unknown"
					}
				}
				if argsStr == "" {
					fmt.Fprintf(b, "%s%s_%s(%s);\n", indent, resolvedClassName, methodName, objectName)
				} else {
					fmt.Fprintf(b, "%s%s_%s(%s, %s);\n", indent, resolvedClassName, methodName, objectName, argsStr)
				}
			}
		case stmt.StaticMethodCall != nil:
			className := stmt.StaticMethodCall.Class
			methodName := stmt.StaticMethodCall.Method
			rawArgs := stmt.StaticMethodCall.Args

			// Resolve class name if it's an imported type
			if moduleName, exists := isImportedType(className, program.Imports); exists {
				className = lexer.GenerateUniqueSymbol(className, moduleName)
			}

			var args []string
			for _, arg := range rawArgs {
				resolvedArg := lexer.ResolveSymbol(arg, currentModule)
				resolvedArg = convertThisReferencesGranular(resolvedArg)
				args = append(args, resolvedArg)
			}
			argsStr := strings.Join(args, ", ")

			fmt.Fprintf(b, "%s%s_%s(%s);\n", indent, className, methodName, argsStr)
		case stmt.FunctionCall != nil:
			funcName := lexer.ResolveSymbol(stmt.FunctionCall.Name, currentModule)
			funcName = convertNewToConstructor(funcName)
			args := make([]string, 0)

			if functionReturnsString(funcName) {
				fmt.Fprintf(b, "%s{\n", indent)
				fmt.Fprintf(b, "%s    char temp_buffer[256];\n", indent)
				fmt.Fprintf(b, "%s    %s(temp_buffer", indent, funcName)
				for _, arg := range stmt.FunctionCall.Args {
					resolvedArg := lexer.ResolveSymbol(arg, currentModule)
					fmt.Fprintf(b, ", %s", resolvedArg)
				}
				fmt.Fprintf(b, ");\n")
				fmt.Fprintf(b, "%s}\n", indent)
			} else {
				for _, arg := range stmt.FunctionCall.Args {
					resolvedArg := lexer.ResolveSymbol(arg, currentModule)
					args = append(args, resolvedArg)
					if _, exists := globalArrays[arg]; exists {
						args = append(args, fmt.Sprintf("len(%s)", arg))
					}
				}
				argsStr := strings.Join(args, ", ")
				fmt.Fprintf(b, "%s%s(%s);\n", indent, funcName, argsStr)
			}
		case stmt.RawCode != nil:
			rawLines := strings.Split(stmt.RawCode.Code, "\n")
			for _, rawLine := range rawLines {
				if strings.TrimSpace(rawLine) != "" {
					fmt.Fprintf(b, "%s%s\n", indent, rawLine)
				} else {
					b.WriteString("\n")
				}
			}
		case stmt.MapDecl != nil:
			var (
				mapName     = lexer.ResolveSymbol(stmt.MapDecl.Name, currentModule)
				keyType     = stmt.MapDecl.KeyType
				valueType   = stmt.MapDecl.ValueType
				cKeyType    = mapTypeToCType(keyType)
				cValueType  = mapTypeToCType(valueType)
				mapSize     = len(stmt.MapDecl.Pairs)
				initialSize = mapSize
			)

			if initialSize == 0 {
				initialSize = 10
			}

			if keyType == "string" {
				fmt.Fprintf(b, "%schar %s_keys[%d][256];\n", indent, mapName, initialSize)
			} else {
				fmt.Fprintf(b, "%s %s_keys[%d];\n", cKeyType, mapName, initialSize)
			}

			if valueType == "string" {
				fmt.Fprintf(b, "%schar %s_values[%d][256];\n", indent, mapName, initialSize)
			} else {
				fmt.Fprintf(b, "%s %s_values[%d];\n", cValueType, mapName, initialSize)
			}

			fmt.Fprintf(b, "%sint %s_size = %d;\n", indent, mapName, mapSize)

			if indent != "" {
				generateLocalMapAccessHelper(b, mapName, keyType, valueType, indent)
			} else {
				generateMapAccessHelper(b, mapName, keyType, valueType)
			}

			if len(stmt.MapDecl.Pairs) > 0 {
				for i, pair := range stmt.MapDecl.Pairs {
					key := pair.Key
					value := pair.Value

					if keyType == "string" {
						if !strings.HasPrefix(key, "\"") && !strings.HasSuffix(key, "\"") {
							key = fmt.Sprintf("\"%s\"", key)
						}
						fmt.Fprintf(b, "%sstrcpy(%s_keys[%d], %s);\n", indent, mapName, i, key)
					} else {
						fmt.Fprintf(b, "%s%s_keys[%d] = %s;\n", indent, mapName, i, key)
					}

					if valueType == "string" {
						if !strings.HasPrefix(value, "\"") && !strings.HasSuffix(value, "\"") {
							value = fmt.Sprintf("\"%s\"", value)
						}
						fmt.Fprintf(b, "%sstrcpy(%s_values[%d], %s);\n", indent, mapName, i, value)
					} else {
						fmt.Fprintf(b, "%s%s_values[%d] = %s;\n", indent, mapName, i, value)
					}
				}
			}

		case stmt.GetMap != nil:
			expr := renderMapAccess(stmt.GetMap.MapName, stmt.GetMap.Key, program)
			fmt.Fprintf(b, "%s%s", indent, expr)

		case stmt.PutMap != nil:
			mapName := lexer.ResolveSymbol(stmt.PutMap.MapName, currentModule)
			key := stmt.PutMap.Key
			value := stmt.PutMap.Value

			var keyType, valueType string
			for _, s := range program.Statements {
				if s.MapDecl != nil && s.MapDecl.Name == stmt.PutMap.MapName {
					keyType = s.MapDecl.KeyType
					valueType = s.MapDecl.ValueType
					break
				}
				// Also check inside class declarations (both public and regular)
				if s.ClassDecl != nil && s.ClassDecl.Constructor != nil {
					for _, field := range s.ClassDecl.Constructor.Fields {
						if field.MapDecl != nil && field.MapDecl.Name == stmt.PutMap.MapName {
							keyType = field.MapDecl.KeyType
							valueType = field.MapDecl.ValueType
							break
						}
					}
					if keyType != "" {
						break
					}
				}
				if s.PubClassDecl != nil && s.PubClassDecl.Constructor != nil {
					for _, field := range s.PubClassDecl.Constructor.Fields {
						if field.MapDecl != nil && field.MapDecl.Name == stmt.PutMap.MapName {
							keyType = field.MapDecl.KeyType
							valueType = field.MapDecl.ValueType
							break
						}
					}
					if keyType != "" {
						break
					}
				}
			}

			// Fallback: If we couldn't find the map type in top-level declarations,
			// try to infer it from common patterns for local maps
			if keyType == "" && valueType == "" {
				keyType = "string"
				valueType = "bool"
				if value == "true" || value == "false" {
					valueType = "bool"
				} else if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
					valueType = "string"
				} else if strings.Contains(value, ".") {
					valueType = "float"
				} else {
					if _, err := strconv.Atoi(value); err == nil {
						valueType = "int"
					}
				}
			}

			fmt.Fprintf(b, "%s{\n", indent)
			fmt.Fprintf(b, "%s    int found = 0;\n", indent)
			fmt.Fprintf(b, "%s    for (int i = 0; i < %s_size; i++) {\n", indent, mapName)
			if keyType == "string" {
				if strings.HasPrefix(key, "\"") && strings.HasSuffix(key, "\"") {
					keyStr := key[1 : len(key)-1]                     // Remove quotes
					keyStr = strings.ReplaceAll(keyStr, "\"", "\\\"") // Escape quotes in the string
					fmt.Fprintf(b, "%s        if (strcmp(%s_keys[i], \"%s\") == 0) {\n", indent, mapName, keyStr)
				} else {
					resolvedKey := lexer.ResolveSymbol(key, currentModule)
					fmt.Fprintf(b, "%s        if (strcmp(%s_keys[i], %s) == 0) {\n", indent, mapName, resolvedKey)
				}
			} else {
				fmt.Fprintf(b, "%s        if (%s_keys[i] == %s) {\n", indent, mapName, key)
			}

			if valueType == "string" {
				if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
					fmt.Fprintf(b, "%s            strcpy(%s_values[i], %s);\n", indent, mapName, value)
				} else {
					resolvedValue := lexer.ResolveSymbol(value, currentModule)
					fmt.Fprintf(b, "%s            strcpy(%s_values[i], %s);\n", indent, mapName, resolvedValue)
				}
			} else {
				resolvedValue := lexer.ResolveSymbol(value, currentModule)
				fmt.Fprintf(b, "%s            %s_values[i] = %s;\n", indent, mapName, resolvedValue)
			}

			fmt.Fprintf(b, "%s            found = 1;\n", indent)
			fmt.Fprintf(b, "%s            break;\n", indent)
			fmt.Fprintf(b, "%s        }\n", indent)
			fmt.Fprintf(b, "%s    }\n", indent)

			fmt.Fprintf(b, "%s    if (!found && %s_size < 100) {\n", indent, mapName)

			if keyType == "string" {
				if strings.HasPrefix(key, "\"") && strings.HasSuffix(key, "\"") {
					keyStr := key[1 : len(key)-1]                     // Remove quotes
					keyStr = strings.ReplaceAll(keyStr, "\"", "\\\"") // Escape quotes in the string
					fmt.Fprintf(b, "%s        strcpy(%s_keys[%s_size], \"%s\");\n", indent, mapName, mapName, keyStr)
				} else {
					resolvedKey := lexer.ResolveSymbol(key, currentModule)
					fmt.Fprintf(b, "%s        strcpy(%s_keys[%s_size], %s);\n", indent, mapName, mapName, resolvedKey)
				}
			} else {
				resolvedKey := lexer.ResolveSymbol(key, currentModule)
				fmt.Fprintf(b, "%s        %s_keys[%s_size] = %s;\n", indent, mapName, mapName, resolvedKey)
			}
			if valueType == "string" {
				if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
					fmt.Fprintf(b, "%s        strcpy(%s_values[%s_size], %s);\n", indent, mapName, mapName, value)
				} else {
					resolvedValue := lexer.ResolveSymbol(value, currentModule)
					fmt.Fprintf(b, "%s        strcpy(%s_values[%s_size], %s);\n", indent, mapName, mapName, resolvedValue)
				}
			} else {
				resolvedValue := lexer.ResolveSymbol(value, currentModule)
				fmt.Fprintf(b, "%s        %s_values[%s_size] = %s;\n", indent, mapName, mapName, resolvedValue)
			}

			fmt.Fprintf(b, "%s        %s_size++;\n", indent, mapName)
			fmt.Fprintf(b, "%s    }\n", indent)
			fmt.Fprintf(b, "%s}\n", indent)
		case stmt.ParallelFor != nil:
			varName := lexer.ResolveSymbol(stmt.ParallelFor.Var, currentModule)
			start := lexer.ResolveSymbol(stmt.ParallelFor.Start, currentModule)
			end := lexer.ResolveSymbol(stmt.ParallelFor.End, currentModule)
			end = convertThisReferencesGranular(end)

			// Build the OpenMP pragma with reduction clauses
			pragma := "#pragma omp parallel for"

			// Add reduction clauses if present
			if len(stmt.ParallelFor.Reductions) > 0 {
				// Group reductions by operation type
				reductionGroups := make(map[string][]string)
				for _, reduction := range stmt.ParallelFor.Reductions {
					var ompOp string
					switch reduction.Operation {
					case "sum":
						ompOp = "+"
					case "max":
						ompOp = "max"
					case "min":
						ompOp = "min"
					case "product":
						ompOp = "*"
					default:
						ompOp = reduction.Operation // Use as-is for custom operations
					}
					reductionVar := lexer.ResolveSymbol(reduction.Variable, currentModule)
					reductionGroups[ompOp] = append(reductionGroups[ompOp], reductionVar)
				}

				// Build separate reduction clauses for each operation
				var allReductions []string
				for op, vars := range reductionGroups {
					reductionClause := fmt.Sprintf("reduction(%s:%s)", op, strings.Join(vars, ","))
					allReductions = append(allReductions, reductionClause)
				}
				pragma += " " + strings.Join(allReductions, " ")
			}

			fmt.Fprintf(b, "%s%s\n", indent, pragma)

			// Generate the for loop with optional step
			if stmt.ParallelFor.Step != "" && stmt.ParallelFor.Step != "1" {
				step := lexer.ResolveSymbol(stmt.ParallelFor.Step, currentModule)
				step = convertThisReferencesGranular(step)
				fmt.Fprintf(b, "%sfor (int %s = %s; %s <= %s; %s += %s) {\n", indent, varName, start, varName, end, varName, step)
			} else {
				fmt.Fprintf(b, "%sfor (int %s = %s; %s <= %s; %s++) {\n", indent, varName, start, varName, end, varName)
			}
			renderStatements(b, stmt.ParallelFor.Body, indent+"    ", className, program, currentFunctionReturnType)
			fmt.Fprintf(b, "%s}\n", indent)
		case stmt.ParallelWhile != nil:
			condition := stmt.ParallelWhile.Condition
			condition = processNotKeyword(condition)
			condition = lexer.ResolveSymbol(condition, currentModule)
			condition = convertThisReferencesGranular(condition)
			fmt.Fprintf(b, "%s#pragma omp parallel\n", indent)
			fmt.Fprintf(b, "%s{\n", indent)
			fmt.Fprintf(b, "%s    while (%s) {\n", indent, condition)
			fmt.Fprintf(b, "%s        #pragma omp single nowait\n", indent)
			fmt.Fprintf(b, "%s        {\n", indent)
			renderStatements(b, stmt.ParallelWhile.Body, indent+"            ", className, program, currentFunctionReturnType)
			fmt.Fprintf(b, "%s        }\n", indent)
			fmt.Fprintf(b, "%s    }\n", indent)
			fmt.Fprintf(b, "%s}\n", indent)
		case stmt.ParallelBlock != nil:
			fmt.Fprintf(b, "%s#pragma omp parallel sections\n", indent)
			fmt.Fprintf(b, "%s{\n", indent)
			for _, blockStmt := range stmt.ParallelBlock.Body {
				fmt.Fprintf(b, "%s    #pragma omp section\n", indent)
				fmt.Fprintf(b, "%s    {\n", indent)
				renderStatements(b, []*lexer.Statement{blockStmt}, indent+"        ", className, program, currentFunctionReturnType)
				fmt.Fprintf(b, "%s    }\n", indent)
			}
			fmt.Fprintf(b, "%s}\n", indent)

		case stmt.Allocate != nil:
			varType := stmt.Allocate.Type
			varName := lexer.ResolveSymbol(stmt.Allocate.Name, currentModule)
			size := lexer.ResolveSymbol(stmt.Allocate.Size, currentModule)
			cType := mapTypeToCType(varType)

			varName = convertPropertyAccess(varName)

			logger.Debug(
				"Allocate - original varName: %s, after convertPropertyAccess: %s\n",
				lexer.ResolveSymbol(stmt.Allocate.Name, currentModule),
				varName,
			)

			if strings.HasPrefix(varName, "this->") {
				logger.Debug("Member variable allocation: %s\n", varName)
				if useGC {
					fmt.Fprintf(b, "%s%s = (%s*)GC_malloc(%s * sizeof(%s));\n", indent, varName, cType, size, cType)
				} else {
					fmt.Fprintf(b, "%s%s = (%s*)malloc(%s * sizeof(%s));\n", indent, varName, cType, size, cType)
				}
			} else {
				logger.Debug("Local variable allocation: %s\n", varName)
				if useGC {
					fmt.Fprintf(b, "%s%s* %s = (%s*)GC_malloc(%s * sizeof(%s));\n", indent, cType, varName, cType, size, cType)
				} else {
					fmt.Fprintf(b, "%s%s* %s = (%s*)malloc(%s * sizeof(%s));\n", indent, cType, varName, cType, size, cType)
				}
			}

		case stmt.PubAllocate != nil:
			// Public allocations are handled as global variables, skip in function body

		case stmt.StackAllocate != nil:
			varType := stmt.StackAllocate.Type
			varName := lexer.ResolveSymbol(stmt.StackAllocate.Name, currentModule)
			size := lexer.ResolveSymbol(stmt.StackAllocate.Size, currentModule)
			cType := mapTypeToCType(varType)

			varName = convertPropertyAccess(varName)

			// Stack allocation using Variable Length Arrays (VLA)
			if strings.HasPrefix(varName, "this->") {
				// Member variables can't be stack allocated, fall back to heap
				fmt.Printf("Error: Member variable %s can't be stack allocated\n", varName)
				os.Exit(1)
				// if useGC {
				// 	fmt.Fprintf(b, "%s%s = (%s*)GC_malloc(%s * sizeof(%s));\n", indent, varName, cType, size, cType)
				// } else {
				// 	fmt.Fprintf(b, "%s%s = (%s*)malloc(%s * sizeof(%s));\n", indent, varName, cType, size, cType)
				// }
			} else {
				logger.Debug("Stack variable allocation: %s\n", varName)
				// Use Variable Length Arrays for stack allocation
				fmt.Fprintf(b, "%s%s %s[%s];\n", indent, cType, varName, size)
			}

		case stmt.Free != nil:
			variable := lexer.ResolveSymbol(stmt.Free.Variable, currentModule)

			if useGC {
				fmt.Fprintf(b, "%s%s = NULL;\n", indent, variable)
			} else {
				fmt.Fprintf(b, "%sfree(%s);\n", indent, variable)
				fmt.Fprintf(b, "%s%s = NULL;\n", indent, variable)
			}
		case stmt.NewExpr != nil:
			className := stmt.NewExpr.ClassName
			args := make([]string, 0)
			for _, arg := range stmt.NewExpr.Args {
				resolvedArg := lexer.ResolveSymbol(arg, currentModule)
				args = append(args, resolvedArg)
			}
			argsStr := strings.Join(args, ", ")
			fmt.Fprintf(b, "%s%s_new(%s);\n", indent, className, argsStr)
		}
	}
}

func fixFloatCastGranular(expr string) string {
	return strings.ReplaceAll(expr, "float(", "(float)(")
}

// Converts 'new ClassName(args)' to 'ClassName_new(args)'
func convertNewToConstructor(expr string) string {
	if !strings.HasPrefix(expr, "new ") {
		return expr
	}

	src := strings.TrimSpace(expr[3:])
	if src == "" {
		return fmt.Sprintf("%s_new()", strings.TrimSpace(src))
	}
	parenPos := strings.Index(src, "(")
	if parenPos == -1 {
		return fmt.Sprintf("%s_new()", strings.TrimSpace(src))
	}

	className := strings.TrimSpace(src[:parenPos])
	closeParen := findMatchingParen(src, parenPos)
	if closeParen == -1 {
		return expr
	}
	args := src[parenPos : closeParen+1]

	return fmt.Sprintf("%s_new%s", className, args)
}

func reconstructMethodCalls(variables []string) []string {
	if len(variables) <= 1 {
		return variables
	}

	var result []string
	i := 0

	for i < len(variables) {
		variable := variables[i]
		if strings.Contains(variable, ".") && strings.Contains(variable, "(") && !strings.Contains(variable, ")") {
			reconstructed := variable
			i++
			for i < len(variables) {
				reconstructed += ", " + variables[i]
				if strings.Contains(variables[i], ")") {
					break
				}
				i++
			}
			result = append(result, reconstructed)
		} else {
			result = append(result, variable)
		}
		i++
	}

	return result
}

func convertThisReferencesGranular(expr string) string {
	if expr == "" {
		return expr
	}
	var stringLiterals []string
	reString := regexp.MustCompile(`"(?:\\.|[^"\\])*"`)
	expr = reString.ReplaceAllStringFunc(expr, func(match string) string {
		stringLiterals = append(stringLiterals, match)
		return fmt.Sprintf("__STRING_LITERAL_%d__", len(stringLiterals)-1)
	})
	if isMethodCall(expr) {
		expr = convertMethodCallToC(expr)
	}

	expr = strings.ReplaceAll(expr, "->", "__ARROW__")
	expr = strings.Join(strings.Fields(expr), " ")
	expr = strings.ReplaceAll(expr, "__ARROW__", "->")

	reModuleMember := regexp.MustCompile(`\b([a-zA-Z_][a-zA-Z0-9]*)\.([A-Z_][A-Z0-9_]*)\b`)
	expr = reModuleMember.ReplaceAllString(expr, "${1}_$2")
	reThisMember := regexp.MustCompile(`(^|\s|\(|\[|,|\+|-|\*|/|%|&|\||\^|!|~|\?|:|=|\{|\}|;|,|\s)this\s*\.\s*([a-zA-Z_][a-zA-Z0-9]*)`)
	expr = reThisMember.ReplaceAllString(expr, "${1}this->$2")

	// Handle pointer member access for non-this objects
	reObjMember := regexp.MustCompile(`\b([a-zA-Z_][a-zA-Z0-9]*)\s*\.\s*([a-zA-Z_][a-zA-Z0-9]*)\b`)
	expr = reObjMember.ReplaceAllStringFunc(expr, func(match string) string {
		parts := strings.Split(match, ".")
		if len(parts) == 2 {
			varName := strings.TrimSpace(parts[0])
			fieldName := strings.TrimSpace(parts[1])

			for _, obj := range globalObjects {
				if obj.Name == varName {
					return fmt.Sprintf("%s->%s", varName, fieldName)
				}
			}

			return fmt.Sprintf("%s->%s", varName, fieldName)
		}
		return match
	})

	expr = strings.ReplaceAll(expr, "->->", "->")
	// Don't remove spaces around -> as it can cause parsing issues
	// expr = strings.ReplaceAll(expr, "-> ", "->")
	// expr = strings.ReplaceAll(expr, " ->", "->")
	expr = strings.ReplaceAll(expr, "this ->", "this->")

	// Handle nil conversion
	expr = strings.ReplaceAll(expr, " nil", " NULL")
	expr = strings.ReplaceAll(expr, "nil ", "NULL ")
	expr = strings.ReplaceAll(expr, "(nil)", "(NULL)")
	if expr == "nil" {
		expr = "NULL"
	}

	expr = convertPropertyAccess(expr)
	if currentClassName != "" {
		reThisMethodCall := regexp.MustCompile(`this->([a-zA-Z_][a-zA-Z0-9_]*)\((.*?)\)`)
		expr = reThisMethodCall.ReplaceAllStringFunc(expr, func(match string) string {
			submatches := reThisMethodCall.FindStringSubmatch(match)
			if len(submatches) == 3 {
				methodName := submatches[1]
				args := submatches[2]
				if strings.TrimSpace(args) == "" {
					return fmt.Sprintf("%s_%s(this)", currentClassName, methodName)
				} else {
					return fmt.Sprintf("%s_%s(this, %s)", currentClassName, methodName, args)
				}
			}
			return match
		})
	}

	if isMethodCall(expr) {
		expr = convertMethodCallToC(expr)
	}

	for i, lit := range stringLiterals {
		expr = strings.Replace(expr, fmt.Sprintf("__STRING_LITERAL_%d__", i), lit, 1)
	}

	return expr
}

// Processes all get! expressions in a string and replaces them with the C code
func processGetExpressions(expr string, program *lexer.Program) string {
	logger.Debug("processGetExpressions called with: '%s'\n", expr)
	re := regexp.MustCompile(`get!\s*\(([^)]+)\)((?:\.[a-zA-Z_][a-zA-Z0-9_]*)*)`)
	matches := re.FindAllStringSubmatchIndex(expr, -1)
	if len(matches) == 0 {
		logger.Debug("processGetExpressions: no matches found\n")
		return expr
	}
	logger.Debug("processGetExpressions: found %d matches\n", len(matches))

	result := expr
	offset := 0

	for _, match := range matches {
		fullMatch := expr[match[0]:match[1]]
		argsStr := expr[match[2]:match[3]]
		var propertyAccess string
		if len(match) > 4 && match[4] != -1 {
			propertyAccess = expr[match[4]:match[5]]
		}

		args := []string{}
		start := 0
		parenCount := 0

		for i, c := range argsStr {
			switch c {
			case '(':
				parenCount++
			case ')':
				parenCount--
			case ',':
				if parenCount == 0 {
					arg := strings.TrimSpace(argsStr[start:i])
					if arg != "" {
						args = append(args, arg)
					}
					start = i + 1
				}
			}
		}

		if start < len(argsStr) {
			arg := strings.TrimSpace(argsStr[start:])
			if arg != "" {
				args = append(args, arg)
			}
		}
		if len(args) == 2 {
			var (
				mapName = args[0]
				key     = args[1]
				cCode   = renderMapAccess(mapName, key, program)
				before  = result[:match[0]+offset]
				after   = result[match[1]+offset:]
			)

			if propertyAccess != "" {
				propertyAccess = strings.ReplaceAll(propertyAccess, ".", "->")
				cCode = cCode + propertyAccess
			}

			result = before + cCode + after
			offset += len(cCode) - len(fullMatch)
		}
	}

	return result
}

// Processes all has! expressions in a string and replaces them with the C code
func processHasExpressions(expr string, program *lexer.Program) string {
	re := regexp.MustCompile(`has!\s*\(([^)]+)\)`)
	matches := re.FindAllStringSubmatchIndex(expr, -1)
	if len(matches) == 0 {
		return expr
	}

	result := expr
	offset := 0

	for _, match := range matches {
		fullMatch := expr[match[0]:match[1]]
		argsStr := expr[match[2]:match[3]]
		args := []string{}
		start := 0
		parenCount := 0

		for i, c := range argsStr {
			switch c {
			case '(':
				parenCount++
			case ')':
				parenCount--
			case ',':
				if parenCount == 0 {
					arg := strings.TrimSpace(argsStr[start:i])
					if arg != "" {
						args = append(args, arg)
					}
					start = i + 1
				}
			}
		}

		if start < len(argsStr) {
			arg := strings.TrimSpace(argsStr[start:])
			if arg != "" {
				args = append(args, arg)
			}
		}
		if len(args) == 2 {
			var (
				mapName = args[0]
				key     = args[1]
				cCode   = renderMapHas(mapName, key, program)
				before  = result[:match[0]+offset]
				after   = result[match[1]+offset:]
			)
			result = before + cCode + after
			offset += len(cCode) - len(fullMatch)
		}
	}

	return result
}

func resolveImportedSymbols(value string, imports []*lexer.ImportStmt) string {
	if strings.Contains(value, ".") {
		parts := strings.Split(value, ".")
		if len(parts) == 2 {
			var (
				moduleName = parts[0]
				symbolName = parts[1]
			)
			for _, imp := range imports {
				if imp.Module == moduleName {
					resolved := lexer.GenerateUniqueSymbol(symbolName, moduleName)
					return strings.Replace(value, moduleName+"."+symbolName, resolved, 1)
				}
			}
		}
	}
	result := value
	for _, imp := range imports {
		modulePrefix := imp.Module + "."
		if strings.Contains(result, modulePrefix) {
			tokens := regexp.MustCompile(`([\w\.]+|\S)`).FindAllString(result, -1)

			for i, token := range tokens {
				if strings.HasPrefix(token, modulePrefix) {
					symbolName := token[len(modulePrefix):]
					if i+1 >= len(tokens) || tokens[i+1] != "." {
						tokens[i] = lexer.GenerateUniqueSymbol(symbolName, imp.Module)
					}
				}
			}
			result = strings.Join(tokens, "")
		}
	}

	for _, imp := range imports {
		modulePrefix := imp.Module + "."
		if strings.Contains(result, modulePrefix) {
			re := regexp.MustCompile(`\b` + regexp.QuoteMeta(modulePrefix) + `(\w+)\b`)
			result = re.ReplaceAllStringFunc(result, func(match string) string {
				symbolName := match[len(modulePrefix):]
				return lexer.GenerateUniqueSymbol(symbolName, imp.Module)
			})
		}
	}

	return result
}

// Determines if the given expression is a method call
func isMethodCall(expr string) bool {
	dotIndex := strings.Index(expr, ".")
	if dotIndex == -1 {
		return false
	}

	parenIndex := strings.Index(expr[dotIndex:], "(")
	if parenIndex == -1 {
		return false
	}

	return strings.Contains(expr, ")")
}

// Finds the position of the matching closing parenthesis
func findMatchingParen(s string, openPos int) int {
	if openPos < 0 || openPos >= len(s) || s[openPos] != '(' {
		return -1
	}

	stack := 1
	for i := openPos + 1; i < len(s); i++ {
		switch s[i] {
		case '(':
			stack++
		case ')':
			stack--
			if stack == 0 {
				return i
			}
		}
	}
	return -1
}

func convertMethodCallToC(expr string) string {
	logger.Debug("convertMethodCallToC called with: '%s'\n", expr)
	expr = strings.ReplaceAll(expr, "->", "__ARROW__")

	bitwiseOps := map[string]string{
		"b_or":     "|",
		"b_and":    "&",
		"b_xor":    "^",
		"b_lshift": "<<",
		"b_rshift": ">>",
	}

	result := expr
	for bitwiseOp, cOp := range bitwiseOps {
		oldResult := result
		// Handle multiple patterns for each operator:
		// " b_op " - spaces on both sides
		// " b_op(" - space before, paren after
		// ")b_op " - paren before, space after
		// ")b_op(" - parens on both sides
		// "Nb_op " - number/identifier before, space after
		result = strings.ReplaceAll(result, " "+bitwiseOp+" ", " "+cOp+" ")
		result = strings.ReplaceAll(result, " "+bitwiseOp+"(", " "+cOp+"(")
		result = strings.ReplaceAll(result, ")"+bitwiseOp+" ", ")"+cOp+" ")
		result = strings.ReplaceAll(result, ")"+bitwiseOp+"(", ")"+cOp+"(")

		// Handle cases where operators follow numbers or identifiers directly
		// Use regex to match word characters or digits followed by space + operator + space
		pattern := regexp.MustCompile(`(\w)\s+` + regexp.QuoteMeta(bitwiseOp) + `\s+`)
		matches := pattern.FindAllString(result, -1)
		if len(matches) > 0 {
			logger.Debug("Found regex matches for %s in '%s': %v\n", bitwiseOp, result, matches)
		}
		beforeRegex := result
		result = pattern.ReplaceAllString(result, "$1 "+cOp+" ")
		if result != beforeRegex {
			logger.Debug("Regex replacement for %s: '%s' became '%s'\n", bitwiseOp, beforeRegex, result)
		}

		if result != oldResult {
			logger.Debug("Bitwise conversion %s -> %s: '%s' became '%s'\n", bitwiseOp, cOp, oldResult, result)
		}
	}

	// If we made any bitwise conversions, return early with method call conversion
	if result != expr {
		methodCallPattern := regexp.MustCompile(`(?:\bthis\.\w+|\b\w+)\.[a-zA-Z_]\w*\([^)]*\)`)
		methodCalls := methodCallPattern.FindAllString(result, -1)
		for _, methodCall := range methodCalls {
			converted := convertSingleMethodCall(methodCall)
			if converted != "" && converted != methodCall {
				result = strings.ReplaceAll(result, methodCall, converted)
			}
		}
		return result
	}

	if strings.Contains(expr, "<<") || strings.Contains(expr, ">>") {
		var (
			methodCallPattern = regexp.MustCompile(`(?:\bthis\.\w+|\b\w+)\.[a-zA-Z_]\w*\([^)]*\)`)
			result            = expr
			methodCalls       = methodCallPattern.FindAllString(expr, -1)
		)
		for _, methodCall := range methodCalls {
			converted := convertSingleMethodCall(methodCall)
			if converted != "" && converted != methodCall {
				result = strings.ReplaceAll(result, methodCall, converted)
			}
		}
		return result
	}

	comparisonOps := []string{"==", "!=", ">=", "<=", ">", "<"}
	var op, left, right string
	var hasComparison bool
	for _, cmpOp := range comparisonOps {
		if strings.Contains(expr, cmpOp) {
			parts := strings.SplitN(expr, cmpOp, 2)
			if len(parts) == 2 {
				left = strings.TrimSpace(parts[0])
				right = strings.TrimSpace(parts[1])
				op = cmpOp
				hasComparison = true
				break
			}
		}
	}
	if hasComparison && (strings.HasPrefix(left, "this.") || strings.Contains(left, "(")) {
		convertedLeft := convertSingleMethodCall(left)
		if convertedLeft != "" {
			return fmt.Sprintf("%s %s %s", convertedLeft, op, right)
		}
	}

	arithmeticOps := []string{"+", "-", "*", "/", "%"}
	for _, arithOp := range arithmeticOps {
		if strings.Contains(expr, arithOp) {
			result := expr
			methodCallPattern := regexp.MustCompile(`(?:\bthis\.\w+|\b\w+)\.[a-zA-Z_]\w*\([^)]*\)`)
			methodCalls := methodCallPattern.FindAllString(expr, -1)

			for _, methodCall := range methodCalls {
				converted := convertSingleMethodCall(methodCall)
				if converted != "" && converted != methodCall {
					result = strings.ReplaceAll(result, methodCall, converted)
				}
			}

			if result != expr {
				return result
			}
		}
	}

	var hasArithmetic bool
	for _, arithOp := range arithmeticOps {
		opIndex := -1
		parenCount := 0
		for i, char := range expr {
			if char == '(' {
				parenCount++
			} else if char == ')' {
				parenCount--
			} else if parenCount == 0 && strings.HasPrefix(expr[i:], arithOp) {
				opIndex = i
				break
			}
		}

		if opIndex > 0 {
			left = strings.TrimSpace(expr[:opIndex])
			right = strings.TrimSpace(expr[opIndex+len(arithOp):])
			op = arithOp
			hasArithmetic = true
			break
		}
	}

	if hasArithmetic && isMethodCall(left) {
		convertedLeft := convertSingleMethodCall(left)
		if convertedLeft != "" {
			return fmt.Sprintf("%s %s %s", convertedLeft, op, right)
		}
	}

	result = convertSingleMethodCall(expr)
	result = strings.ReplaceAll(result, "__ARROW__", "->")

	return result
}

func generateMapAccessHelper(b *strings.Builder, mapName, keyType, valueType string) {
	helperName := fmt.Sprintf("__get_%s_value", mapName)
	var cKeyType string
	switch keyType {
	case "string":
		cKeyType = "char*"
	case "char":
		cKeyType = "char"
	default:
		cKeyType = mapTypeToCType(keyType)
	}

	var cValueType string
	switch valueType {
	case "string":
		cValueType = "char*"
	case "char":
		cValueType = "char"
	default:
		cValueType = mapTypeToCType(valueType)
	}

	fmt.Fprintf(b, "%s %s(%s key) {\n", cValueType, helperName, cKeyType)
	fmt.Fprintf(b, "    for (int i = 0; i < %s_size; i++) {\n", mapName)

	if keyType == "string" {
		fmt.Fprintf(b, "        if (strcmp(%s_keys[i], key) == 0) {\n", mapName)
	} else {
		fmt.Fprintf(b, "        if (%s_keys[i] == key) {\n", mapName)
	}

	fmt.Fprintf(b, "            return %s_values[i];\n", mapName)
	fmt.Fprintf(b, "        }\n")
	fmt.Fprintf(b, "    }\n")

	switch valueType {
	case "string":
		fmt.Fprintf(b, "    return \"\";\n")
	case "char":
		fmt.Fprintf(b, "    return '\\0';\n")
	default:
		fmt.Fprintf(b, "    return 0;\n")
	}

	fmt.Fprintf(b, "}\n\n")

	if keyType == "string" {
		fmt.Fprintf(b, "char* __get_%s_key_at(int index) {\n", mapName)
		fmt.Fprintf(b, "    if (index >= 0 && index < %s_size) {\n", mapName)
		fmt.Fprintf(b, "        return %s_keys[index];\n", mapName)
		fmt.Fprintf(b, "    }\n")
		fmt.Fprintf(b, "    return \"\";\n")
		fmt.Fprintf(b, "}\n\n")
	} else {
		cKeyType := mapTypeToCType(keyType)
		fmt.Fprintf(b, "%s __get_%s_key_at(int index) {\n", cKeyType, mapName)
		fmt.Fprintf(b, "    if (index >= 0 && index < %s_size) {\n", mapName)
		fmt.Fprintf(b, "        return %s_keys[index];\n", mapName)
		fmt.Fprintf(b, "    }\n")
		if keyType == "char" {
			fmt.Fprintf(b, "    return '\\0';\n")
		} else {
			fmt.Fprintf(b, "    return 0;\n")
		}
		fmt.Fprintf(b, "}\n\n")
	}

	fmt.Fprintf(b, "int __get_%s_size() {\n", mapName)
	fmt.Fprintf(b, "    return %s_size;\n", mapName)
	fmt.Fprintf(b, "}\n\n")
}

func generateLocalMapAccessHelper(b *strings.Builder, mapName, keyType, valueType string, indent string) {
	var cKeyType string
	switch keyType {
	case "string":
		cKeyType = "char*"
	case "char":
		cKeyType = "char"
	default:
		cKeyType = mapTypeToCType(keyType)
	}
	var cValueType string
	switch valueType {
	case "string":
		cValueType = "char*"
	case "char":
		cValueType = "char"
	default:
		cValueType = mapTypeToCType(valueType)
	}
	helperName := fmt.Sprintf("__get_%s_value", mapName)
	fmt.Fprintf(b, "%s%s %s(%s key) {\n", indent, cValueType, helperName, cKeyType)
	fmt.Fprintf(b, "%s    for (int i = 0; i < %s_size; i++) {\n", indent, mapName)
	if keyType == "string" {
		fmt.Fprintf(b, "%s        if (strcmp(%s_keys[i], key) == 0) {\n", indent, mapName)
	} else {
		fmt.Fprintf(b, "%s        if (%s_keys[i] == key) {\n", indent, mapName)
	}
	fmt.Fprintf(b, "%s            return %s_values[i];\n", indent, mapName)
	fmt.Fprintf(b, "%s        }\n", indent)
	fmt.Fprintf(b, "%s    }\n", indent)
	switch valueType {
	case "string":
		fmt.Fprintf(b, "%s    return \"\";\n", indent)
	case "char":
		fmt.Fprintf(b, "%s    return '\\0';\n", indent)
	default:
		fmt.Fprintf(b, "%s    return 0;\n", indent)
	}
	fmt.Fprintf(b, "%s}\n\n", indent)
	if keyType == "string" {
		fmt.Fprintf(b, "%schar* __get_%s_key_at(int index) {\n", indent, mapName)
		fmt.Fprintf(b, "%s    if (index >= 0 && index < %s_size) {\n", indent, mapName)
		fmt.Fprintf(b, "%s        return %s_keys[index];\n", indent, mapName)
		fmt.Fprintf(b, "%s    }\n", indent)
		fmt.Fprintf(b, "%s    return \"\";\n", indent)
		fmt.Fprintf(b, "%s}\n\n", indent)
	} else {
		cKeyType := mapTypeToCType(keyType)
		fmt.Fprintf(b, "%s%s __get_%s_key_at(int index) {\n", indent, cKeyType, mapName)
		fmt.Fprintf(b, "%s    if (index >= 0 && index < %s_size) {\n", indent, mapName)
		fmt.Fprintf(b, "%s        return %s_keys[index];\n", indent, mapName)
		fmt.Fprintf(b, "%s    }\n", indent)
		if keyType == "char" {
			fmt.Fprintf(b, "%s    return '\\0';\n", indent)
		} else {
			fmt.Fprintf(b, "%s    return 0;\n", indent)
		}
		fmt.Fprintf(b, "%s}\n\n", indent)
	}

	fmt.Fprintf(b, "%sint __get_%s_size() {\n", indent, mapName)
	fmt.Fprintf(b, "%s    return %s_size;\n", indent, mapName)
	fmt.Fprintf(b, "%s}\n\n", indent)
	fmt.Fprintf(b, "%sint __has_%s_key(%s key) {\n", indent, mapName, cKeyType)
	fmt.Fprintf(b, "%s    for (int i = 0; i < %s_size; i++) {\n", indent, mapName)
	if keyType == "string" {
		fmt.Fprintf(b, "%s        if (strcmp(%s_keys[i], key) == 0) {\n", indent, mapName)
	} else {
		fmt.Fprintf(b, "%s        if (%s_keys[i] == key) {\n", indent, mapName)
	}
	fmt.Fprintf(b, "%s            return 1;\n", indent)
	fmt.Fprintf(b, "%s        }\n", indent)
	fmt.Fprintf(b, "%s    }\n", indent)
	fmt.Fprintf(b, "%s    return 0;\n", indent)
	fmt.Fprintf(b, "%s}\n\n", indent)
}

// Generates a C expression for accessing a map value by key
func renderMapAccess(mapName, key string, program *lexer.Program) string {
	if after, ok := strings.CutPrefix(mapName, "this."); ok {
		fieldName := after
		var keyType string
		for _, s := range program.Statements {
			if s.ClassDecl != nil && s.ClassDecl.Constructor != nil {
				for _, stmt := range s.ClassDecl.Constructor.Fields {
					if stmt.MapDecl != nil && strings.HasPrefix(stmt.MapDecl.Name, "this.") {
						declFieldName := strings.TrimPrefix(stmt.MapDecl.Name, "this.")
						if declFieldName == fieldName {
							keyType = stmt.MapDecl.KeyType
							break
						}
					}
				}
			}
			if s.PubClassDecl != nil && s.PubClassDecl.Constructor != nil {
				for _, stmt := range s.PubClassDecl.Constructor.Fields {
					if stmt.MapDecl != nil && strings.HasPrefix(stmt.MapDecl.Name, "this.") {
						declFieldName := strings.TrimPrefix(stmt.MapDecl.Name, "this.")
						if declFieldName == fieldName {
							keyType = stmt.MapDecl.KeyType
							break
						}
					}
				}
			}
		}

		helperName := fmt.Sprintf("%s_get_%s_value", currentClassName, fieldName)

		var keyExpr string
		if keyType == "string" {
			if strings.HasPrefix(key, "\"") && strings.HasSuffix(key, "\"") {
				keyStr := key[1 : len(key)-1]
				keyStr = strings.ReplaceAll(keyStr, "\"", "\\\"")
				keyExpr = fmt.Sprintf(`"%s"`, keyStr)
			} else {
				keyExpr = lexer.ResolveSymbol(key, currentModule)
			}
		} else {
			if !unicode.IsDigit(rune(key[0])) && !(key[0] == '-' && len(key) > 1 && unicode.IsDigit(rune(key[1]))) {
				key = lexer.ResolveSymbol(key, currentModule)
			}
			keyExpr = key
		}

		return fmt.Sprintf("%s(this, %s)", helperName, keyExpr)
	}

	resolvedMapName := lexer.ResolveSymbol(mapName, currentModule)

	// Find the map to determine key type
	var keyType string
	for _, s := range program.Statements {
		if s.MapDecl != nil && s.MapDecl.Name == mapName {
			keyType = s.MapDecl.KeyType
			break
		}
	}

	helperName := fmt.Sprintf("__get_%s_value", resolvedMapName)

	var keyExpr string
	if keyType == "string" || keyType == "char*" {
		if strings.HasPrefix(key, "\"") && strings.HasSuffix(key, "\"") {
			keyStr := key[1 : len(key)-1]                     // Remove quotes
			keyStr = strings.ReplaceAll(keyStr, "\"", "\\\"") // Escape quotes
			keyExpr = fmt.Sprintf(`"%s"`, keyStr)
		} else {
			keyExpr = lexer.ResolveSymbol(key, currentModule)
			if keyExpr == key && len(key) == 3 && key[0] == '\'' && key[2] == '\'' {
				keyExpr = fmt.Sprintf("'%c'", key[1])
			}
		}
	} else {
		if !unicode.IsDigit(rune(key[0])) && !(key[0] == '-' && len(key) > 1 && unicode.IsDigit(rune(key[1]))) {
			key = lexer.ResolveSymbol(key, currentModule)
		}
		keyExpr = key
		if keyType == "char" && len(keyExpr) == 3 && keyExpr[0] == '\'' && keyExpr[2] == '\'' {
			keyExpr = fmt.Sprintf("'%c'", keyExpr[1])
		}
	}

	return fmt.Sprintf("%s(%s)", helperName, keyExpr)
}

func renderMapHas(mapName, key string, program *lexer.Program) string {
	if after, ok := strings.CutPrefix(mapName, "this."); ok {
		fieldName := after
		var keyType string
		for _, s := range program.Statements {
			if s.ClassDecl != nil && s.ClassDecl.Constructor != nil {
				for _, stmt := range s.ClassDecl.Constructor.Fields {
					if stmt.MapDecl != nil && strings.HasPrefix(stmt.MapDecl.Name, "this.") {
						declFieldName := strings.TrimPrefix(stmt.MapDecl.Name, "this.")
						if declFieldName == fieldName {
							keyType = stmt.MapDecl.KeyType
							break
						}
					}
				}
			}
			if s.PubClassDecl != nil && s.PubClassDecl.Constructor != nil {
				for _, stmt := range s.PubClassDecl.Constructor.Fields {
					if stmt.MapDecl != nil && strings.HasPrefix(stmt.MapDecl.Name, "this.") {
						declFieldName := strings.TrimPrefix(stmt.MapDecl.Name, "this.")
						if declFieldName == fieldName {
							keyType = stmt.MapDecl.KeyType
							break
						}
					}
				}
			}
		}

		var keyExpr string
		if keyType == "string" {
			if strings.HasPrefix(key, "\"") && strings.HasSuffix(key, "\"") {
				keyStr := key[1 : len(key)-1]
				keyStr = strings.ReplaceAll(keyStr, "\"", "\\\"")
				keyExpr = fmt.Sprintf(`"%s"`, keyStr)
			} else {
				resolvedKey := lexer.ResolveSymbol(key, "")
				keyExpr = resolvedKey
			}
			return fmt.Sprintf("(__check_string_key_exists(this->%s_keys, this->%s_size, %s))", fieldName, fieldName, keyExpr)
		} else {
			if !unicode.IsDigit(rune(key[0])) && !(key[0] == '-' && len(key) > 1 && unicode.IsDigit(rune(key[1]))) {
				key = lexer.ResolveSymbol(key, "")
			}
			keyExpr = key
			return fmt.Sprintf("(__check_key_exists(this->%s_keys, this->%s_size, %s))", fieldName, fieldName, keyExpr)
		}
	}

	resolvedMapName := lexer.ResolveSymbol(mapName, "")
	return fmt.Sprintf("(__has_%s_key(%s))", resolvedMapName, lexer.ResolveSymbol(key, ""))
}

func convertSingleMethodCall(expr string) string {
	if strings.Contains(expr, "this.") {
		startIdx := strings.Index(expr, "this.")
		if startIdx == -1 {
			return expr
		}

		prefix := expr[:startIdx]

		dotIndex := startIdx + 4
		if dotIndex >= len(expr) || expr[dotIndex] != '.' {
			return expr
		}

		restOfExpr := expr[dotIndex+1:]

		nextDotIndex := strings.Index(restOfExpr, ".")
		if nextDotIndex != -1 {
			fieldName := restOfExpr[:nextDotIndex]
			methodPart := restOfExpr[nextDotIndex+1:]

			parenIndex := strings.Index(methodPart, "(")
			if parenIndex != -1 {
				methodName := methodPart[:parenIndex]
				closeParen := findMatchingParen(methodPart, parenIndex)
				if closeParen == -1 {
					return expr
				}

				args := ""
				if closeParen > parenIndex+1 {
					args = methodPart[parenIndex+1 : closeParen]
				}

				suffix := ""
				if closeParen+1 < len(methodPart) {
					suffix = methodPart[closeParen+1:]
				}

				className := ""
				logger.Debug("Looking up field type for '%s', currentClassName='%s'\n", fieldName, currentClassName)

				if currentClassName != "" {
					if classInfo, exists := globalClasses[currentClassName]; exists {
						for _, field := range classInfo.Fields {
							if field.Name == fieldName {
								fieldType := field.Type
								if after, ok := strings.CutPrefix(fieldType, "ref "); ok {
									fieldType = after
								}
								className = fieldType
								logger.Debug("Found field '%s' of type '%s' in class '%s'\n", fieldName, fieldType, currentClassName)
								break
							}
						}
					}
				}

				if className == "" {
					logger.Debug("Searching all classes for field '%s'\n", fieldName)
					for classNameIter, classInfo := range globalClasses {
						for _, field := range classInfo.Fields {
							if field.Name == fieldName {
								fieldType := field.Type
								if after, ok := strings.CutPrefix(fieldType, "ref "); ok {
									fieldType = after
								}
								className = fieldType
								logger.Debug("Found field '%s' of type '%s' in class '%s'\n", fieldName, fieldType, classNameIter)
								break
							}
						}
						if className != "" {
							break
						}
					}
				}

				logger.Debug("Final className for field '%s': '%s'\n", fieldName, className)
				if className == "" {
					return expr
				}

				objectRef := fmt.Sprintf("this->%s", fieldName)

				var result string
				if args == "" {
					result = fmt.Sprintf("%s%s_%s(%s)%s", prefix, className, methodName, objectRef, suffix)
					logger.Debug("Generated method call (no args): '%s'\n", result)
				} else {
					processedArgs := processMethodArguments(args)
					result = fmt.Sprintf("%s%s_%s(%s, %s)%s", prefix, className, methodName, objectRef, processedArgs, suffix)
					logger.Debug("Generated method call (with args): '%s'\n", result)
				}

				return result
			}
		}

		parenIndex := strings.Index(expr[dotIndex:], "(")
		if parenIndex == -1 {
			fieldName := expr[dotIndex+1:]
			return fmt.Sprintf("%sthis->%s", prefix, fieldName)
		}

		parenIndex += dotIndex
		var (
			methodName = expr[dotIndex+1 : parenIndex]
			className  = currentClassName
		)
		if className == "" {
			return expr
		}
		closeParen := findMatchingParen(expr, parenIndex)
		if closeParen == -1 {
			return expr
		}

		suffix := ""
		if closeParen+1 < len(expr) {
			suffix = expr[closeParen+1:]
		}

		args := ""
		if closeParen > parenIndex+1 {
			args = expr[parenIndex+1 : closeParen]
		}

		if args != "" {
			if strings.Contains(args, "this.") {
				nestedProcessed := convertMethodCallToC(args)
				if nestedProcessed != args {
					expr = expr[:parenIndex+1] + nestedProcessed + expr[closeParen:]
					return convertSingleMethodCall(expr)
				}
			}
		}
		if args == "" {
			return fmt.Sprintf("%s%s_%s(this)%s", prefix, className, methodName, suffix)
		}
		return fmt.Sprintf("%s%s_%s(this, %s)%s", prefix, className, methodName, args, suffix)
	}

	dotIndex := strings.Index(expr, ".")
	if dotIndex == -1 {
		return expr
	}

	parenIndex := strings.Index(expr[dotIndex:], "(")
	if parenIndex == -1 {
		return expr
	}
	parenIndex += dotIndex

	if !strings.Contains(expr, ")") {
		return expr
	}

	objectName := strings.TrimSpace(expr[:dotIndex])
	methodName := strings.TrimSpace(expr[dotIndex+1 : parenIndex])

	closeParen := findMatchingParen(expr, parenIndex)
	if closeParen == -1 {
		return expr
	}

	args := ""
	if closeParen > parenIndex+1 {
		args = expr[parenIndex+1 : closeParen]
	}

	var resolvedClassName string
	var resolvedObjectName string

	if strings.HasPrefix(objectName, "this.") {
		fieldName := objectName[5:] // Remove "this."
		resolvedClassName = currentClassName
		resolvedObjectName = fmt.Sprintf("this->%s", fieldName)
	} else if strings.Contains(objectName, "__ARROW__") {
		if strings.HasPrefix(objectName, "this__ARROW__") {
			fieldName := objectName[13:]
			resolvedObjectName = fmt.Sprintf("this->%s", fieldName)
			if currentClassName != "" {
				if classInfo, exists := globalClasses[currentClassName]; exists {
					for _, field := range classInfo.Fields {
						if field.Name == fieldName {
							fieldType := field.Type
							if after, ok := strings.CutPrefix(fieldType, "ref "); ok {
								fieldType = after
							}
							resolvedClassName = fieldType
							logger.Debug("Found field '%s' of type '%s' in class '%s'\n", fieldName, fieldType, currentClassName)
							break
						}
					}
				}
			}
			if resolvedClassName == "" {
				logger.Debug("Could not resolve field type for '%s' in class '%s'\n", fieldName, currentClassName)
				return expr
			}
		} else {
			// For other __ARROW__ patterns (like object__ARROW__method), treat as normal object method call

			resolvedObjectName = strings.ReplaceAll(objectName, "__ARROW__", "->")
			baseObjectName := objectName
			if arrowIndex := strings.Index(baseObjectName, "__ARROW__"); arrowIndex != -1 {
				baseObjectName = baseObjectName[:arrowIndex]
			}

			logger.Debug("Looking for object '%s' in globalObjects\n", baseObjectName)
			for objName, obj := range globalObjects {
				if objName == baseObjectName {
					resolvedClassName = obj.Type
					if strings.Contains(resolvedClassName, ".") {
						parts := strings.Split(resolvedClassName, ".")
						resolvedClassName = lexer.GenerateUniqueSymbol(parts[1], parts[0])
					}
					logger.Debug("Found object '%s' of type '%s' in globalObjects\n", baseObjectName, resolvedClassName)
					break
				}
			}

			if resolvedClassName == "" {
				logger.Debug("Could not resolve object '%s' from globalObjects, checking if it's a method call pattern\n", baseObjectName)
				return strings.ReplaceAll(expr, "__ARROW__", "->")
			}
		}
	} else {
		for objName, obj := range globalObjects {
			if objName == objectName {
				resolvedClassName = obj.Type
				if strings.Contains(resolvedClassName, ".") {
					parts := strings.Split(resolvedClassName, ".")
					resolvedClassName = lexer.GenerateUniqueSymbol(parts[1], parts[0])
				}
				break
			}
		}
		if resolvedClassName == "" && currentFunction != nil {
			for _, param := range currentFunction.Parameters {
				if param.Name == objectName {
					paramType := param.Type
					if param.IsRef && strings.HasPrefix(paramType, "ref ") {
						paramType = strings.TrimPrefix(paramType, "ref ")
					}
					resolvedClassName = paramType
					break
				}
			}
		}

		if resolvedClassName == "" {
			return expr
		}

		resolvedObjectName = lexer.ResolveSymbol(objectName, currentModule)
	}

	if args == "" {
		return fmt.Sprintf("%s_%s(%s)", resolvedClassName, methodName, resolvedObjectName)
	}

	processedArgs := processMethodArguments(args)
	return fmt.Sprintf("%s_%s(%s, %s)", resolvedClassName, methodName, resolvedObjectName, processedArgs)
}

func generateTopLevelFunctionImplementation(b *strings.Builder, funcDecl *lexer.TopLevelFuncDeclStmt, program *lexer.Program) {
	currentFunction = funcDecl
	defer func() { currentFunction = nil }()

	// This means return array length.
	returnType := "int"

	if strings.HasPrefix(funcDecl.ReturnType, "list[") && strings.HasSuffix(funcDecl.ReturnType, "]") {
		returnType = "int"
	} else if funcDecl.ReturnType != "" && funcDecl.ReturnType != "void" {
		if funcDecl.ReturnType == "string" {
			returnType = "void"
		} else {
			returnType = mapTypeToCType(funcDecl.ReturnType)
		}
	} else {
		returnType = "void"
	}

	fmt.Fprintf(b, "%s %s(", returnType, funcDecl.Name)

	paramList := make([]string, 0)

	if strings.HasPrefix(funcDecl.ReturnType, "list[") && strings.HasSuffix(funcDecl.ReturnType, "]") {
		innerType := strings.TrimPrefix(strings.TrimSuffix(funcDecl.ReturnType, "]"), "list[")
		if innerType == "string" {
			paramList = append(paramList, "char _output_array[][256]")
		} else {
			cType := mapTypeToCType(innerType)
			paramList = append(paramList, fmt.Sprintf("%s _output_array[]", cType))
		}
		paramList = append(paramList, "int _max_size")
	} else if funcDecl.ReturnType == "string" {
		paramList = append(paramList, "char* _output_buffer")
	}

	for _, param := range funcDecl.Parameters {
		var (
			paramType = mapTypeToCType(param.Type)
			paramName = param.Name
		)
		if param.IsList || strings.HasPrefix(param.Type, "list[") {
			if param.Type == "string" {
				paramList = append(paramList, fmt.Sprintf("char %s[][256]", paramName))
			} else {
				paramList = append(paramList, fmt.Sprintf("%s %s[]", paramType, paramName))
			}
			paramList = append(paramList, fmt.Sprintf("int %s_len", paramName))
		} else {
			if param.Type == "string" {
				paramType = "char*"
			}
			if param.IsRef && !strings.HasSuffix(paramType, "*") {
				paramType = paramType + "*"
			}
			paramList = append(paramList, fmt.Sprintf("%s %s", paramType, paramName))
		}
	}

	b.WriteString(strings.Join(paramList, ", "))
	b.WriteString(") {\n")

	if funcDecl.ReturnType == "string" {
		for _, stmt := range funcDecl.Body {
			if stmt.RawCode != nil {
				modifiedCode := strings.ReplaceAll(stmt.RawCode.Code, "return buffer;", "strcpy(_output_buffer, buffer); return;")
				modifiedCode = strings.ReplaceAll(modifiedCode, `return "";`, `strcpy(_output_buffer, ""); return;`)
				rawLines := strings.SplitSeq(modifiedCode, "\n")
				for rawLine := range rawLines {
					if strings.TrimSpace(rawLine) != "" {
						fmt.Fprintf(b, "    %s\n", rawLine)
					} else {
						b.WriteString("\n")
					}
				}
			} else if stmt.Return != nil {
				value := stmt.Return.Value
				if isFunctionCall(value) {
					funcName, args := parseFunctionCall(value)
					resolvedFuncName := lexer.ResolveSymbol(funcName, currentModule)
					if functionReturnsString(resolvedFuncName) {
						if len(args) == 0 {
							fmt.Fprintf(b, "    %s(_output_buffer);\n", resolvedFuncName)
						} else {
							resolvedArgs := make([]string, len(args))
							for i, arg := range args {
								resolvedArgs[i] = lexer.ResolveSymbol(arg, currentModule)
							}
							fmt.Fprintf(b, "    %s(_output_buffer, %s);\n", resolvedFuncName, strings.Join(resolvedArgs, ", "))
						}
						fmt.Fprintf(b, "    return;\n")
					} else {
						resolvedCall := resolveFunctionCall(value)
						fmt.Fprintf(b, "    strcpy(_output_buffer, %s);\n", resolvedCall)
						fmt.Fprintf(b, "    return;\n")
					}
				} else {
					value = strings.ReplaceAll(value, "this.", "this->")
					value = lexer.ResolveSymbol(value, currentModule)
					if value == `""` {
						fmt.Fprintf(b, "    strcpy(_output_buffer, \"\");\n")
					} else {
						fmt.Fprintf(b, "    strcpy(_output_buffer, %s);\n", value)
					}
					fmt.Fprintf(b, "    return;\n")
				}
			} else {
				renderStatements(b, []*lexer.Statement{stmt}, "    ", "", program, funcDecl.ReturnType)
			}
		}
	} else {
		renderStatements(b, funcDecl.Body, "    ", "", program, funcDecl.ReturnType)
	}

	b.WriteString("}\n\n")
}

func isNumericOrBoolean(s string) bool {
	if s == "true" || s == "false" {
		return true
	}
	if _, err := strconv.Atoi(s); err == nil {
		return true
	}
	if _, err := strconv.ParseFloat(s, 64); err == nil {
		return true
	}
	return false
}

func isValidIdentifier(s string) bool {
	if s == "" {
		return false
	}
	if !unicode.IsLetter(rune(s[0])) && s[0] != '_' {
		return false
	}
	for _, c := range s[1:] {
		if !unicode.IsLetter(c) && !unicode.IsDigit(c) && c != '_' {
			return false
		}
	}
	return true
}

// Type checking functions
func isStringLiteral(value string) bool {
	return strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")
}

func isArithmeticExpression(value string) bool {
	if len(value) > 1 && (value[0] == '-' || value[0] == '+') {
		remaining := strings.TrimSpace(value[1:])
		if !isNumericLiteral(remaining) && remaining != "" {
			return true
		}
	}
	operators := []string{"+", "-", "*", "/", "%", "b_xor", "b_and", "b_or", "<<", ">>"}
	for _, op := range operators {
		if strings.Contains(value, " "+op+" ") {
			return true
		}
	}
	return false
}

func isNumericLiteral(value string) bool {
	_, errInt := strconv.Atoi(value)
	_, errFloat := strconv.ParseFloat(value, 64)
	return errInt == nil || errFloat == nil
}

func inferArithmeticExpressionType(value string) string {
	// Handle unary operators
	if len(value) > 1 && (value[0] == '-' || value[0] == '+') {
		operand := strings.TrimSpace(value[1:])
		if !isNumericLiteral(operand) {
			operandType := inferValueType(operand)
			if isNumericType(operandType) {
				return operandType
			}
		}
	}

	// Handle binary operators
	operators := []string{" + ", " - ", " * ", " / ", " % "}

	for _, op := range operators {
		if strings.Contains(value, op) {
			parts := strings.Split(value, op)
			if len(parts) >= 2 {
				leftType := inferValueType(strings.TrimSpace(parts[0]))
				rightType := inferValueType(strings.TrimSpace(parts[1]))

				// If both operands are numbers, return the "higher" type
				if isNumericType(leftType) && isNumericType(rightType) {
					if leftType == "f32" || leftType == "float" || rightType == "f32" || rightType == "float" {
						return "f32"
					}
					if leftType == "f64" || leftType == "double" || rightType == "f64" || rightType == "double" {
						return "f64"
					}
					return "i32"
				}
			}
			break
		}
	}
	return "unknown"
}

func isNumericType(typeName string) bool {
	numericTypes := []string{"i32", "int", "f32", "float", "f64", "double", "i16", "i64", "u16", "u32", "u64"}
	return slices.Contains(numericTypes, typeName)
}

func inferValueType(value string) string {
	if isStringLiteral(value) {
		return "string"
	}

	if value == "true" || value == "false" {
		return "bool"
	}

	if isArithmeticExpression(value) {
		return inferArithmeticExpressionType(value)
	}

	if _, err := strconv.Atoi(value); err == nil {
		return "i32"
	}

	if _, err := strconv.ParseFloat(value, 64); err == nil {
		return "f32"
	}

	if localType, exists := localVars[value]; exists {
		return localType
	}

	if globalVar, exists := globalVars[value]; exists {
		return globalVar.Type
	}

	return "unknown"
}

func areTypesCompatible(targetType, valueType string) bool {
	if targetType == valueType {
		return true
	}
	if (targetType == "i32" || targetType == "int") && (valueType == "i32" || valueType == "int") {
		return true
	}
	if (targetType == "f32" || targetType == "float") && (valueType == "f32" || valueType == "float") {
		return true
	}
	if (targetType == "f64" || targetType == "double") && (valueType == "f64" || valueType == "double") {
		return true
	}
	if (targetType == "f32" || targetType == "float" || targetType == "f64" || targetType == "double") &&
		(valueType == "i32" || valueType == "int") {
		return true
	}
	return false
}

func checkTypeCompatibility(varName, varType, value string) error {
	valueType := inferValueType(value)

	if valueType == "unknown" || valueType == "string" {
		if isFunctionCall(value) {
			return nil
		}
		if value == "\"\"" {
			return nil
		}
		if regexp.MustCompile(`^".*"$`).MatchString(value) {
			return nil
		}

		if isValidIdentifier(value) {
			if _, exists := localVars[value]; exists {
				return nil
			}
			if _, exists := globalVars[value]; exists {
				return nil
			}
			if isSimpleIdentifier(value) {
				return nil
			}
		}

		// This handles cases like complex arithmetic, method calls, array access, etc.
		if containsValidExpressionElements(value) {
			return nil
		}

		logger.ErrorAndExit(
			fmt.Sprintf(
				"TypeError: Unknown identifier or type for value '%s' when assigning to variable '%s' of type '%s'", value, varName, varType,
			),
		)
	}

	if !areTypesCompatible(varType, valueType) {
		logger.ErrorAndExit(
			fmt.Sprintf(
				"TypeError: Cannot assign value of type '%s' to variable '%s' of type '%s'", valueType, varName, varType,
			),
		)
	}

	return nil
}

func isSimpleIdentifier(s string) bool {
	return isValidIdentifier(s) && !strings.Contains(s, ".") && !strings.Contains(s, "->")
}

func containsValidExpressionElements(value string) bool {
	if isArithmeticExpression(value) {
		return true
	}
	comparisonOps := []string{"==", "!=", "<", ">", "<=", ">="}
	for _, op := range comparisonOps {
		if strings.Contains(value, " "+op+" ") {
			return true
		}
	}
	logicalOps := []string{"&&", "||", "and", "or"}
	for _, op := range logicalOps {
		if strings.Contains(value, " "+op+" ") {
			return true
		}
	}
	bitwiseOps := []string{"b_xor", "b_and", "b_or", "<<", ">>", "^"}
	for _, op := range bitwiseOps {
		if strings.Contains(value, " "+op+" ") {
			return true
		}
	}
	if strings.Contains(value, ".") && strings.Contains(value, "(") {
		return true
	}
	if strings.Contains(value, "[") && strings.Contains(value, "]") {
		return true
	}
	if strings.Contains(value, "(") && strings.Contains(value, ")") {
		return true
	}
	if strings.Contains(value, "->") {
		return true
	}
	return false
}

// Generates a C function prototype for a class method
func generateMethodPrototype(className, methodName, returnType string, parameters []*lexer.MethodParameter, isStatic bool) string {
	cReturnType := "void"
	if returnType != "" && returnType != "void" {
		cReturnType = mapTypeToCType(returnType)
	}

	var paramList []string

	// Static methods don't have 'this' parameter
	if !isStatic {
		paramList = append(paramList, fmt.Sprintf("%s* this", className))
	}

	for _, param := range parameters {
		paramType := mapTypeToCType(param.Type)
		// For ref parameters, don't add extra * since they should be handled as single pointers
		if param.IsRef {
			// For ref parameters, ensure they are treated as single pointers
			if !strings.HasSuffix(paramType, "*") {
				paramType = paramType + "*"
			}
		} else {
			// For non-ref parameters, apply the normal rules
			if _, isPrimitive := primitiveTypes[param.Type]; !isPrimitive && param.Type != "string" {
				paramType = paramType + "*"
			} else if param.Type == "string" {
				paramType = "char*"
			}
		}
		paramList = append(paramList, fmt.Sprintf("%s %s", paramType, param.Name))
	}

	return fmt.Sprintf("%s %s_%s(%s)", cReturnType, className, methodName, strings.Join(paramList, ", "))
}

func generateFunctionPrototype(funcDecl *lexer.TopLevelFuncDeclStmt) string {
	returnType := "int"
	// Default to int (will return array length for list types)

	var paramList []string

	// Handle functions that return lists
	if strings.HasPrefix(funcDecl.ReturnType, "list[") && strings.HasSuffix(funcDecl.ReturnType, "]") {
		innerType := strings.TrimPrefix(strings.TrimSuffix(funcDecl.ReturnType, "]"), "list[")
		if innerType == "string" {
			paramList = append(paramList, "char _output_array[][256]")
		} else {
			cType := mapTypeToCType(innerType)
			paramList = append(paramList, fmt.Sprintf("%s _output_array[]", cType))
		}
		paramList = append(paramList, "int _max_size")
		returnType = "int"
	} else if funcDecl.ReturnType != "" && funcDecl.ReturnType != "void" {
		if funcDecl.ReturnType == "string" {
			returnType = "void"
			paramList = append(paramList, "char* _output_buffer")
		} else {
			returnType = mapTypeToCType(funcDecl.ReturnType)
		}
	} else {
		returnType = "void"
	}

	for _, param := range funcDecl.Parameters {
		paramType := mapTypeToCType(param.Type)
		paramName := param.Name

		if param.IsList || strings.HasPrefix(param.Type, "list[") {
			if param.Type == "string" {
				paramList = append(paramList, fmt.Sprintf("char %s[][256]", paramName))
			} else {
				paramList = append(paramList, fmt.Sprintf("%s %s[]", paramType, paramName))
			}
			paramList = append(paramList, fmt.Sprintf("int %s_len", paramName))
		} else {
			if param.Type == "string" {
				paramType = "char*"
			}
			if param.IsRef && !strings.HasSuffix(paramType, "*") {
				paramType = paramType + "*"
			}
			paramList = append(paramList, fmt.Sprintf("%s %s", paramType, paramName))
		}
	}
	if funcDecl.Name == "main" && len(funcDecl.Parameters) == 0 {
		return "int main(int argc, char** argv)"
	}

	return fmt.Sprintf("%s %s(%s)", returnType, funcDecl.Name, strings.Join(paramList, ", "))
}

func isEnumType(typeName string) bool {
	_, exists := globalEnums[typeName]
	return exists
}

func isCustomClassType(typeName string) bool {
	_, exists := globalClasses[typeName]
	return exists
}

func mapTypeToCType(mapType string) string {
	logger.Debug("mapTypeToCType called with: '%s'\n", mapType)

	// Handle ref types by stripping "ref " prefix and making it a pointer
	if strings.HasPrefix(mapType, "ref ") {
		baseType := strings.TrimPrefix(mapType, "ref ")
		cType := mapTypeToCType(baseType)
		// Don't double-add asterisk if already a pointer
		if strings.HasSuffix(cType, "*") {
			logger.Debug("ref type '%s' -> '%s' (already pointer)\n", mapType, cType)
			return cType
		}
		result := cType + "*"
		logger.Debug("ref type '%s' -> '%s'\n", mapType, result)
		return result
	}

	if isEnumType(mapType) {
		logger.Debug("enum type '%s' -> '%s'\n", mapType, mapType)
		return mapType
	}
	if isCustomClassType(mapType) {
		result := mapType + "*"
		logger.Debug("custom class type '%s' -> '%s'\n", mapType, result)
		return result
	}
	switch mapType {
	case "int", "i32", "i32*":
		return "int"
	case "float", "f32":
		return "float"
	case "double", "f64":
		return "double"
	case "char":
		return "char"
	case "string":
		return "char*"
	case "bool":
		return "bool"
	case "u16":
		return "unsigned short"
	case "u32":
		return "unsigned int"
	case "u64":
		return "unsigned long"
	case "i16":
		return "short"
	case "i64":
		return "long"
	default:
		if strings.HasPrefix(mapType, "list[") && strings.HasSuffix(mapType, "]") {
			innerType := extractListInnerType(mapType)
			if innerType == "" {
				return mapType
			}
			cInnerType := mapTypeToCType(innerType)
			if innerType == "string" {
				return "char"
			}
			return cInnerType
		}
		if strings.HasPrefix(mapType, "map[") && strings.HasSuffix(mapType, "]") {
			// Return void* to indicate complex map type
			return "void*"
		}
		return mapType
	}
}

// Extracts the inner type from a list type, handling nested brackets
func extractListInnerType(listType string) string {
	if !strings.HasPrefix(listType, "list[") || !strings.HasSuffix(listType, "]") {
		return ""
	}

	bracketDepth := 0
	start := strings.Index(listType, "[")
	if start == -1 {
		return ""
	}

	for i := start; i < len(listType); i++ {
		switch listType[i] {
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
			if bracketDepth == 0 {
				return listType[start+1 : i]
			}
		}
	}
	return ""
}

func isImportedType(typeName string, imports []*lexer.ImportStmt) (string, bool) {
	for _, imp := range imports {
		if module, exists := lexer.LoadedModules[imp.Module]; exists {
			if _, classExists := module.PublicClasses[typeName]; classExists {
				return imp.Module, true
			}
		}
	}
	return "", false
}

// Checks if a type is a complex collection (nested lists or maps)
func isComplexCollectionType(typeName string) bool {
	if strings.HasPrefix(typeName, "list[") {
		innerType := extractListInnerType(typeName)
		return strings.HasPrefix(innerType, "list[") || strings.HasPrefix(innerType, "map[")
	}
	if strings.HasPrefix(typeName, "map[") {
		return true
	}
	return false
}

// Handles rendering of complex nested collection types
func renderComplexListDecl(b *strings.Builder, listDecl *lexer.ListDeclStmt, indent, currentModule string) {
	listType := listDecl.Type
	listName := lexer.ResolveSymbol(listDecl.Name, currentModule)

	if strings.HasPrefix(listType, "list[list[") {
		innerType := extractListInnerType(listType)
		if innerType != "" {
			innerInnerType := extractListInnerType(innerType)
			if innerInnerType == "string" {
				fmt.Fprintf(b, "%schar %s[%d][100][256];\n", indent, listName, len(listDecl.Elements))
				fmt.Fprintf(b, "%sint %s_lengths[%d]; // Track length of each inner list\n", indent, listName, len(listDecl.Elements))
				for i, elem := range listDecl.Elements {
					if strings.HasPrefix(elem, "[") && strings.HasSuffix(elem, "]") {
						innerElements := parseListElements(elem[1 : len(elem)-1])
						fmt.Fprintf(b, "%s%s_lengths[%d] = %d;\n", indent, listName, i, len(innerElements))
						for j, innerElem := range innerElements {
							innerElem = strings.TrimSpace(innerElem)
							if strings.HasPrefix(innerElem, "\"") && strings.HasSuffix(innerElem, "\"") {
								innerElem = innerElem[1 : len(innerElem)-1]
							}
							fmt.Fprintf(b, "%sstrcpy(%s[%d][%d], \"%s\");\n", indent, listName, i, j, innerElem)
						}
					}
				}
				fmt.Fprintf(b, "%sint %s_len = %d;\n", indent, listName, len(listDecl.Elements))
			} else {
				cInnerType := mapTypeToCType(innerInnerType)
				fmt.Fprintf(b, "%s%s %s[%d][100];\n", indent, cInnerType, listName, len(listDecl.Elements))
				fmt.Fprintf(b, "%sint %s_lengths[%d];\n", indent, listName, len(listDecl.Elements))

				for i, elem := range listDecl.Elements {
					if strings.HasPrefix(elem, "[") && strings.HasSuffix(elem, "]") {
						innerElements := parseListElements(elem[1 : len(elem)-1])
						fmt.Fprintf(b, "%s%s_lengths[%d] = %d;\n", indent, listName, i, len(innerElements))
						for j, innerElem := range innerElements {
							innerElem = strings.TrimSpace(innerElem)
							fmt.Fprintf(b, "%s%s[%d][%d] = %s;\n", indent, listName, i, j, innerElem)
						}
					}
				}
				fmt.Fprintf(b, "%sint %s_len = %d;\n", indent, listName, len(listDecl.Elements))
			}
		}
	} else {
		// TODO: Complex types support.
		fmt.Fprintf(b, "%s// Complex type %s not fully implemented yet\n", indent, listType)
		fmt.Fprintf(b, "%svoid* %s; // Placeholder\n", indent, listName)
	}
}

func parseListElements(elementsStr string) []string {
	var elements []string
	var current strings.Builder
	inQuotes := false
	bracketDepth := 0

	for i, char := range elementsStr {
		switch char {
		case '"':
			if i == 0 || elementsStr[i-1] != '\\' {
				inQuotes = !inQuotes
			}
			current.WriteRune(char)
		case '[':
			if !inQuotes {
				bracketDepth++
			}
			current.WriteRune(char)
		case ']':
			if !inQuotes {
				bracketDepth--
			}
			current.WriteRune(char)
		case ',':
			if !inQuotes && bracketDepth == 0 {
				element := strings.TrimSpace(current.String())
				if element != "" {
					elements = append(elements, element)
				}
				current.Reset()
			} else {
				current.WriteRune(char)
			}
		default:
			current.WriteRune(char)
		}
	}

	element := strings.TrimSpace(current.String())
	if element != "" {
		elements = append(elements, element)
	}

	return elements
}
