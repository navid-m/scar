// By Navid M (c)
// Date: 2025
// License: GPL3
//
// IL code generator for the scar programming language.

package renderer

import (
	"fmt"
	"scar/lexer"
	"strings"
)

var (
	globalClasses   = make(map[string]*ClassInfo)
	globalObjects   = make(map[string]*ObjectInfo)
	globalFunctions = make(map[string]*lexer.TopLevelFuncDeclStmt)
	globalArrays    = make(map[string]string)
	currentModule   = ""
)

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
		if stmt.PubTopLevelFuncDecl != nil {
			topLevelFunc := &lexer.TopLevelFuncDeclStmt{
				Name:       stmt.PubTopLevelFuncDecl.Name,
				Parameters: stmt.PubTopLevelFuncDecl.Parameters,
				ReturnType: stmt.PubTopLevelFuncDecl.ReturnType,
				Body:       stmt.PubTopLevelFuncDecl.Body,
			}
			globalFunctions[stmt.PubTopLevelFuncDecl.Name] = topLevelFunc
		}
	}

	for _, module := range lexer.LoadedModules {
		for _, classDecl := range module.PublicClasses {
			collectClassInfoWithModule(classDecl, module.Name)
		}
	}

	b.WriteString(`#include <stdio.h>
#include <string.h>
#include <unistd.h>
#include <omp.h>
#include <stdlib.h>

int _exception = 0;

`)

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
		if stmt.ClassDecl == nil && stmt.PubClassDecl == nil && stmt.PubVarDecl == nil {
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
		// We track unique fields here to avoid duplicates
		fieldMap := make(map[string]bool)
		for _, param := range classDecl.Constructor.Parameters {
			if _, exists := fieldMap[param.Name]; !exists {
				fieldInfo := FieldInfo{
					Name: param.Name,
					Type: param.Type,
				}
				classInfo.Fields = append(classInfo.Fields, fieldInfo)
				fieldMap[param.Name] = true
			}
		}
		for _, stmt := range classDecl.Constructor.Fields {
			if stmt.VarDecl != nil {
				fieldName := stmt.VarDecl.Name
				if strings.HasPrefix(fieldName, "this.") {
					fieldName = fieldName[5:]
				}
				if _, exists := fieldMap[fieldName]; !exists {
					fieldInfo := FieldInfo{
						Name: fieldName,
						Type: stmt.VarDecl.Type,
					}
					classInfo.Fields = append(classInfo.Fields, fieldInfo)
					fieldMap[fieldName] = true
				}
			}
			if stmt.VarAssign != nil && strings.HasPrefix(stmt.VarAssign.Name, "this.") {
				fieldName := stmt.VarAssign.Name[5:]
				if _, exists := fieldMap[fieldName]; !exists {
					fieldType := inferTypeFromValue(stmt.VarAssign.Value)
					fieldInfo := FieldInfo{
						Name: fieldName,
						Type: fieldType,
					}
					classInfo.Fields = append(classInfo.Fields, fieldInfo)
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

func inferTypeFromValue(value string) string {
	if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
		return "string"
	}
	if strings.HasPrefix(value, "new ") {
		return "object"
	}
	if strings.Contains(value, ".") && !strings.HasPrefix(value, "new ") {
		return "float"
	}
	if value == "true" || value == "false" {
		return "bool"
	}
	return "int"
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

func generateClassImplementation(b *strings.Builder, classDecl *lexer.ClassDeclStmt, moduleName string, program *lexer.Program) {
	className := classDecl.Name
	if moduleName != "" {
		className = lexer.GenerateUniqueSymbol(classDecl.Name, moduleName)
	}

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

	fmt.Fprintf(b, "    %s* obj = malloc(sizeof(%s));\n", className, className)

	if classDecl.Constructor != nil {
		assignedFields := make(map[string]bool)

		if len(classDecl.Constructor.Parameters) > 0 {
			for _, param := range classDecl.Constructor.Parameters {
				if _, exists := assignedFields[param.Name]; !exists {
					if param.Type == "string" {
						fmt.Fprintf(b, "    strcpy(obj->%s, %s);\n", param.Name, param.Name)
					} else {
						fmt.Fprintf(b, "    obj->%s = %s;\n", param.Name, param.Name)
					}
					assignedFields[param.Name] = true
				}
			}
		}

		for _, field := range classDecl.Constructor.Fields {
			if field.VarDecl != nil && strings.HasPrefix(field.VarDecl.Name, "this.") {
				fieldName := field.VarDecl.Name[5:]
				if _, exists := assignedFields[fieldName]; exists {
					continue
				}
				value := field.VarDecl.Value
				fieldType := field.VarDecl.Type

				if fieldType == "string" {
					if !strings.HasPrefix(value, "\"") {
						value = fmt.Sprintf("\"%s\"", value)
					}
					fmt.Fprintf(b, "    strcpy(obj->%s, %s);\n", fieldName, value)
				} else {
					fmt.Fprintf(b, "    obj->%s = %s;\n", fieldName, value)
				}
				assignedFields[fieldName] = true
			} else if field.VarAssign != nil && strings.HasPrefix(field.VarAssign.Name, "this.") {
				fieldName := field.VarAssign.Name[5:]
				if _, exists := assignedFields[fieldName]; exists {
					continue
				}
				value := field.VarAssign.Value
				var fieldType string
				if classInfo, exists := globalClasses[className]; exists {
					for _, f := range classInfo.Fields {
						if f.Name == fieldName {
							fieldType = f.Type
							break
						}
					}
				}

				if fieldType == "string" {
					if !strings.HasPrefix(value, "\"") && !isValidIdentifier(value) {
						value = fmt.Sprintf("\"%s\"", value)
					}
					fmt.Fprintf(b, "    strcpy(obj->%s, %s);\n", fieldName, value)
				} else {
					fmt.Fprintf(b, "    obj->%s = %s;\n", fieldName, value)
				}
				assignedFields[fieldName] = true
			} else if field.Print != nil {
				if field.Print.Format != "" && len(field.Print.Variables) > 0 {
					args := make([]string, len(field.Print.Variables))
					for i, v := range field.Print.Variables {
						if strings.HasPrefix(v, "this.") {
							fieldName := v[5:]
							args[i] = fmt.Sprintf("obj->%s", fieldName)
						} else {
							args[i] = lexer.ResolveSymbol(v, currentModule)
						}
					}
					argsStr := strings.Join(args, ", ")
					escapedFormat := strings.ReplaceAll(field.Print.Format, "\"", "\\\"")
					b.WriteString(fmt.Sprintf("    printf(\"%s\\n\", %s);\n", escapedFormat, argsStr))
				} else {
					fmt.Fprintf(b, "    printf(\"%s\\n\");\n", field.Print.Print)
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
		renderStatements(b, method.Body, "    ", className, program)
		b.WriteString("}\n\n")
	}
}

func renderStatements(b *strings.Builder, stmts []*lexer.Statement, indent string, className string, program *lexer.Program) {
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
				fmt.Fprintf(b, "%sprintf(\"%s\\n\", %s);\n", indent, escapedFormat, argsStr)
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
			fmt.Fprintf(b, "%swhile (%s) {\n", indent, condition)
			renderStatements(b, stmt.While.Body, indent+"    ", className, program)
			fmt.Fprintf(b, "%s}\n", indent)
		case stmt.For != nil:
			varName := lexer.ResolveSymbol(stmt.For.Var, currentModule)
			start := lexer.ResolveSymbol(stmt.For.Start, currentModule)
			end := lexer.ResolveSymbol(stmt.For.End, currentModule)
			fmt.Fprintf(b, "%sfor (int %s = %s; %s <= %s; %s++) {\n", indent, varName, start, varName, end, varName)
			renderStatements(b, stmt.For.Body, indent+"    ", className, program)
			fmt.Fprintf(b, "%s}\n", indent)
		case stmt.If != nil:
			condition := lexer.ResolveSymbol(stmt.If.Condition, currentModule)
			fmt.Fprintf(b, "%sif (%s) {\n", indent, condition)
			renderStatements(b, stmt.If.Body, indent+"    ", className, program)
			fmt.Fprintf(b, "%s}\n", indent)
			for _, elif := range stmt.If.ElseIfs {
				elifCondition := lexer.ResolveSymbol(elif.Condition, currentModule)
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
			cType := mapTypeToCType(stmt.VarDecl.Type)
			varName := lexer.ResolveSymbol(stmt.VarDecl.Name, currentModule)
			value := lexer.ResolveSymbol(stmt.VarDecl.Value, currentModule)
			if stmt.VarDecl.Type == "string" {
				fmt.Fprintf(b, "%s%s %s[256];\n", indent, cType, varName)
				if value != "" {
					if !strings.HasPrefix(value, "\"") {
						value = fmt.Sprintf("\"%s\"", value)
					}
					fmt.Fprintf(b, "%sstrcpy(%s, %s);\n", indent, varName, value)
				}
			} else {
				fmt.Fprintf(b, "%s%s %s = %s;\n", indent, cType, varName, value)
			}
		case stmt.VarAssign != nil:
			varName := lexer.ResolveSymbol(stmt.VarAssign.Name, currentModule)
			value := lexer.ResolveSymbol(stmt.VarAssign.Value, currentModule)
			if strings.HasPrefix(varName, "this.") {
				varName = "this->" + varName[5:]
			}
			if strings.Contains(varName, "[") && strings.Contains(varName, "]") {
				arrayName := varName[:strings.Index(varName, "[")]
				if arrayType, exists := globalArrays[arrayName]; exists && arrayType == "string" {
					if !strings.HasPrefix(value, "\"") {
						value = fmt.Sprintf("\"%s\"", value)
					}
					fmt.Fprintf(b, "%sstrcpy(%s, %s);\n", indent, varName, value)
				} else {
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
					if !strings.HasPrefix(value, "\"") && !isValidIdentifier(value) {
						value = fmt.Sprintf("\"%s\"", value)
					}
					fmt.Fprintf(b, "%sstrcpy(%s, %s);\n", indent, varName, value)
				} else {
					fmt.Fprintf(b, "%s%s = %s;\n", indent, varName, value)
				}
			}
		case stmt.ListDecl != nil:
			listType := mapTypeToCType(stmt.ListDecl.Type)
			listName := lexer.ResolveSymbol(stmt.ListDecl.Name, currentModule)
			globalArrays[stmt.ListDecl.Name] = stmt.ListDecl.Type
			if stmt.ListDecl.Type == "string" {
				fmt.Fprintf(b, "%s%s %s[%d][256];\n", indent, listType, listName, len(stmt.ListDecl.Elements))
			} else {
				fmt.Fprintf(b, "%s%s %s[%d];\n", indent, listType, listName, len(stmt.ListDecl.Elements))
			}
			for i, elem := range stmt.ListDecl.Elements {
				elem = lexer.ResolveSymbol(elem, currentModule)
				if stmt.ListDecl.Type == "string" {
					if !strings.HasPrefix(elem, "\"") {
						elem = fmt.Sprintf("\"%s\"", elem)
					}
					fmt.Fprintf(b, "%sstrcpy(%s[%d], %s);\n", indent, listName, i, elem)
				} else {
					fmt.Fprintf(b, "%s%s[%d] = %s;\n", indent, listName, i, elem)
				}
			}
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
			argsStr := ""
			if len(args) > 0 {
				constructorArgs := make([]string, 0)
				for _, arg := range args {
					if arg != typeName && arg != resolvedType {
						constructorArgs = append(constructorArgs, lexer.ResolveSymbol(arg, currentModule))
					}
				}
				argsStr = strings.Join(constructorArgs, ", ")
			}
			fmt.Fprintf(b, "%s%s* %s = %s_new(%s);\n", indent, resolvedType, varName, resolvedType, argsStr)
		case stmt.VarDeclMethodCall != nil:
			varType := mapTypeToCType(stmt.VarDeclMethodCall.Type)
			varName := lexer.ResolveSymbol(stmt.VarDeclMethodCall.Name, currentModule)
			objectName := lexer.ResolveSymbol(stmt.VarDeclMethodCall.Object, currentModule)
			methodName := stmt.VarDeclMethodCall.Method
			args := make([]string, len(stmt.VarDeclMethodCall.Args))
			for i, arg := range stmt.VarDeclMethodCall.Args {
				args[i] = lexer.ResolveSymbol(arg, currentModule)
			}
			argsStr := strings.Join(args, ", ")
			var resolvedClassName string
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
			if resolvedClassName == "" {
				resolvedClassName = "unknown"
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
			if resolvedClassName == "" {
				resolvedClassName = "unknown"
			}
			if argsStr == "" {
				fmt.Fprintf(b, "%s%s = %s_%s(%s);\n", indent, varName, resolvedClassName, methodName, objectName)
			} else {
				fmt.Fprintf(b, "%s%s = %s_%s(%s, %s);\n", indent, varName, resolvedClassName, methodName, objectName, argsStr)
			}
		case stmt.VarDeclInferred != nil:
			varName := lexer.ResolveSymbol(stmt.VarDeclInferred.Name, currentModule)
			value := lexer.ResolveSymbol(stmt.VarDeclInferred.Value, currentModule)
			varType := inferTypeFromValue(stmt.VarDeclInferred.Value)
			cType := mapTypeToCType(varType)
			if varType == "string" {
				fmt.Fprintf(b, "%s%s %s[256];\n", indent, cType, varName)
				if value != "" {
					if !strings.HasPrefix(value, "\"") {
						value = fmt.Sprintf("\"%s\"", value)
					}
					fmt.Fprintf(b, "%sstrcpy(%s, %s);\n", indent, varName, value)
				}
			} else {
				fmt.Fprintf(b, "%s%s %s = %s;\n", indent, cType, varName, value)
			}
		case stmt.VarDeclRead != nil:
			varName := lexer.ResolveSymbol(stmt.VarDeclRead.Name, currentModule)
			filePath := stmt.VarDeclRead.FilePath
			fpVarName := fmt.Sprintf("fp_read_%s", varName)

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
		case stmt.FunctionCall != nil:
			funcName := lexer.ResolveSymbol(stmt.FunctionCall.Name, currentModule)
			args := make([]string, len(stmt.FunctionCall.Args))
			for i, arg := range stmt.FunctionCall.Args {
				args[i] = lexer.ResolveSymbol(arg, currentModule)
			}
			argsStr := strings.Join(args, ", ")
			fmt.Fprintf(b, "%s%s(%s);\n", indent, funcName, argsStr)
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
			keyType := mapTypeToCType(stmt.MapDecl.KeyType)
			valueType := mapTypeToCType(stmt.MapDecl.ValueType)
			mapSize := len(stmt.MapDecl.Pairs)
			if mapSize == 0 {
				mapSize = 10
			}
			if stmt.MapDecl.KeyType == "string" {
				fmt.Fprintf(b, "%s%s %s_keys[%d][256];\n", indent, keyType, mapName, mapSize)
			} else {
				fmt.Fprintf(b, "%s%s %s_keys[%d];\n", indent, keyType, mapName, mapSize)
			}

			if stmt.MapDecl.ValueType == "string" {
				fmt.Fprintf(b, "%s%s %s_values[%d][256];\n", indent, valueType, mapName, mapSize)
			} else {
				fmt.Fprintf(b, "%s%s %s_values[%d];\n", indent, valueType, mapName, mapSize)
			}

			fmt.Fprintf(b, "%sint %s_size = %d;\n", indent, mapName, len(stmt.MapDecl.Pairs))

			for i, pair := range stmt.MapDecl.Pairs {
				key := pair.Key
				value := pair.Value

				if stmt.MapDecl.KeyType == "string" {
					if !strings.HasPrefix(key, "\"") {
						key = fmt.Sprintf("\"%s\"", key)
					}
					fmt.Fprintf(b, "%sstrcpy(%s_keys[%d], %s);\n", indent, mapName, i, key)
				} else {
					fmt.Fprintf(b, "%s%s_keys[%d] = %s;\n", indent, mapName, i, key)
				}

				if stmt.MapDecl.ValueType == "string" {
					if !strings.HasPrefix(value, "\"") {
						value = fmt.Sprintf("\"%s\"", value)
					}
					fmt.Fprintf(b, "%sstrcpy(%s_values[%d], %s);\n", indent, mapName, i, value)
				} else {
					fmt.Fprintf(b, "%s%s_values[%d] = %s;\n", indent, mapName, i, value)
				}
			}
		case stmt.ParallelFor != nil:
			varName := lexer.ResolveSymbol(stmt.ParallelFor.Var, currentModule)
			start := lexer.ResolveSymbol(stmt.ParallelFor.Start, currentModule)
			end := lexer.ResolveSymbol(stmt.ParallelFor.End, currentModule)
			fmt.Fprintf(b, "%s#pragma omp parallel for\n", indent)
			fmt.Fprintf(b, "%sfor (int %s = %s; %s <= %s; %s++) {\n", indent, varName, start, varName, end, varName)
			renderStatements(b, stmt.ParallelFor.Body, indent+"    ", className, program)
			fmt.Fprintf(b, "%s}\n", indent)
		}
	}
}

func generateTopLevelFunctionImplementation(b *strings.Builder, funcDecl *lexer.TopLevelFuncDeclStmt, program *lexer.Program) {
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
	renderStatements(b, funcDecl.Body, "    ", "", program)
	b.WriteString("}\n\n")
}

func isValidIdentifier(s string) bool {
	if len(s) == 0 {
		return false
	}
	if !((s[0] >= 'a' && s[0] <= 'z') || (s[0] >= 'A' && s[0] <= 'Z') || s[0] == '_') {
		return false
	}
	for i := 1; i < len(s); i++ {
		if !((s[i] >= 'a' && s[i] <= 'z') || (s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= '0' && s[i] <= '9') || s[i] == '_') {
			return false
		}
	}
	return true
}

func mapTypeToCType(mapType string) string {
	switch mapType {
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
