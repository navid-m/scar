package main

import (
	"fmt"
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

func renderC(program *Program) string {
	var b strings.Builder

	for _, stmt := range program.Statements {
		if stmt.ClassDecl != nil {
			collectClassInfo(stmt.ClassDecl)
		}
		if stmt.ObjectDecl != nil {
			objectInfo := &ObjectInfo{
				Name: stmt.ObjectDecl.Name,
				Type: stmt.ObjectDecl.Type,
			}
			globalObjects[stmt.ObjectDecl.Name] = objectInfo
		}
	}

	b.WriteString(`#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

`)
	for _, classInfo := range globalClasses {
		generateStructDefinition(&b, classInfo)
		b.WriteString("\n")
	}
	for className, classInfo := range globalClasses {
		for _, method := range classInfo.Methods {
			returnType := "void"
			if method.ReturnType != "" && method.ReturnType != "void" {
				returnType = mapTypeToCType(method.ReturnType)
			}

			fmt.Fprintf(&b, "%s %s_%s(%s* this", returnType, className, method.Name, className)
			if classDecl := findClassDecl(program, className); classDecl != nil {
				if methodDecl := findMethodDecl(classDecl, method.Name); methodDecl != nil {
					for _, param := range methodDecl.Parameters {
						paramType := mapTypeToCType(param.Type)
						fmt.Fprintf(&b, ", %s %s", paramType, param.Name)
					}
				}
			}
			b.WriteString(");\n")
		}
		fmt.Fprintf(&b, "%s* %s_new();\n", className, className)
		b.WriteString("\n")
	}

	for _, stmt := range program.Statements {
		if stmt.ClassDecl != nil {
			generateClassImplementation(&b, stmt.ClassDecl)
		}
	}

	b.WriteString("int main() {\n")

	var mainStatements []*Statement
	for _, stmt := range program.Statements {
		if stmt.ClassDecl == nil {
			mainStatements = append(mainStatements, stmt)
		}
	}

	renderStatements(&b, mainStatements, "    ")
	b.WriteString("    return 0;\n")
	b.WriteString("}\n")
	return b.String()
}

func collectClassInfo(classDecl *ClassDeclStmt) {
	classInfo := &ClassInfo{
		Name:    classDecl.Name,
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

	globalClasses[classDecl.Name] = classInfo
}

func generateStructDefinition(b *strings.Builder, classInfo *ClassInfo) {
	fmt.Fprintf(b, "typedef struct %s {\n", classInfo.Name)
	for _, field := range classInfo.Fields {
		cType := mapTypeToCType(field.Type)
		if field.Type == "string" {
			fmt.Fprintf(b, "    %s %s[256];\n", cType, field.Name)
		} else {
			fmt.Fprintf(b, "    %s %s;\n", cType, field.Name)
		}
	}

	fmt.Fprintf(b, "} %s;\n", classInfo.Name)
}

func generateClassImplementation(b *strings.Builder, classDecl *ClassDeclStmt) {
	className := classDecl.Name

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
			fmt.Fprintf(b, ", %s %s", paramType, param.Name)
		}

		b.WriteString(") {\n")
		renderStatements(b, method.Body, "    ")
		b.WriteString("}\n\n")
	}
}

func renderStatements(b *strings.Builder, stmts []*Statement, indent string) {
	for _, stmt := range stmts {
		switch {
		case stmt.Print != nil:
			if stmt.Print.Format != "" && len(stmt.Print.Variables) > 0 {
				args := make([]string, len(stmt.Print.Variables))
				for i, v := range stmt.Print.Variables {
					if strings.HasPrefix(v, "this.") {
						fieldName := v[5:]
						args[i] = fmt.Sprintf("this->%s", fieldName)
					} else if strings.Contains(v, "[") && strings.Contains(v, "]") {
						args[i] = v
					} else {
						args[i] = v
					}
				}
				argsStr := strings.Join(args, ", ")
				fmt.Fprintf(b, "%sprintf(\"%s\\n\", %s);\n", indent, stmt.Print.Format, argsStr)
			} else {
				fmt.Fprintf(b, "%sprintf(\"%s\\n\");\n", indent, stmt.Print.Print)
			}
		case stmt.Sleep != nil:
			fmt.Fprintf(b, "%ssleep(%s);\n", indent, stmt.Sleep.Duration)
		case stmt.Break != nil:
			fmt.Fprintf(b, "%sbreak;\n", indent)
		case stmt.Return != nil:
			fmt.Fprintf(b, "%sreturn %s;\n", indent, stmt.Return.Value)
		case stmt.While != nil:
			fmt.Fprintf(b, "%swhile (%s) {\n", indent, stmt.While.Condition)
			renderStatements(b, stmt.While.Body, indent+"    ")
			fmt.Fprintf(b, "%s}\n", indent)
		case stmt.For != nil:
			varName := stmt.For.Var
			start := stmt.For.Start
			end := stmt.For.End
			fmt.Fprintf(b, "%sfor (int %s = %s; %s <= %s; %s++) {\n", indent, varName, start, varName, end, varName)
			renderStatements(b, stmt.For.Body, indent+"    ")
			fmt.Fprintf(b, "%s}\n", indent)
		case stmt.If != nil:
			fmt.Fprintf(b, "%sif (%s) {\n", indent, stmt.If.Condition)
			renderStatements(b, stmt.If.Body, indent+"    ")

			for _, elif := range stmt.If.ElseIfs {
				fmt.Fprintf(b, "%s} else if (%s) {\n", indent, elif.Condition)
				renderStatements(b, elif.Body, indent+"    ")
			}

			if stmt.If.Else != nil {
				fmt.Fprintf(b, "%s} else {\n", indent)
				renderStatements(b, stmt.If.Else.Body, indent+"    ")
			}

			fmt.Fprintf(b, "%s}\n", indent)
		case stmt.VarDecl != nil:
			renderVarDecl(b, stmt.VarDecl, indent)
		case stmt.VarAssign != nil:
			renderVarAssign(b, stmt.VarAssign, indent)
		case stmt.ListDecl != nil:
			renderListDecl(b, stmt.ListDecl, indent)
		case stmt.ObjectDecl != nil:
			renderObjectDecl(b, stmt.ObjectDecl, indent)
		case stmt.MethodCall != nil:
			renderMethodCall(b, stmt.MethodCall, indent)
		case stmt.VarDeclMethodCall != nil:
			renderVarDeclMethodCall(b, stmt.VarDeclMethodCall, indent)
		case stmt.ClassDecl != nil:
			continue
		}
	}
}

