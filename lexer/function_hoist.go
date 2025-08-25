// By Navid M (c)
// Date: 2025
// License: GPL3
//
// Contains function hoisting logic for the Scar compiler.

package lexer

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Represents a function and its dependencies in the dependency graph
type functionNode struct {
	name         string
	dependencies map[string]bool
	statement    *Statement
	order        int
}

// Analyzes a function body and extracts called function names
func processFunctionDependencies(body []*Statement) map[string]bool {
	deps := make(map[string]bool)
	callRe := regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	addCallsFromText := func(text string) {
		if text == "" {
			return
		}
		matches := callRe.FindAllStringSubmatch(text, -1)
		for _, m := range matches {
			if len(m) > 1 {
				name := m[1]
				if name != "" {
					deps[name] = true
				}
			}
		}
	}

	var processStmt func(*Statement)
	processStmt = func(stmt *Statement) {
		switch {
		case stmt.FunctionCall != nil:
			deps[stmt.FunctionCall.Name] = true

		case stmt.ListDeclFunctionCall != nil:
			// e.g. x := someFunc(...)
			addCallsFromText(stmt.ListDeclFunctionCall.FunctionCall)

		case stmt.Run != nil:
			addCallsFromText(stmt.Run.FunctionCall)

		case stmt.TopLevelFuncDecl != nil:
			subDeps := processFunctionDependencies(stmt.TopLevelFuncDecl.Body)
			for dep := range subDeps {
				deps[dep] = true
			}

		case stmt.PubTopLevelFuncDecl != nil:
			subDeps := processFunctionDependencies(stmt.PubTopLevelFuncDecl.Body)
			for dep := range subDeps {
				deps[dep] = true
			}

		case stmt.If != nil:
			addCallsFromText(stmt.If.Condition)
			for _, s := range stmt.If.Body {
				processStmt(s)
			}
			for _, elif := range stmt.If.ElseIfs {
				addCallsFromText(elif.Condition)
				for _, s := range elif.Body {
					processStmt(s)
				}
			}
			if stmt.If.Else != nil {
				for _, s := range stmt.If.Else.Body {
					processStmt(s)
				}
			}

		case stmt.While != nil:
			addCallsFromText(stmt.While.Condition)
			for _, s := range stmt.While.Body {
				processStmt(s)
			}

		case stmt.For != nil:
			addCallsFromText(stmt.For.Start)
			addCallsFromText(stmt.For.End)
			for _, s := range stmt.For.Body {
				processStmt(s)
			}

		case stmt.ReverseFor != nil:
			addCallsFromText(stmt.ReverseFor.Start)
			addCallsFromText(stmt.ReverseFor.End)
			for _, s := range stmt.ReverseFor.Body {
				processStmt(s)
			}

		case stmt.VerboseFor != nil:
			addCallsFromText(stmt.VerboseFor.Init)
			addCallsFromText(stmt.VerboseFor.Condition)
			addCallsFromText(stmt.VerboseFor.Increment)
			for _, s := range stmt.VerboseFor.Body {
				processStmt(s)
			}

		case stmt.ParallelFor != nil:
			addCallsFromText(stmt.ParallelFor.Start)
			addCallsFromText(stmt.ParallelFor.End)
			for _, s := range stmt.ParallelFor.Body {
				processStmt(s)
			}

		case stmt.ParallelWhile != nil:
			addCallsFromText(stmt.ParallelWhile.Condition)
			for _, s := range stmt.ParallelWhile.Body {
				processStmt(s)
			}

		case stmt.ParallelBlock != nil:
			for _, s := range stmt.ParallelBlock.Body {
				processStmt(s)
			}

		case stmt.Platform != nil:
			for _, s := range stmt.Platform.Body {
				processStmt(s)
			}

		case stmt.TryCatch != nil:
			for _, s := range stmt.TryCatch.TryBody {
				processStmt(s)
			}
			for _, s := range stmt.TryCatch.CatchBody {
				processStmt(s)
			}

		case stmt.VarDecl != nil:
			addCallsFromText(stmt.VarDecl.Value)

		case stmt.VarAssign != nil:
			addCallsFromText(stmt.VarAssign.Value)

		case stmt.IndexAssign != nil:
			addCallsFromText(stmt.IndexAssign.Index)
			addCallsFromText(stmt.IndexAssign.Value)

		case stmt.Return != nil:
			addCallsFromText(stmt.Return.Value)

		case stmt.Foreach != nil:
			addCallsFromText(stmt.Foreach.Collection)

		case stmt.CatString != nil:
			addCallsFromText(stmt.CatString.Value)

		case stmt.CatList != nil:
			for _, v := range stmt.CatList.Lists {
				addCallsFromText(v)
			}

		case stmt.MapDecl != nil:
			for _, p := range stmt.MapDecl.Pairs {
				addCallsFromText(p.Key)
				addCallsFromText(p.Value)
			}

		case stmt.MethodCall != nil:
			addCallsFromText(stmt.MethodCall.Object)
			for _, a := range stmt.MethodCall.Args {
				addCallsFromText(a)
			}

		case stmt.StaticMethodCall != nil:
			for _, a := range stmt.StaticMethodCall.Args {
				addCallsFromText(a)
			}

		case stmt.ObjectDecl != nil:
			for _, a := range stmt.ObjectDecl.Args {
				addCallsFromText(a)
			}

		case stmt.ListDecl != nil:
			for _, e := range stmt.ListDecl.Elements {
				addCallsFromText(e)
			}

		case stmt.ClassDecl != nil:
			for _, method := range stmt.ClassDecl.Methods {
				for _, s := range method.Body {
					processStmt(s)
				}
			}

		case stmt.PubClassDecl != nil:
			for _, method := range stmt.PubClassDecl.Methods {
				for _, s := range method.Body {
					processStmt(s)
				}
			}
		}
	}

	for _, stmt := range body {
		processStmt(stmt)
	}

	delete(deps, "")

	return deps
}

