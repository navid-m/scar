package main

import (
	"fmt"
	"strings"
)

func renderC(program *Program) string {
	var b strings.Builder
	b.WriteString(`#include <stdio.h>
#include <unistd.h>
#include <string.h>

int main() {
`)
	renderStatements(&b, program.Statements, "    ")
	b.WriteString("    return 0;\n")
	b.WriteString("}\n")
	return b.String()
}

func renderStatements(b *strings.Builder, stmts []*Statement, indent string) {
	for _, stmt := range stmts {
		switch {
		case stmt.Print != nil:
			if stmt.Print.Format != "" && stmt.Print.Variable != "" {
				if strings.Contains(stmt.Print.Format, "%s") {
					fmt.Fprintf(b, "%sprintf(\"%s\\n\", %s);\n", indent, stmt.Print.Format, stmt.Print.Variable)
				} else if strings.Contains(stmt.Print.Format, "%d") {
					fmt.Fprintf(b, "%sprintf(\"%s\\n\", %s);\n", indent, stmt.Print.Format, stmt.Print.Variable)
				} else {
					fmt.Fprintf(b, "%sprintf(\"%s %s\\n\");\n", indent, stmt.Print.Format, stmt.Print.Variable)
				}
			} else {
				fmt.Fprintf(b, "%sprintf(\"%s\\n\");\n", indent, stmt.Print.Print)
			}
		case stmt.Sleep != nil:
			fmt.Fprintf(b, "%ssleep(%s);\n", indent, stmt.Sleep.Duration)
		case stmt.Break != nil:
			fmt.Fprintf(b, "%sbreak;\n", indent)
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
			fmt.Fprintf(b, "%s}\n", indent)
		case stmt.VarDecl != nil:
			renderVarDecl(b, stmt.VarDecl, indent)
		}
	}
}

func renderVarDecl(b *strings.Builder, varDecl *VarDeclStmt, indent string) {
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