func renderObjectDecl(b *strings.Builder, objDecl *ObjectDeclStmt, indent string) {
	objectInfo := &ObjectInfo{
		Name: objDecl.Name,
		Type: objDecl.Type,
	}
	globalObjects[objDecl.Name] = objectInfo

	fmt.Fprintf(b, "%s%s* %s = %s_new();\n", indent, objDecl.Type, objDecl.Name, objDecl.Type)
}

func renderMethodCall(b *strings.Builder, methodCall *MethodCallStmt, indent string) {
	objectType := getObjectType(methodCall.Object)

	fmt.Fprintf(b, "%s%s_%s(%s", indent, objectType, methodCall.Method, methodCall.Object)

	for _, arg := range methodCall.Args {
		fmt.Fprintf(b, ", %s", arg)
	}

	b.WriteString(");\n")
}

func renderVarDeclMethodCall(b *strings.Builder, varDecl *VarDeclMethodCallStmt, indent string) {
	objectType := getObjectType(varDecl.Object)
	cType := mapTypeToCType(varDecl.Type)

	fmt.Fprintf(b, "%s%s %s = %s_%s(%s", indent, cType, varDecl.Name, objectType, varDecl.Method, varDecl.Object)

	for _, arg := range varDecl.Args {
		fmt.Fprintf(b, ", %s", arg)
	}

	b.WriteString(");\n")
}

func getObjectType(objectName string) string {
	if objectInfo, exists := globalObjects[objectName]; exists {
		return objectInfo.Type
	}
	for className := range globalClasses {
		if strings.Contains(strings.ToLower(objectName), strings.ToLower(className)) {
			return className
		}
	}
	return "Object"
}

func renderVarDecl(b *strings.Builder, varDecl *VarDeclStmt, indent string) {
	if strings.HasPrefix(varDecl.Name, "this.") {
		return
	}

	cType := mapTypeToCType(varDecl.Type)
	value := varDecl.Value
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

func renderVarAssign(b *strings.Builder, varAssign *VarAssignStmt, indent string) {
	value := varAssign.Value
	name := varAssign.Name

	if strings.HasPrefix(name, "this.") {
		fieldName := name[5:]
		if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
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
		} else {
			fmt.Fprintf(b, "%s%s = %s;\n", indent, name, value)
		}
	}
}

func renderListDecl(b *strings.Builder, listDecl *ListDeclStmt, indent string) {
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
			if !strings.HasPrefix(elem, "\"") {
				elem = fmt.Sprintf("\"%s\"", elem)
			}
			fmt.Fprintf(b, "%sstrcpy(%s[%d], %s);\n", indent, listName, i, elem)
		}
	} else {
		fmt.Fprintf(b, "%s%s %s[%d]", indent, cType, listName, arraySize)

		if len(elements) > 0 {
			fmt.Fprintf(b, " = {")
			for i, elem := range elements {
				if i > 0 {
					fmt.Fprintf(b, ", ")
				}
				fmt.Fprintf(b, "%s", elem)
			}
			fmt.Fprintf(b, "}")
		}
		fmt.Fprintf(b, ";\n")
	}
}

func findClassDecl(program *Program, className string) *ClassDeclStmt {
	for _, stmt := range program.Statements {
		if stmt.ClassDecl != nil && stmt.ClassDecl.Name == className {
			return stmt.ClassDecl
		}
	}
	return nil
}

func findMethodDecl(classDecl *ClassDeclStmt, methodName string) *MethodDeclStmt {
	for _, method := range classDecl.Methods {
		if method.Name == methodName {
			return method
		}
	}
	return nil
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