// Builds a dependency graph of all functions in the program
func buildDependencyGraph(statements []*Statement, aliases map[string]string) (map[string]*functionNode, error) {
	graph := make(map[string]*functionNode)
	order := 0

	for _, stmt := range statements {
		if stmt.TopLevelFuncDecl != nil {
			name := stmt.TopLevelFuncDecl.Name
			if _, exists := graph[name]; exists {
				return nil, fmt.Errorf("duplicate function declaration: %s", name)
			}
			graph[name] = &functionNode{
				name:         name,
				dependencies: make(map[string]bool),
				statement:    stmt,
				order:        order,
			}
			order++
		} else if stmt.PubTopLevelFuncDecl != nil {
			name := stmt.PubTopLevelFuncDecl.Name
			if _, exists := graph[name]; exists {
				return nil, fmt.Errorf("duplicate function declaration: %s", name)
			}
			graph[name] = &functionNode{
				name:         name,
				dependencies: make(map[string]bool),
				statement:    stmt,
				order:        order,
			}
			order++
		}
	}

	for name, node := range graph {
		var body []*Statement
		if node.statement.TopLevelFuncDecl != nil {
			body = node.statement.TopLevelFuncDecl.Body
		} else if node.statement.PubTopLevelFuncDecl != nil {
			body = node.statement.PubTopLevelFuncDecl.Body
		}

		for dep := range processFunctionDependencies(body) {
			real := dep
			visited := make(map[string]bool)
			for {
				t, ok := aliases[real]
				if !ok || visited[real] {
					break
				}
				visited[real] = true
				real = t
			}
			if _, exists := graph[real]; exists && real != name {
				node.dependencies[real] = true
			}
		}
	}

	return graph, nil
}

// Performs a topological sort on the function dependency graph
func topologicalSort(graph map[string]*functionNode) ([]*Statement, error) {
	var result []*Statement
	visited := make(map[string]bool)
	temp := make(map[string]bool)
	var cycle []string

	var visit func(string) error
	visit = func(name string) error {
		if temp[name] {
			start := 0
			for i, n := range cycle {
				if n == name {
					start = i
					break
				}
			}
			cycle = append(cycle[start:], name)
			return fmt.Errorf("circular dependency detected: %s", strings.Join(cycle, " -> "))
		}

		if visited[name] {
			return nil
		}

		temp[name] = true
		cycle = append(cycle, name)

		node, exists := graph[name]
		if !exists {
			return fmt.Errorf("function not found: %s", name)
		}

		deps := make([]*functionNode, 0, len(node.dependencies))
		for dep := range node.dependencies {
			if dn, ok := graph[dep]; ok {
				deps = append(deps, dn)
			}
		}
		sort.Slice(deps, func(i, j int) bool {
			if deps[i].order == deps[j].order {
				return deps[i].name < deps[j].name
			}
			return deps[i].order < deps[j].order
		})
		for _, dn := range deps {
			if err := visit(dn.name); err != nil {
				return err
			}
		}

		temp[name] = false
		visited[name] = true
		result = append(result, node.statement)
		return nil
	}

	nodes := make([]*functionNode, 0, len(graph))
	for _, n := range graph {
		nodes = append(nodes, n)
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].order == nodes[j].order {
			return nodes[i].name < nodes[j].name
		}
		return nodes[i].order < nodes[j].order
	})
	for _, n := range nodes {
		if !visited[n.name] {
			if err := visit(n.name); err != nil {
				return nil, err
			}
		}
	}

	return result, nil
}

// Reorders function declarations to satisfy dependencies
func HoistFunctions(statements []*Statement) ([]*Statement, error) {
	var funcStmts, otherStmts []*Statement
	aliases := make(map[string]string)
	for _, stmt := range statements {
		if stmt.TopLevelFuncDecl != nil || stmt.PubTopLevelFuncDecl != nil {
			funcStmts = append(funcStmts, stmt)
		} else {
			otherStmts = append(otherStmts, stmt)
		}
		if stmt.Alias != nil {
			aliases[stmt.Alias.AliasName] = stmt.Alias.Target
		}
	}
	if len(funcStmts) <= 1 {
		return statements, nil
	}
	graph, err := buildDependencyGraph(funcStmts, aliases)
	if err != nil {
		return nil, err
	}
	sortedFuncStmts, err := topologicalSort(graph)
	if err != nil {
		return nil, err
	}
	var result []*Statement
	result = append(result, otherStmts...)
	result = append(result, sortedFuncStmts...)
	return result, nil
}
