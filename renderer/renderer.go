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
	"strconv"
	"strings"
	"unicode"

	"scar/lexer"
)

// EnumInfo stores information about an enum type
type EnumInfo struct {
	Name   string
	Values []string
}

var (
	globalClasses    = make(map[string]*ClassInfo)
	globalEnums      = make(map[string]*EnumInfo)
	globalObjects    = make(map[string]*ObjectInfo)
	globalFunctions  = make(map[string]*lexer.TopLevelFuncDeclStmt)
	globalArrays     = make(map[string]string)
	globalVars       = make(map[string]*lexer.PubVarDeclStmt)
	currentModule    = ""
	currentClassName = ""
	primitiveTypes   = map[string]string{
		"int":    "int",
		"float":  "float",
		"double": "double",
		"bool":   "bool",
		"char":   "char",
		"string": "char*",
		"u16":    "uint16_t",
		"u32":    "uint32_t",
		"u64":    "uint64_t",
		"i16":    "int16_t",
		"i32":    "int32_t",
		"i64":    "int64_t",
		"f32":    "float",
		"f64":    "double",
	}
)

func RenderC(program *lexer.Program, baseDir string) string {
	var b strings.Builder

	for _, importStmt := range program.Imports {
		_, err := lexer.LoadModule(importStmt.Module, baseDir)
		if err != nil {
			fmt.Printf("\033[31mFailed to load module '%s': %v\033[0m\n", importStmt.Module, err)
			os.Exit(1)
		}
	}

	for _, stmt := range program.Statements {
		if stmt.ClassDecl != nil {
			collectClassInfo(stmt.ClassDecl)
		}
		if stmt.PubVarDecl != nil {
			globalVars[stmt.PubVarDecl.Name] = stmt.PubVarDecl
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
		// Handle enum declarations
		if stmt.EnumDecl != nil {
			enumInfo := &EnumInfo{
				Name:   stmt.EnumDecl.Name,
				Values: stmt.EnumDecl.Values,
			}
			globalEnums[stmt.EnumDecl.Name] = enumInfo
		}
		// Handle public enum declarations
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
		b.WriteString(fmt.Sprintf("typedef enum {\n"))
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

int _exception = 0;

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
				if classDecl, exists := module.PublicClasses[className]; exists {
					constructor = classDecl.Constructor
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
		cType := mapTypeToCType(varDecl.Type)
		value := varDecl.Value

		if varDecl.Type == "string" {
			if !strings.HasPrefix(value, "\"") {
				value = fmt.Sprintf("\"%s\"", value)
			}
			fmt.Fprintf(&b, "%s %s[256];\n", cType, varName)
			fmt.Fprintf(&b, "void init_%s() { strcpy(%s, %s); }\n", varName, varName, value)
		} else {
			fmt.Fprintf(&b, "%s %s = %s;\n", cType, varName, value)
		}
	}
	b.WriteString("\n")
	for varName, varDecl := range globalVars {
		if varDecl.Type == "string" {
			fmt.Fprintf(&b, "    init_%s();\n", varName)
		}
	}
	for _, module := range lexer.LoadedModules {
		for varName, varDecl := range module.PublicVars {
			cType := mapTypeToCType(varDecl.Type)
			uniqueName := lexer.GenerateUniqueSymbol(varName, module.Name)
			if varDecl.Type == "string" {
				fmt.Fprintf(&b, "extern %s %s[256];\n", cType, uniqueName)
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
				fmt.Fprintf(&b, "%s %s[256];\n", cType, uniqueName)
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

	b.WriteString("int main() {\n")

	for _, module := range lexer.LoadedModules {
		for varName, varDecl := range module.PublicVars {
			if varDecl.Type == "string" {
				uniqueName := lexer.GenerateUniqueSymbol(varName, module.Name)
				fmt.Fprintf(&b, "    init_%s();\n", uniqueName)
			}
		}
	}

	var mainStatements []*lexer.Statement
	for _, stmt := range program.Statements {
		if stmt.ClassDecl == nil && stmt.PubClassDecl == nil && stmt.PubVarDecl == nil && stmt.TopLevelFuncDecl == nil && stmt.PubTopLevelFuncDecl == nil {
			mainStatements = append(mainStatements, stmt)
		}
	}

	renderStatements(&b, mainStatements, "    ", "", program)
	b.WriteString("    return 0;\n")
	b.WriteString("}\n")

	for _, stmt := range program.Statements {
		if stmt.PubTopLevelFuncDecl == nil && stmt.ClassDecl == nil && stmt.PubClassDecl == nil && stmt.PubVarDecl == nil && stmt.TopLevelFuncDecl == nil {
			mainStatements = append(mainStatements, stmt)
		}
	}
	return b.String()
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
					fmt.Printf("Debug: Field %s, Value %s, Inferred Type: %s, IsRef: %v\n", fieldName, stmt.VarAssign.Value, fieldType, isRef)
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
	funcName := strings.TrimSpace(value[:parenIndex])
	argsWithParens := value[parenIndex:]
	resolvedFuncName := lexer.ResolveSymbol(funcName, currentModule)
	return resolvedFuncName + argsWithParens
}

func inferTypeFromValue(value string) string {
	if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
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

	fmt.Fprintf(b, "    %s* this = malloc(sizeof(%s));\n", className, className)

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
					fmt.Fprintf(b, "    this->%s = 0;\n", field.Name)
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
				fieldName := strings.TrimPrefix(stmt.MapDecl.Name, "this.")
				mapSize := len(stmt.MapDecl.Pairs)

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
				fieldName := stmt.VarDecl.Name
				value := stmt.VarDecl.Value

				if fieldName == "this" {
					continue
				}

				fieldName = strings.TrimPrefix(fieldName, "this.")

				if stmt.VarDecl.IsRef {
					if value == "0" || value == "NULL" {
						fmt.Fprintf(b, "    this->%s = NULL;\n", fieldName)
					} else {
						fmt.Fprintf(b, "    this->%s = %s;\n", fieldName, value)
					}
				} else if stmt.VarDecl.Type == "string" {
					isStringField := stmt.VarDecl.Type == "string"

					fmt.Printf("Debug: VarDecl field %s, value %s, isStringField %v\n", fieldName, value, isStringField)

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

				fmt.Printf("Debug: VarAssign field %s, value %s, isStringField %v\n", fieldName, value, isStringField)

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
				renderStatements(b, []*lexer.Statement{stmt}, "    ", className, program)
			}
		}
	}

	b.WriteString("    return this;\n}\n\n")

	// Generate instance map access helper functions after constructor
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
		prototype := generateMethodPrototype(className, method.Name, returnType, method.Parameters)
		b.WriteString(prototype)
		b.WriteString(";\n")
	}
	b.WriteString("\n")

	for _, method := range classDecl.Methods {
		returnType := "void"
		if method.ReturnType != "" && method.ReturnType != "void" {
			returnType = mapTypeToCType(method.ReturnType)
		}

		fmt.Fprintf(b, "%s %s_%s(%s* this", returnType, className, method.Name, className)

		for _, param := range method.Parameters {
			paramType := mapTypeToCType(param.Type)
			if _, isPrimitive := primitiveTypes[param.Type]; !isPrimitive && param.Type != "string" {
				paramType = paramType + "*"
			} else if param.Type == "string" {
				paramType = "char*"
			}
			fmt.Fprintf(b, ", %s %s", paramType, param.Name)
		}

		b.WriteString(") {\n")
		renderStatements(b, method.Body, "    ", className, program)
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

// New helper function for instance map put:
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

func renderStatements(b *strings.Builder, stmts []*lexer.Statement, indent string, className string, program *lexer.Program) {
	if className != "" {
		fmt.Printf("Debug: renderStatements - className: '%s'\n", className)
		currentClassName = className
	}
	for _, stmt := range stmts {
		switch {
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
						args[i] = resolvedVar
					}
				}
				argsStr := strings.Join(args, ", ")
				escapedFormat := strings.ReplaceAll(stmt.Put.Format, "\"", "\\\"")
				fmt.Fprintf(b, "%sprintf(\"%s\", %s);\n", indent, escapedFormat, argsStr)
			} else if stmt.Put.Put != "" {
				fmt.Fprintf(b, "%sprintf(\"%s\");\n", indent, stmt.Put.Put)
			}

		case stmt.CatList != nil:
			if stmt.CatList.Target != "" {
				targetVar := lexer.ResolveSymbol(stmt.CatList.Target, currentModule)
				// Determine the array type from the first list
				// lexer.ResolveSymbol(stmt.CatList.Lists[0], currentModule)
				listType := "int"
				for listName, arrayType := range globalArrays {
					if listName == stmt.CatList.Lists[0] {
						listType = arrayType
						break
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
				globalArrays[stmt.CatList.Target] = listType

			} else {
				if len(stmt.CatList.Lists) < 2 {
					fmt.Fprintf(b, "%s// Error: catlist! requires at least 2 lists\n", indent)
					break
				}

				targetList := lexer.ResolveSymbol(stmt.CatList.Lists[0], currentModule)
				listType := "int"

				// TODO: Maybe change the default later?
				// Can be done after implementing generics.

				for listName, arrayType := range globalArrays {
					if listName == stmt.CatList.Lists[0] {
						listType = arrayType
						break
					}
				}

				for i := 1; i < len(stmt.CatList.Lists); i++ {
					sourceList := lexer.ResolveSymbol(stmt.CatList.Lists[i], currentModule)

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
		case stmt.Print != nil:
			if stmt.Print.Format != "" && len(stmt.Print.Variables) > 0 {
				var (
					variables = reconstructMethodCalls(stmt.Print.Variables)
					args      = make([]string, len(variables))
				)
				for i, v := range variables {
					if strings.HasPrefix(v, "get!") {
						getArgs := v[4:]
						getArgs = strings.TrimSpace(getArgs[1 : len(getArgs)-1])
						parts := strings.SplitN(getArgs, ",", 2)
						if len(parts) == 2 {
							mapName := strings.TrimSpace(parts[0])
							key := strings.TrimSpace(parts[1])
							args[i] = renderMapAccess(mapName, key, program)
						} else {
							args[i] = v
						}
					} else if isMethodCall(v) {
						args[i] = convertMethodCallToC(v)
					} else {
						resolvedVar := lexer.ResolveSymbol(v, currentModule)
						resolvedVar = convertThisReferencesGranular(resolvedVar)
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
				if isMethodCall(value) {
					value = convertMethodCallToC(value)
				} else if strings.HasPrefix(value, "this.") {
					value = "this->" + value[5:]
				} else {
					value = lexer.ResolveSymbol(value, currentModule)
					value = convertThisReferencesGranular(value)
				}

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

				fmt.Fprintf(b, "%sreturn %s;\n", indent, value)
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
			renderStatements(b, stmt.TryCatch.TryBody, indent+"    ", className, program)
			fmt.Fprintf(b, "%s    if (_exception != 0) {\n", indent)
			fmt.Fprintf(b, "%scatch_label:\n", indent)
			renderStatements(b, stmt.TryCatch.CatchBody, indent+"    ", className, program)
			fmt.Fprintf(b, "%s    }\n", indent)
			fmt.Fprintf(b, "%s    _exception = _prev_exception;\n", indent)
			fmt.Fprintf(b, "%s}\n", indent)
		case stmt.While != nil:
			condition := lexer.ResolveSymbol(stmt.While.Condition, currentModule)
			condition = convertThisReferencesGranular(condition)
			fmt.Fprintf(b, "%swhile (%s) {\n", indent, condition)
			renderStatements(b, stmt.While.Body, indent+"    ", className, program)
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
				// This should have been caught in fucking parsing, but just in case
				fmt.Fprintf(b, "%s// Error: invalid foreach collection format\n", indent)
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
			} else { // values
				if stmt.Foreach.VarType == "string" {
					fmt.Fprintf(b, "%s    %s* %s = %s_values[__i];\n", indent, varType, varName, resolvedMapName)
				} else {
					fmt.Fprintf(b, "%s    %s %s = %s_values[__i];\n", indent, varType, varName, resolvedMapName)
				}
			}

			// Render the body
			renderStatements(b, stmt.Foreach.Body, indent+"    ", className, program)

			fmt.Fprintf(b, "%s}\n", indent)
		case stmt.For != nil:
			var (
				varName = stmt.For.Var
				start   = lexer.ResolveSymbol(stmt.For.Start, currentModule)
				end     = stmt.For.End
			)
			end = convertThisReferencesGranular(end)
			end = lexer.ResolveSymbol(end, currentModule)

			// Clean up variable name - remove type prefix if present
			if after, ok := strings.CutPrefix(varName, "int "); ok {
				varName = after
			}

			// Clean up any malformed characters that might have been introduced
			varName = strings.ReplaceAll(varName, "*", "")
			varName = strings.ReplaceAll(varName, "_", "")

			endCond := end
			if strings.ContainsAny(end, "+-*/><=!&|^%(") {
				endCond = fmt.Sprintf("(%s)", end)
			}

			fmt.Fprintf(b, "%sfor (int %s = %s; %s <= %s; %s++) {\n",
				indent, varName, start, varName, endCond, varName)
			renderStatements(b, stmt.For.Body, indent+"    ", className, program)
			fmt.Fprintf(b, "%s}\n", indent)
		case stmt.If != nil:
			condition := stmt.If.Condition
			if isMethodCall(condition) {
				condition = convertMethodCallToC(condition)
			} else {
				condition = lexer.ResolveSymbol(condition, currentModule)
				condition = convertThisReferencesGranular(condition)
			}
			condition = resolveImportedSymbols(condition, program.Imports)
			fmt.Fprintf(b, "%sif (%s) {\n", indent, condition)
			renderStatements(b, stmt.If.Body, indent+"    ", className, program)
			fmt.Fprintf(b, "%s}\n", indent)

			for _, elif := range stmt.If.ElseIfs {
				elifCondition := elif.Condition
				if isMethodCall(elifCondition) {
					elifCondition = convertMethodCallToC(elifCondition)
				} else {
					elifCondition = lexer.ResolveSymbol(elifCondition, currentModule)
					elifCondition = convertThisReferencesGranular(elifCondition)
				}
				elifCondition = resolveImportedSymbols(elifCondition, program.Imports)
				fmt.Fprintf(b, "%selse if (%s) {\n", indent, elifCondition)
				renderStatements(b, elif.Body, indent+"    ", className, program)
				fmt.Fprintf(b, "%s}\n", indent)
			}

			if stmt.If.Else != nil {
				fmt.Fprintf(b, "%selse {\n", indent)
				renderStatements(b, stmt.If.Else.Body, indent+"    ", className, program)
				fmt.Fprintf(b, "%s}\n", indent)
			}
		case stmt.VarDecl != nil:
			var (
				varType = stmt.VarDecl.Type
				varName = lexer.ResolveSymbol(stmt.VarDecl.Name, currentModule)
				value   = stmt.VarDecl.Value
			)

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
				return
			}

			if isMethodCall(value) {
				value = convertMethodCallToC(value)
			} else {
				value = convertThisReferencesGranular(value)
			}

			value = processGetExpressions(value, program)
			value = fixFloatCastGranular(value)
			value = resolveImportedSymbols(value, program.Imports)

			fmt.Printf("Debug: VarDecl var %s, value %s, type %s\n", varName, value, stmt.VarDecl.Type)

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
					fmt.Fprintf(b, "%s%s %s = %s;\n", indent, varType, varName, value)
				}
			}
		case stmt.VarAssign != nil:
			varName := lexer.ResolveSymbol(stmt.VarAssign.Name, currentModule)
			value := stmt.VarAssign.Value
			value = fixFloatCastGranular(value)
			value = convertThisReferencesGranular(value)
			if strings.HasPrefix(varName, "this.") {
				varName = "this->" + varName[5:]
			} else if strings.Contains(varName, ".") {
				re := regexp.MustCompile(`([a-zA-Z_][a-zA-Z0-9_]*)\.([a-zA-Z_][a-zA-Z0-9_]*)`)
				varName = re.ReplaceAllString(varName, "$1->$2")
			}

			if strings.Contains(varName, "[") && strings.Contains(varName, "]") {
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
				for _, classInfo := range globalClasses {
					for _, field := range classInfo.Fields {
						if field.Name == varName || ("this->"+field.Name) == varName {
							varType = field.Type
							break
						}
					}
				}
				if varType == "string" {
					if isFunctionCall(value) {
						value = resolveFunctionCall(value)
						fmt.Fprintf(b, "%sstrcpy(%s, %s);\n", indent, varName, value)
					} else {
						// Only quote the value if it's a literal string (starts and ends with quotes)
						// and not a variable/parameter name or expression
						if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
							fmt.Fprintf(b, "%sstrcpy(%s, %s);\n", indent, varName, value)
						} else if isNumericOrBoolean(value) || isValidIdentifier(value) {
							fmt.Fprintf(b, "%sstrcpy(%s, %s);\n", indent, varName, value)
						} else {
							fmt.Fprintf(b, "%sstrcpy(%s, \"%s\");\n", indent, varName, value)
						}
					}
				} else {
					if isFunctionCall(value) {
						value = resolveFunctionCall(value)
					}
					fmt.Fprintf(b, "%s%s = %s;\n", indent, varName, value)
				}
			}
		case stmt.ListDecl != nil:
			listType := mapTypeToCType(stmt.ListDecl.Type)
			listName := lexer.ResolveSymbol(stmt.ListDecl.Name, currentModule)
			globalArrays[stmt.ListDecl.Name] = stmt.ListDecl.Type
			if stmt.ListDecl.Type == "string" {
				fmt.Fprintf(b, "%s%s %s[%d][256];\n", indent, "char", listName, len(stmt.ListDecl.Elements))
			} else {
				fmt.Fprintf(b, "%s%s %s[%d];\n", indent, listType, listName, len(stmt.ListDecl.Elements))
			}
			for i, elem := range stmt.ListDecl.Elements {
				elem = lexer.ResolveSymbol(elem, currentModule)
				if stmt.ListDecl.Type == "string" {
					if !strings.HasPrefix(elem, "\"") && !strings.HasSuffix(elem, "\"") {
						elem = fmt.Sprintf("\"%s\"", elem)
					}
					fmt.Fprintf(b, "%sstrcpy(%s[%d], %s);\n", indent, listName, i, elem)
				} else {
					fmt.Fprintf(b, "%s%s[%d] = %s;\n", indent, listName, i, elem)
				}
			}
			fmt.Fprintf(b, "%sint %s_len = %d;\n", indent, listName, len(stmt.ListDecl.Elements))
		case stmt.ObjectDecl != nil:
			varName := lexer.ResolveSymbol(stmt.ObjectDecl.Name, currentModule)
			typeName := stmt.ObjectDecl.Type
			args := stmt.ObjectDecl.Args
			resolvedType := typeName

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
				fmt.Fprintf(b, "%s%s %s = %s_%s(%s);\n", indent, varType, varName, resolvedClassName, methodName, objectName)
			} else {
				fmt.Fprintf(b, "%s%s %s = %s_%s(%s, %s);\n", indent, varType, varName, resolvedClassName, methodName, objectName, argsStr)
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
			if varType == "string" {
				fmt.Fprintf(b, "%s%s %s[256];\n", indent, cType, varName)
				if value != "" {
					if !strings.HasPrefix(value, "\"") && !strings.HasSuffix(value, "\"") {
						value = fmt.Sprintf("\"%s\"", value)
					}
					fmt.Fprintf(b, "%sstrcpy(%s, %s);\n", indent, varName, value)
				}
			} else {
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
			fmt.Fprintf(b, "%s%s = malloc(size + 1);\n", indent+"    ", varName)
			fmt.Fprintf(b, "%sfread(%s, 1, size, %s);\n", indent+"    ", varName, fpVarName)
			fmt.Fprintf(b, "%s%s[size] = '\\0';\n", indent+"    ", varName)
			fmt.Fprintf(b, "%sfclose(%s);\n", indent+"    ", fpVarName)
			fmt.Fprintf(b, "%s}\n", indent)
		case stmt.VarDeclWrite != nil:
			var (
				content   = lexer.ResolveSymbol(stmt.VarDeclWrite.Content, currentModule)
				filePath  = fmt.Sprintf("\"%s\"", stmt.VarDeclWrite.FilePath)
				mode      = stmt.VarDeclWrite.Mode
				fpVarName = fmt.Sprintf("fp_write_%d", len(stmt.VarDeclWrite.FilePath))
				fileMode  string
			)
			if mode == "append!" {
				fileMode = "\"a\""
			} else {
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
			objectName := stmt.MethodCall.Object
			methodName := stmt.MethodCall.Method
			args := make([]string, len(stmt.MethodCall.Args))
			for i, arg := range stmt.MethodCall.Args {
				args[i] = lexer.ResolveSymbol(arg, currentModule)
			}
			argsStr := strings.Join(args, ", ")

			fmt.Printf("Debug: Method call - object: '%s', method: '%s', args: %v\n", objectName, methodName, stmt.MethodCall.Args)
			fmt.Printf("Debug: Current class name: '%s'\n", className)

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
					resolvedClassName = "unknown"
				}
				if argsStr == "" {
					fmt.Fprintf(b, "%s%s_%s(%s);\n", indent, resolvedClassName, methodName, objectName)
				} else {
					fmt.Fprintf(b, "%s%s_%s(%s, %s);\n", indent, resolvedClassName, methodName, objectName, argsStr)
				}
			}
		case stmt.FunctionCall != nil:
			funcName := lexer.ResolveSymbol(stmt.FunctionCall.Name, currentModule)
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
			mapName := lexer.ResolveSymbol(stmt.MapDecl.Name, currentModule)
			keyType := stmt.MapDecl.KeyType
			valueType := stmt.MapDecl.ValueType
			cKeyType := mapTypeToCType(keyType)
			cValueType := mapTypeToCType(valueType)
			mapSize := len(stmt.MapDecl.Pairs)
			initialSize := mapSize
			if initialSize == 0 {
				initialSize = 10
			}

			if keyType == "string" {
				fmt.Fprintf(b, "%s %s_keys[%d][256];\n", cKeyType, mapName, initialSize)
			} else {
				fmt.Fprintf(b, "%s %s_keys[%d];\n", cKeyType, mapName, initialSize)
			}

			if valueType == "string" {
				fmt.Fprintf(b, "%s %s_values[%d][256];\n", cValueType, mapName, initialSize)
			} else {
				fmt.Fprintf(b, "%s %s_values[%d];\n", cValueType, mapName, initialSize)
			}

			fmt.Fprintf(b, "%sint %s_size = %d;\n", indent, mapName, mapSize)
			generateMapAccessHelper(b, mapName, keyType, valueType)

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

			// Handle key assignment based on type
			if keyType == "string" {
				if strings.HasPrefix(key, "\"") && strings.HasSuffix(key, "\"") {
					// String literal key - copy directly
					keyStr := key[1 : len(key)-1]                     // Remove quotes
					keyStr = strings.ReplaceAll(keyStr, "\"", "\\\"") // Escape quotes in the string
					fmt.Fprintf(b, "%s        strcpy(%s_keys[%s_size], \"%s\");\n", indent, mapName, mapName, keyStr)
				} else {
					// Variable key - resolve symbol first
					resolvedKey := lexer.ResolveSymbol(key, currentModule)
					fmt.Fprintf(b, "%s        strcpy(%s_keys[%s_size], %s);\n", indent, mapName, mapName, resolvedKey)
				}
			} else {
				// Non-string key - direct assignment
				fmt.Fprintf(b, "%s        %s_keys[%s_size] = %s;\n", indent, mapName, mapName, key)
			}

			// Handle value assignment based on type
			if valueType == "string" {
				if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
					// String literal value - copy directly
					fmt.Fprintf(b, "%s        strcpy(%s_values[%s_size], %s);\n", indent, mapName, mapName, value)
				} else {
					// Variable value - resolve symbol first
					resolvedValue := lexer.ResolveSymbol(value, currentModule)
					fmt.Fprintf(b, "%s        strcpy(%s_values[%s_size], %s);\n", indent, mapName, mapName, resolvedValue)
				}
			} else {
				// Non-string value - direct assignment
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
			fmt.Fprintf(b, "%s#pragma omp parallel for\n", indent)
			fmt.Fprintf(b, "%sfor (int %s = %s; %s <= %s; %s++) {\n", indent, varName, start, varName, end, varName)
			renderStatements(b, stmt.ParallelFor.Body, indent+"    ", className, program)
			fmt.Fprintf(b, "%s}\n", indent)
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

	expr = strings.Join(strings.Fields(expr), " ")
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
	expr = strings.ReplaceAll(expr, "-> ", "->")
	expr = strings.ReplaceAll(expr, " ->", "->")
	expr = strings.ReplaceAll(expr, "this ->", "this->")

	// Handle nil conversion
	expr = strings.ReplaceAll(expr, " nil", " NULL")
	expr = strings.ReplaceAll(expr, "nil ", "NULL ")
	expr = strings.ReplaceAll(expr, "(nil)", "(NULL)")
	if expr == "nil" {
		expr = "NULL"
	}

	if isMethodCall(expr) {
		expr = convertMethodCallToC(expr)
	}

	for i, lit := range stringLiterals {
		expr = strings.Replace(expr, fmt.Sprintf("__STRING_LITERAL_%d__", i), lit, 1)
	}

	return expr
}

// Processes all get! expressions in a string and replaces them with the appropriate C code
func processGetExpressions(expr string, program *lexer.Program) string {
	re := regexp.MustCompile(`get!\s*\(([^)]+)\)`)
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
				cCode   = renderMapAccess(mapName, key, program)
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

// Converts method calls in the format 'this.method(args)' to 'ClassName_method(this, args)'
func convertMethodCallToC(expr string) string {
	comparisonOps := []string{"==", "!=", ">", "<", ">=", "<="}
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
	return convertSingleMethodCall(expr)
}

// Generates a function for map access
func generateMapAccessHelper(b *strings.Builder, mapName, keyType, valueType string) {
	helperName := fmt.Sprintf("__get_%s_value", mapName)

	// Handle key type conversion
	var cKeyType string
	switch keyType {
	case "string":
		cKeyType = "char*"
	case "char":
		cKeyType = "char"
	default:
		cKeyType = mapTypeToCType(keyType)
	}

	// Handle value type conversion - for return type
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

	// Key comparison
	if keyType == "string" {
		// String key comparison
		fmt.Fprintf(b, "        if (strcmp(%s_keys[i], key) == 0) {\n", mapName)
	} else {
		// Numeric or char key comparison
		fmt.Fprintf(b, "        if (%s_keys[i] == key) {\n", mapName)
	}

	// Return value - for string values, x_values[i] is already char*
	fmt.Fprintf(b, "            return %s_values[i];\n", mapName)

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

// Generates a C expression for accessing a map value by key
func renderMapAccess(mapName, key string, program *lexer.Program) string {
	if strings.HasPrefix(mapName, "this.") {
		fieldName := strings.TrimPrefix(mapName, "this.")
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

	// Generate the key expression
	var keyExpr string
	if keyType == "string" || keyType == "char*" {
		// String key - handle string literals and variables
		if strings.HasPrefix(key, "\"") && strings.HasSuffix(key, "\"") {
			// String literal key
			keyStr := key[1 : len(key)-1]                     // Remove quotes
			keyStr = strings.ReplaceAll(keyStr, "\"", "\\\"") // Escape quotes
			keyExpr = fmt.Sprintf(`"%s"`, keyStr)
		} else {
			// Variable key - resolve symbol
			keyExpr = lexer.ResolveSymbol(key, currentModule)
			// If it's not a string literal and not a variable (could be a character literal)
			if keyExpr == key && len(key) == 3 && key[0] == '\'' && key[2] == '\'' {
				keyExpr = fmt.Sprintf("'%c'", key[1])
			}
		}
	} else {
		// Non-string key - resolve symbol if it's an identifier
		if !unicode.IsDigit(rune(key[0])) && !(key[0] == '-' && len(key) > 1 && unicode.IsDigit(rune(key[1]))) {
			key = lexer.ResolveSymbol(key, currentModule)
		}
		keyExpr = key

		// Handle character literals for char keys
		if keyType == "char" && len(keyExpr) == 3 && keyExpr[0] == '\'' && keyExpr[2] == '\'' {
			keyExpr = fmt.Sprintf("'%c'", keyExpr[1])
		}
	}

	return fmt.Sprintf("%s(%s)", helperName, keyExpr)
}

func convertSingleMethodCall(expr string) string {
	if strings.Contains(expr, "this.") {
		startIdx := strings.Index(expr, "this.")
		if startIdx == -1 {
			return expr
		}
		dotIndex := startIdx + 4
		if dotIndex >= len(expr) || expr[dotIndex] != '.' {
			return expr
		}
		parenIndex := strings.Index(expr[dotIndex:], "(")
		if parenIndex == -1 {
			fieldName := expr[dotIndex+1:]
			return fmt.Sprintf("this->%s", fieldName)
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
			return fmt.Sprintf("%s_%s(this)", className, methodName)
		}
		return fmt.Sprintf("%s_%s(this, %s)", className, methodName, args)
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

	if resolvedClassName == "" {
		return expr
	}

	resolvedObjectName := lexer.ResolveSymbol(objectName, currentModule)

	if args == "" {
		return fmt.Sprintf("%s_%s(%s)", resolvedClassName, methodName, resolvedObjectName)
	}
	return fmt.Sprintf("%s_%s(%s, %s)", resolvedClassName, methodName, resolvedObjectName, args)
}

func generateTopLevelFunctionImplementation(b *strings.Builder, funcDecl *lexer.TopLevelFuncDeclStmt, program *lexer.Program) {
	returnType := "void"
	if funcDecl.ReturnType != "" && funcDecl.ReturnType != "void" {
		if funcDecl.ReturnType == "string" {
			returnType = "void"
		} else {
			returnType = mapTypeToCType(funcDecl.ReturnType)
		}
	}

	fmt.Fprintf(b, "%s %s(", returnType, funcDecl.Name)

	paramList := make([]string, 0)
	if funcDecl.ReturnType == "string" {
		paramList = append(paramList, "char* _output_buffer")
	}

	for _, param := range funcDecl.Parameters {
		var (
			paramType = mapTypeToCType(param.Type)
			paramName = param.Name
		)
		if param.IsList {
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
					fmt.Fprintf(b, "    strcpy(_output_buffer, %s);\n", value)
					fmt.Fprintf(b, "    return;\n")
				}
			} else {
				renderStatements(b, []*lexer.Statement{stmt}, "    ", "", program)
			}
		}
	} else {
		renderStatements(b, funcDecl.Body, "    ", "", program)
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

// Generates a C function prototype for a class method
func generateMethodPrototype(className, methodName, returnType string, parameters []*lexer.MethodParameter) string {
	cReturnType := "void"
	if returnType != "" && returnType != "void" {
		cReturnType = mapTypeToCType(returnType)
	}
	paramList := []string{fmt.Sprintf("%s* this", className)}
	for _, param := range parameters {
		paramType := mapTypeToCType(param.Type)
		if _, isPrimitive := primitiveTypes[param.Type]; !isPrimitive && param.Type != "string" {
			paramType = paramType + "*"
		} else if param.Type == "string" {
			paramType = "char*"
		}
		paramList = append(paramList, fmt.Sprintf("%s %s", paramType, param.Name))
	}

	return fmt.Sprintf("%s %s_%s(%s)", cReturnType, className, methodName, strings.Join(paramList, ", "))
}

func generateFunctionPrototype(funcDecl *lexer.TopLevelFuncDeclStmt) string {
	returnType := "void"
	if funcDecl.ReturnType != "" && funcDecl.ReturnType != "void" {
		if funcDecl.ReturnType == "string" {
			returnType = "void"
		} else {
			returnType = mapTypeToCType(funcDecl.ReturnType)
		}
	}

	var paramList []string
	if funcDecl.ReturnType == "string" {
		paramList = append(paramList, "char* _output_buffer")
	}

	for _, param := range funcDecl.Parameters {
		paramType := mapTypeToCType(param.Type)
		paramName := param.Name

		if param.IsList {
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

func mapTypeToCType(mapType string) string {
	if isEnumType(mapType) {
		return mapType
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
		return "char"
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
			innerType := strings.TrimPrefix(strings.TrimSuffix(mapType, "]"), "list[")
			cInnerType := mapTypeToCType(innerType)
			if innerType == "string" {
				return "char"
			}
			return cInnerType
		}
		return mapType
	}
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
