package main

import (
	"fmt"
	"strings"
)

func renderC(program *Program) string {
	var b strings.Builder
	b.WriteString(`#include <stdio.h>
#include <unistd.h>

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
			fmt.Fprintf(b, "%sprintf(\"%s\\n\");\n", indent, stmt.Print.Print)
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
		}
	}
}
