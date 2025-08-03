// By Navid M (c)
// Date: 2025
// License: GPL3
//
// Contains function hoisting logic for the Scar compiler.

package lexer

import (
	"fmt"
	"strings"
)

// functionNode represents a function and its dependencies in the dependency graph
type functionNode struct {
	name         string
	dependencies map[string]bool
	statement    *Statement
}

// processFunctionDependencies analyzes a function body and extracts called function names
func processFunctionDependencies(body []*Statement) map[string]bool {
	deps := make(map[string]bool)

	var processStmt func(*Statement)
	processStmt = func(stmt *Statement) {
		switch {
		case stmt.FunctionCall != nil:
			deps[stmt.FunctionCall.Name] = true

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
			// Process main if body
			for _, s := range stmt.If.Body {
				processStmt(s)
			}

			// Process elif branches
			for _, elif := range stmt.If.ElseIfs {
				for _, s := range elif.Body {
					processStmt(s)
				}
			}

			// Process else branch if it exists
			if stmt.If.Else != nil {
				for _, s := range stmt.If.Else.Body {
					processStmt(s)
				}
			}

		case stmt.While != nil:
			for _, s := range stmt.While.Body {
				processStmt(s)
			}

		case stmt.For != nil:
			for _, s := range stmt.For.Body {
				processStmt(s)
			}

		case stmt.ParallelFor != nil:
			for _, s := range stmt.ParallelFor.Body {
				processStmt(s)
			}

		case stmt.TryCatch != nil:
			// Process try block
			for _, s := range stmt.TryCatch.TryBody {
				processStmt(s)
			}
			// Process catch block
			for _, s := range stmt.TryCatch.CatchBody {
				processStmt(s)
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

	// Remove self-references
	delete(deps, "")

	return deps
}

// buildDependencyGraph builds a dependency graph of all functions in the program
func buildDependencyGraph(statements []*Statement) (map[string]*functionNode, error) {
	graph := make(map[string]*functionNode)

	// First pass: collect all function declarations
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
			}
		} else if stmt.PubTopLevelFuncDecl != nil {
			name := stmt.PubTopLevelFuncDecl.Name
			if _, exists := graph[name]; exists {
				return nil, fmt.Errorf("duplicate function declaration: %s", name)
			}
			graph[name] = &functionNode{
				name:         name,
				dependencies: make(map[string]bool),
				statement:    stmt,
			}
		}
	}

	// Second pass: analyze dependencies
	for name, node := range graph {
		var body []*Statement
		if node.statement.TopLevelFuncDecl != nil {
			body = node.statement.TopLevelFuncDecl.Body
		} else if node.statement.PubTopLevelFuncDecl != nil {
			body = node.statement.PubTopLevelFuncDecl.Body
		}

		for dep := range processFunctionDependencies(body) {
			// Only track dependencies on other functions we know about
			if _, exists := graph[dep]; exists && dep != name {
				node.dependencies[dep] = true
			}
		}
	}

	return graph, nil
}

// topologicalSort performs a topological sort on the function dependency graph
func topologicalSort(graph map[string]*functionNode) ([]*Statement, error) {
	var result []*Statement
	visited := make(map[string]bool)
	temp := make(map[string]bool)
	var cycle []string

	var visit func(string) error
	visit = func(name string) error {
		if temp[name] {
			// Cycle detected
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

		for dep := range node.dependencies {
			if err := visit(dep); err != nil {
				return err
			}
		}

		temp[name] = false
		visited[name] = true
		result = append(result, node.statement)
		return nil
	}

	for name := range graph {
		if !visited[name] {
			if err := visit(name); err != nil {
				return nil, err
			}
		}
	}

	return result, nil
}

// HoistFunctions reorders function declarations to satisfy dependencies
func HoistFunctions(statements []*Statement) ([]*Statement, error) {
	// Separate function declarations from other statements
	var funcStmts, otherStmts []*Statement
	for _, stmt := range statements {
		if stmt.TopLevelFuncDecl != nil || stmt.PubTopLevelFuncDecl != nil {
			funcStmts = append(funcStmts, stmt)
		} else {
			otherStmts = append(otherStmts, stmt)
		}
	}

	// If there are no functions or only one function, no need to hoist
	if len(funcStmts) <= 1 {
		return statements, nil
	}

	// Build dependency graph
	graph, err := buildDependencyGraph(funcStmts)
	if err != nil {
		return nil, err
	}

	// Perform topological sort
	sortedFuncStmts, err := topologicalSort(graph)
	if err != nil {
		return nil, err
	}

	// Combine non-function statements with sorted function declarations
	// Put all non-function statements first, then sorted functions
	var result []*Statement
	result = append(result, otherStmts...)
	result = append(result, sortedFuncStmts...)

	return result, nil
}
