package lexer

import (
	"fmt"
	"strings"
)

// Represents the signature of a function for validation
type FunctionSignature struct {
	Name       string
	Parameters []*MethodParameter
	ReturnType string
	Module     string
}

// Validates function call arguments
type ArgumentValidator struct {
	functions map[string]*FunctionSignature
}

// Creates a new argument validator
func NewArgumentValidator() *ArgumentValidator {
	return &ArgumentValidator{
		functions: make(map[string]*FunctionSignature),
	}
}

// Registers a function signature for validation
func (av *ArgumentValidator) RegisterFunction(
	name string,
	params []*MethodParameter,
	returnType string,
	module string,
) {
	signature := &FunctionSignature{
		Name:       name,
		Parameters: params,
		ReturnType: returnType,
		Module:     module,
	}
	av.functions[name] = signature
	if module != "" {
		av.functions[module+"_"+name] = signature
		av.functions[module+"::"+name] = signature
	}
}

func (av *ArgumentValidator) ValidateFunctionCall(funcCall *FunctionCallStmt, line int) error {
	funcName := funcCall.Name
	signature, exists := av.functions[funcName]
	if !exists {
		for name, sig := range av.functions {
			if name == funcName {
				signature = sig
				exists = true
				break
			}
			if strings.Contains(name, "::") {
				parts := strings.Split(name, "::")
				if len(parts) == 2 && parts[1] == funcName {
					signature = sig
					exists = true
					break
				}
			}
			if strings.Contains(funcName, "::") {
				callParts := strings.Split(funcName, "::")
				if len(callParts) == 2 {
					if name == callParts[1] {
						signature = sig
						exists = true
						break
					}
					if name == callParts[0]+"_"+callParts[1] {
						signature = sig
						exists = true
						break
					}
				}
			}
		}
	}

	if !exists {
		return nil
	}

	expectedCount := 0
	for range signature.Parameters {
		expectedCount++
	}

	actualCount := len(funcCall.Args)

	if actualCount != expectedCount {
		return fmt.Errorf("line %d: function '%s' expects %d arguments, but %d were provided",
			line, funcName, expectedCount, actualCount)
	}

	return nil
}

func (av *ArgumentValidator) ValidateMethodCall(methodCall *MethodCallStmt, line int) error {
	// For now, skip method validation
	return nil
}

func findMatchingParen(expr string, openPos int) int {
	if openPos >= len(expr) || expr[openPos] != '(' {
		return -1
	}

	parenCount := 1
	for i := openPos + 1; i < len(expr); i++ {
		switch expr[i] {
		case '(':
			parenCount++
		case ')':
			parenCount--
			if parenCount == 0 {
				return i
			}
		}
	}
	return -1
}

func (av *ArgumentValidator) ValidateStringFunctionCall(expr string, line int) error {
	if !strings.Contains(expr, "(") || !strings.Contains(expr, ")") {
		return nil
	}
	parenStart := strings.Index(expr, "(")
	parenEnd := findMatchingParen(expr, parenStart)
	if parenStart == -1 || parenEnd == -1 || parenEnd < parenStart {
		return nil
	}
	funcName := strings.TrimSpace(expr[:parenStart])
	argsStr := strings.TrimSpace(expr[parenStart+1 : parenEnd])
	args := parseArguments(argsStr)

	tempCall := &FunctionCallStmt{
		Name: funcName,
		Args: args,
	}
	return av.ValidateFunctionCall(tempCall, line)
}

// Parses function arguments while respecting nested parentheses
func parseArguments(argsStr string) []string {
	if strings.TrimSpace(argsStr) == "" {
		return []string{}
	}

	var args []string
	var current strings.Builder
	parenDepth := 0

	for _, char := range argsStr {
		switch char {
		case '(':
			parenDepth++
			current.WriteRune(char)
		case ')':
			parenDepth--
			current.WriteRune(char)
		case ',':
			if parenDepth == 0 {
				args = append(args, strings.TrimSpace(current.String()))
				current.Reset()
			} else {
				current.WriteRune(char)
			}
		default:
			current.WriteRune(char)
		}
	}

	if current.Len() > 0 {
		args = append(args, strings.TrimSpace(current.String()))
	}

	return args
}

