package renderer

import (
	"fmt"
	"scar/lexer"
	"strings"
)

type MethodInfo struct {
	Name       string
	Parameters []string
	ReturnType string
}

type FieldInfo struct {
	Name string
	Type string
}

type ClassInfo struct {
	Name    string
	Fields  []FieldInfo
	Methods []MethodInfo
}

type ObjectInfo struct {
	Name string
	Type string
}

var globalClasses = make(map[string]*ClassInfo)
var globalObjects = make(map[string]*ObjectInfo)
var globalFunctions = make(map[string]*lexer.TopLevelFuncDeclStmt)
var currentModule = ""

func RenderC(program *lexer.Program, baseDir string) string {
	var b strings.Builder

	for _, importStmt := range program.Imports {
		_, err := lexer.LoadModule(importStmt.Module, baseDir)
		if err != nil {
			fmt.Printf("Warning: Failed to load module '%s': %v\n", importStmt.Module, err)
		}
	}

	for _, stmt := range program.Statements {
		if stmt.ClassDecl != nil {
			collectClassInfo(stmt.ClassDecl)
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
	}

	for _, module := range lexer.LoadedModules {
		for _, classDecl := range module.PublicClasses {
			collectClassInfoWithModule(classDecl, module.Name)
		}
	}

	b.WriteString(`#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

`)

	for className, classInfo := range globalClasses {
		generateStructDefinition(&b, classInfo, className)
		b.WriteString("\n")
	}

	for className := range globalClasses {
		fmt.Fprintf(&b, "%s* %s_new();\n", className, className)
		b.WriteString("\n")
	}

	for _, funcDecl := range globalFunctions {
		returnType := "void"
		if funcDecl.ReturnType != "" && funcDecl.ReturnType != "void" {
			returnType = mapTypeToCType(funcDecl.ReturnType)
		}

		fmt.Fprintf(&b, "%s %s(", returnType, funcDecl.Name)

		for i, param := range funcDecl.Parameters {
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
	}
	b.WriteString("\n")

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
			cType := mapTypeToCType(varDecl.Type)
			uniqueName := lexer.GenerateUniqueSymbol(varName, module.Name)
			value := varDecl.Value

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
			generateClassImplementation(&b, stmt.ClassDecl, "")
		}
		if stmt.PubClassDecl != nil {
			classDecl := &lexer.ClassDeclStmt{
				Name:        stmt.PubClassDecl.Name,
				Constructor: stmt.PubClassDecl.Constructor,
				Methods:     stmt.PubClassDecl.Methods,
			}
			generateClassImplementation(&b, classDecl, "")
		}
	}

	for _, module := range lexer.LoadedModules {
		for _, classDecl := range module.PublicClasses {
			generateClassImplementation(&b, classDecl, module.Name)
		}
	}

	for _, funcDecl := range globalFunctions {
		generateTopLevelFunctionImplementation(&b, funcDecl)
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
		if stmt.ClassDecl == nil && stmt.PubClassDecl == nil && stmt.PubVarDecl == nil {
			mainStatements = append(mainStatements, stmt)
		}
	}

	renderStatements(&b, mainStatements, "    ", "")
	b.WriteString("    return 0;\n")
	b.WriteString("}\n")
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
		for _, field := range classDecl.Constructor.Fields {
			if field.VarDecl != nil {
				fieldName := field.VarDecl.Name
				if strings.HasPrefix(fieldName, "this.") {
					fieldName = fieldName[5:]
					fieldInfo := FieldInfo{
						Name: fieldName,
						Type: field.VarDecl.Type,
					}
					classInfo.Fields = append(classInfo.Fields, fieldInfo)
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

func generateStructDefinition(b *strings.Builder, classInfo *ClassInfo, structName string) {
	fmt.Fprintf(b, "typedef struct %s {\n", structName)
	for _, field := range classInfo.Fields {
		cType := mapTypeToCType(field.Type)
		if field.Type == "string" {
			fmt.Fprintf(b, "    %s %s[256];\n", cType, field.Name)
		} else {
			fmt.Fprintf(b, "    %s %s;\n", cType, field.Name)
		}
	}
	fmt.Fprintf(b, "} %s;\n", structName)
}

func generateClassImplementation(b *strings.Builder, classDecl *lexer.ClassDeclStmt, moduleName string) {
	className := classDecl.Name
	if moduleName != "" {
		className = lexer.GenerateUniqueSymbol(classDecl.Name, moduleName)
	}

	fmt.Fprintf(b, "%s* %s_new() {\n", className, className)
	fmt.Fprintf(b, "    %s* obj = malloc(sizeof(%s));\n", className, className)

	if classDecl.Constructor != nil {
		for _, field := range classDecl.Constructor.Fields {
			if field.VarDecl != nil {
				fieldName := field.VarDecl.Name
				if strings.HasPrefix(fieldName, "this.") {
					fieldName = fieldName[5:] // Remove "this."
					fieldType := field.VarDecl.Type
					value := field.VarDecl.Value

					if fieldType == "string" {
						if !strings.HasPrefix(value, "\"") {
							value = fmt.Sprintf("\"%s\"", value)
						}
						fmt.Fprintf(b, "    strcpy(obj->%s, %s);\n", fieldName, value)
					} else {
						fmt.Fprintf(b, "    obj->%s = %s;\n", fieldName, value)
					}
				}
			}
		}
	}

	b.WriteString("    return obj;\n")
	b.WriteString("}\n\n")

	for _, method := range classDecl.Methods {
		returnType := "void"
		if method.ReturnType != "" && method.ReturnType != "void" {
			returnType = mapTypeToCType(method.ReturnType)
		}

		fmt.Fprintf(b, "%s %s_%s(%s* this", returnType, className, method.Name, className)

		for _, param := range method.Parameters {
			paramType := mapTypeToCType(param.Type)
			if param.Type == "string" {
				paramType = "char*"
			}
			fmt.Fprintf(b, ", %s %s", paramType, param.Name)
		}

		b.WriteString(") {\n")
		renderStatements(b, method.Body, "    ", classDecl.Name)
		b.WriteString("}\n\n")
	}
}

func renderStatements(b *strings.Builder, stmts []*lexer.Statement, indent string, className string) {
	for _, stmt := range stmts {
		switch {
		case stmt.Print != nil:
			if stmt.Print.Format != "" && len(stmt.Print.Variables) > 0 {
				args := make([]string, len(stmt.Print.Variables))
				for i, v := range stmt.Print.Variables {
					resolvedVar := lexer.ResolveSymbol(v, currentModule)
					if strings.HasPrefix(v, "this.") {
						fieldName := v[5:]
						args[i] = fmt.Sprintf("this->%s", fieldName)
					} else if strings.Contains(v, "[") && strings.Contains(v, "]") {
						args[i] = resolvedVar
					} else {
						args[i] = resolvedVar
					}
				}
				argsStr := strings.Join(args, ", ")
				escapedFormat := strings.ReplaceAll(stmt.Print.Format, "\"", "\\\"")
				b.WriteString(fmt.Sprintf("%sprintf(\"%s\\n\", %s);\n", indent, escapedFormat, argsStr))
			} else {
				fmt.Fprintf(b, "%sprintf(\"%s\\n\");\n", indent, stmt.Print.Print)
			}
		case stmt.Sleep != nil:
			fmt.Fprintf(b, "%ssleep(%s);\n", indent, stmt.Sleep.Duration)
		case stmt.Break != nil:
			fmt.Fprintf(b, "%sbreak;\n", indent)
		case stmt.Return != nil:
			value := stmt.Return.Value
			if strings.HasPrefix(value, "this.") {
				value = "this->" + value[5:]
			} else {
				value = lexer.ResolveSymbol(value, currentModule)
			}
			fmt.Fprintf(b, "%sreturn %s;\n", indent, value)

		case stmt.While != nil:
			condition := lexer.ResolveSymbol(stmt.While.Condition, currentModule)
			fmt.Fprintf(b, "%swhile (%s) {\n", indent, condition)
			renderStatements(b, stmt.While.Body, indent+"    ", className)
			fmt.Fprintf(b, "%s}\n", indent)
		case stmt.For != nil:
			varName := stmt.For.Var
			start := lexer.ResolveSymbol(stmt.For.Start, currentModule)
			end := lexer.ResolveSymbol(stmt.For.End, currentModule)
			fmt.Fprintf(b, "%sfor (int %s = %s; %s <= %s; %s++) {\n", indent, varName, start, varName, end, varName)
			renderStatements(b, stmt.For.Body, indent+"    ", className)
			fmt.Fprintf(b, "%s}\n", indent)
		case stmt.If != nil:
			condition := lexer.ResolveSymbol(stmt.If.Condition, currentModule)
			fmt.Fprintf(b, "%sif (%s) {\n", indent, condition)
			renderStatements(b, stmt.If.Body, indent+"    ", className)

			for _, elif := range stmt.If.ElseIfs {
				elifCondition := lexer.ResolveSymbol(elif.Condition, currentModule)
				fmt.Fprintf(b, "%s} else if (%s) {\n", indent, elifCondition)
				renderStatements(b, elif.Body, indent+"    ", className)
			}

			if stmt.If.Else != nil {
				fmt.Fprintf(b, "%s} else {\n", indent)
				renderStatements(b, stmt.If.Else.Body, indent+"    ", className)
			}

			fmt.Fprintf(b, "%s}\n", indent)
		case stmt.VarDecl != nil:
			renderVarDecl(b, stmt.VarDecl, indent)
		case stmt.PubVarDecl != nil:
			continue
		case stmt.VarAssign != nil:
			renderVarAssign(b, stmt.VarAssign, indent, className)
		case stmt.ListDecl != nil:
			renderListDecl(b, stmt.ListDecl, indent)
		case stmt.ObjectDecl != nil:
			renderObjectDecl(b, stmt.ObjectDecl, indent)
		case stmt.MethodCall != nil:
			renderMethodCall(b, stmt.MethodCall, indent)
		case stmt.VarDeclMethodCall != nil:
			renderVarDeclMethodCall(b, stmt.VarDeclMethodCall, indent)
		case stmt.FunctionCall != nil:
			renderFunctionCall(b, stmt.FunctionCall, indent)
		case stmt.TopLevelFuncDecl != nil:
			continue
		case stmt.ClassDecl != nil:
			continue
		case stmt.PubClassDecl != nil:
			continue
		}
	}
}

func renderObjectDecl(b *strings.Builder, objDecl *lexer.ObjectDeclStmt, indent string) {
	objectType := objDecl.Type
	if len(objDecl.Args) >= 2 {
		moduleName := objDecl.Args[0]
		className := objDecl.Args[1]
		objectType = fmt.Sprintf("%s_%s", moduleName, className)
	} else {
		objectType = lexer.ResolveSymbol(objDecl.Type, currentModule)
	}

	objectInfo := &ObjectInfo{
		Name: objDecl.Name,
		Type: objectType,
	}
	globalObjects[objDecl.Name] = objectInfo

	fmt.Fprintf(b, "%s%s* %s = %s_new();\n", indent, objectType, objDecl.Name, objectType)
}

func renderMethodCall(b *strings.Builder, methodCall *lexer.MethodCallStmt, indent string) {
	objectType := getObjectType(methodCall.Object)

	fmt.Fprintf(b, "%s%s_%s(%s", indent, objectType, methodCall.Method, methodCall.Object)

	for _, arg := range methodCall.Args {
		resolvedArg := lexer.ResolveSymbol(arg, currentModule)
		fmt.Fprintf(b, ", %s", resolvedArg)
	}

	b.WriteString(");\n")
}

func renderVarDeclMethodCall(b *strings.Builder, varDecl *lexer.VarDeclMethodCallStmt, indent string) {
	objectType := getObjectType(varDecl.Object)
	cType := mapTypeToCType(varDecl.Type)

	fmt.Fprintf(b, "%s%s %s = %s_%s(%s", indent, cType, varDecl.Name, objectType, varDecl.Method, varDecl.Object)

	for _, arg := range varDecl.Args {
		resolvedArg := lexer.ResolveSymbol(arg, currentModule)
		fmt.Fprintf(b, ", %s", resolvedArg)
	}

	b.WriteString(");\n")
}

func getObjectType(objectName string) string {
	if objectInfo, exists := globalObjects[objectName]; exists {
		return objectInfo.Type
	}

	for moduleName, module := range lexer.LoadedModules {
		for className := range module.PublicClasses {
			qualifiedName := fmt.Sprintf("%s_%s", moduleName, className)
			if strings.Contains(strings.ToLower(objectName), strings.ToLower(qualifiedName)) {
				return qualifiedName
			}
		}
	}

	for className := range globalClasses {
		if strings.Contains(strings.ToLower(objectName), strings.ToLower(className)) {
			return className
		}
	}
	return "Object"
}

func renderVarDecl(b *strings.Builder, varDecl *lexer.VarDeclStmt, indent string) {
	if strings.HasPrefix(varDecl.Name, "this.") {
		return
	}

	cType := mapTypeToCType(varDecl.Type)
	value := varDecl.Value

	if strings.Contains(value, "*") || strings.Contains(value, "+") || strings.Contains(value, "-") || strings.Contains(value, "/") || strings.Contains(value, "%") {
		parts := strings.Fields(value)
		var resolvedParts []string
		for _, part := range parts {
			if lexer.IsOperator(part) {
				resolvedParts = append(resolvedParts, part)
			} else {
				resolvedParts = append(resolvedParts, lexer.ResolveSymbol(part, currentModule))
			}
		}
		value = strings.Join(resolvedParts, " ")
	} else {
		value = lexer.ResolveSymbol(varDecl.Value, currentModule)
	}

	if varDecl.Type == "string" {
		if !strings.HasPrefix(value, "\"") {
			value = fmt.Sprintf("\"%s\"", value)
		}
		fmt.Fprintf(b, "%s%s %s[256];\n", indent, cType, varDecl.Name)
		fmt.Fprintf(b, "%sstrcpy(%s, %s);\n", indent, varDecl.Name, value)
	} else {
		fmt.Fprintf(b, "%s%s %s = %s;\n", indent, cType, varDecl.Name, value)
	}
}

func renderVarAssign(b *strings.Builder, varAssign *lexer.VarAssignStmt, indent string, className string) {
	value := lexer.ResolveSymbol(varAssign.Value, currentModule)
	name := varAssign.Name

	if strings.HasPrefix(name, "this.") {
		fieldName := name[5:]
		var fieldType string
		if classInfo, ok := globalClasses[className]; ok {
			for _, field := range classInfo.Fields {
				if field.Name == fieldName {
					fieldType = field.Type
					break
				}
			}
		}

		if fieldType == "string" {
			fmt.Fprintf(b, "%sstrcpy(this->%s, %s);\n", indent, fieldName, value)
		} else {
			fmt.Fprintf(b, "%sthis->%s = %s;\n", indent, fieldName, value)
		}
		return
	}

	if strings.Contains(name, "[") && strings.Contains(name, "]") {
		if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
			bracketStart := strings.Index(name, "[")
			listName := name[:bracketStart]
			indexPart := name[bracketStart:]
			fmt.Fprintf(b, "%sstrcpy(%s%s, %s);\n", indent, listName, indexPart, value)
		} else {
			fmt.Fprintf(b, "%s%s = %s;\n", indent, name, value)
		}
	} else {
		if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
			fmt.Fprintf(b, "%sstrcpy(%s, %s);\n", indent, name, value)
		} else if strings.Contains(value, ".") && strings.Contains(value, "(") && strings.Contains(value, ")") {
			parts := strings.SplitN(value, ".", 2)
			objectName := parts[0]
			methodCallPart := parts[1]
			methodName := methodCallPart[:strings.Index(methodCallPart, "(")]
			argsString := methodCallPart[strings.Index(methodCallPart, "(")+1 : strings.LastIndex(methodCallPart, ")")]
			args := []string{}
			if argsString != "" {
				argList := strings.SplitSeq(argsString, ", ")
				for arg := range argList {
					args = append(args, lexer.ResolveSymbol(arg, currentModule))
				}
			}

			objectType := getObjectType(objectName)
			fmt.Fprintf(b, "%s%s = %s_%s(%s", indent, name, objectType, methodName, objectName)
			for _, arg := range args {
				fmt.Fprintf(b, ", %s", arg)
			}
			b.WriteString(");\n")

		} else {
			fmt.Fprintf(b, "%s%s = %s;\n", indent, name, value)
		}
	}
}

func renderListDecl(b *strings.Builder, listDecl *lexer.ListDeclStmt, indent string) {
	cType := mapTypeToCType(listDecl.Type)
	listName := listDecl.Name
	elements := listDecl.Elements
	arraySize := len(elements)
	if arraySize == 0 {
		arraySize = 100
	} else if arraySize < 10 {
		arraySize = 10
	}

	if listDecl.Type == "string" {
		fmt.Fprintf(b, "%s%s %s[%d][256];\n", indent, cType, listName, arraySize)
		for i, elem := range elements {
			resolvedElem := lexer.ResolveSymbol(elem, currentModule)
			if !strings.HasPrefix(resolvedElem, "\"") {
				resolvedElem = fmt.Sprintf("\"%s\"", resolvedElem)
			}
			fmt.Fprintf(b, "%sstrcpy(%s[%d], %s);\n", indent, listName, i, resolvedElem)
		}
	} else {
		fmt.Fprintf(b, "%s%s %s[%d]", indent, cType, listName, arraySize)

		if len(elements) > 0 {
			fmt.Fprintf(b, " = {")
			for i, elem := range elements {
				if i > 0 {
					fmt.Fprintf(b, ", ")
				}
				resolvedElem := lexer.ResolveSymbol(elem, currentModule)
				fmt.Fprintf(b, "%s", resolvedElem)
			}
			fmt.Fprintf(b, "}")
		}
		fmt.Fprintf(b, ";\n")
	}
}

func findClassDeclByName(program *lexer.Program, className string) *lexer.ClassDeclStmt {
	for _, stmt := range program.Statements {
		if stmt.ClassDecl != nil && stmt.ClassDecl.Name == className {
			return stmt.ClassDecl
		}
		if stmt.PubClassDecl != nil && stmt.PubClassDecl.Name == className {
			return &lexer.ClassDeclStmt{
				Name:        stmt.PubClassDecl.Name,
				Constructor: stmt.PubClassDecl.Constructor,
				Methods:     stmt.PubClassDecl.Methods,
			}
		}
	}

	for _, module := range lexer.LoadedModules {
		if classDecl, exists := module.PublicClasses[className]; exists {
			return classDecl
		}
		modulePrefixedName := lexer.GenerateUniqueSymbol(className, module.Name)
		if strings.HasSuffix(className, "_"+module.Name) || className == modulePrefixedName {
			originalName := strings.TrimSuffix(className, "_"+module.Name)
			if classDecl, exists := module.PublicClasses[originalName]; exists {
				return classDecl
			}
		}
	}

	return nil
}

func findMethodDecl(classDecl *lexer.ClassDeclStmt, methodName string) *lexer.MethodDeclStmt {
	for _, method := range classDecl.Methods {
		if method.Name == methodName {
			return method
		}
	}
	return nil
}

func generateTopLevelFunctionImplementation(b *strings.Builder, funcDecl *lexer.TopLevelFuncDeclStmt) {
	returnType := "void"
	if funcDecl.ReturnType != "" && funcDecl.ReturnType != "void" {
		returnType = mapTypeToCType(funcDecl.ReturnType)
	}

	fmt.Fprintf(b, "%s %s(", returnType, funcDecl.Name)

	for i, param := range funcDecl.Parameters {
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
	renderStatements(b, funcDecl.Body, "    ", "")
	b.WriteString("}\n\n")
}

func renderFunctionCall(b *strings.Builder, funcCall *lexer.FunctionCallStmt, indent string) {
	fmt.Fprintf(b, "%s%s(", indent, funcCall.Name)

	for i, arg := range funcCall.Args {
		if i > 0 {
			b.WriteString(", ")
		}
		resolvedArg := lexer.ResolveSymbol(arg, currentModule)
		fmt.Fprintf(b, "%s", resolvedArg)
	}

	b.WriteString(");\n")
}

func mapTypeToCType(dslType string) string {
	switch dslType {
	case "int":
		return "int"
	case "float":
		return "float"
	case "double":
		return "double"
	case "char":
		return "char"
	case "string":
		return "char"
	case "bool":
		return "int"
	default:
		return "int"
	}
}