// Validates all function calls in a program
func ValidateProgram(program *Program) []error {
	validator := NewArgumentValidator()
	var errors []error

	for _, stmt := range program.Statements {
		if stmt.Import != nil {
			_, err := LoadModule(stmt.Import.Module, "")
			if err != nil {
				continue
			}
		}
	}
	for _, imp := range program.Imports {
		_, err := LoadModule(imp.Module, "")
		if err != nil {
			continue
		}
	}

	for _, stmt := range program.Statements {
		if stmt.TopLevelFuncDecl != nil {
			validator.RegisterFunction(
				stmt.TopLevelFuncDecl.Name,
				stmt.TopLevelFuncDecl.Parameters,
				stmt.TopLevelFuncDecl.ReturnType,
				"",
			)
		}
		if stmt.PubTopLevelFuncDecl != nil {
			validator.RegisterFunction(
				stmt.PubTopLevelFuncDecl.Name,
				stmt.PubTopLevelFuncDecl.Parameters,
				stmt.PubTopLevelFuncDecl.ReturnType,
				"",
			)
		}
	}

	for _, module := range LoadedModules {
		for funcName, funcDecl := range module.PublicFuncs {
			validator.RegisterFunction(funcName, funcDecl.Parameters, funcDecl.ReturnType, module.Name)
		}
	}
	lineNum := 1
	for _, stmt := range program.Statements {
		stmtErrors := validateStatementRecursive(stmt, validator, lineNum)
		errors = append(errors, stmtErrors...)
		lineNum++
	}

	return errors
}

// Validates function calls in a statement and its nested statements
func validateStatementRecursive(stmt *Statement, validator *ArgumentValidator, line int) []error {
	var errors []error
	if stmt.FunctionCall != nil {
		if err := validator.ValidateFunctionCall(stmt.FunctionCall, line); err != nil {
			errors = append(errors, err)
		}
	}
	if stmt.MethodCall != nil {
		if err := validator.ValidateMethodCall(stmt.MethodCall, line); err != nil {
			errors = append(errors, err)
		}
	}
	if stmt.VarDecl != nil && stmt.VarDecl.Value != "" {
		if err := validator.ValidateStringFunctionCall(stmt.VarDecl.Value, line); err != nil {
			errors = append(errors, err)
		}
	}
	if stmt.VarAssign != nil {
		if err := validator.ValidateStringFunctionCall(stmt.VarAssign.Value, line); err != nil {
			errors = append(errors, err)
		}
	}
	if stmt.VarDeclMethodCall != nil {
		// TODO: This is a method call, handle differently
	}
	if stmt.Print != nil {
		for _, variable := range stmt.Print.Variables {
			if err := validator.ValidateStringFunctionCall(variable, line); err != nil {
				errors = append(errors, err)
			}
		}
	}

	if stmt.Return != nil {
		if err := validator.ValidateStringFunctionCall(stmt.Return.Value, line); err != nil {
			errors = append(errors, err)
		}
	}
	if stmt.If != nil {
		for _, nestedStmt := range stmt.If.Body {
			nestedErrors := validateStatementRecursive(nestedStmt, validator, line)
			errors = append(errors, nestedErrors...)
		}
		for _, elif := range stmt.If.ElseIfs {
			for _, nestedStmt := range elif.Body {
				nestedErrors := validateStatementRecursive(nestedStmt, validator, line)
				errors = append(errors, nestedErrors...)
			}
		}
		if stmt.If.Else != nil {
			for _, nestedStmt := range stmt.If.Else.Body {
				nestedErrors := validateStatementRecursive(nestedStmt, validator, line)
				errors = append(errors, nestedErrors...)
			}
		}
	}

	if stmt.While != nil {
		for _, nestedStmt := range stmt.While.Body {
			nestedErrors := validateStatementRecursive(nestedStmt, validator, line)
			errors = append(errors, nestedErrors...)
		}
	}

	if stmt.For != nil {
		for _, nestedStmt := range stmt.For.Body {
			nestedErrors := validateStatementRecursive(nestedStmt, validator, line)
			errors = append(errors, nestedErrors...)
		}
	}

	if stmt.TopLevelFuncDecl != nil {
		for _, nestedStmt := range stmt.TopLevelFuncDecl.Body {
			nestedErrors := validateStatementRecursive(nestedStmt, validator, line)
			errors = append(errors, nestedErrors...)
		}
	}

	if stmt.PubTopLevelFuncDecl != nil {
		for _, nestedStmt := range stmt.PubTopLevelFuncDecl.Body {
			nestedErrors := validateStatementRecursive(nestedStmt, validator, line)
			errors = append(errors, nestedErrors...)
		}
	}

	return errors
}
