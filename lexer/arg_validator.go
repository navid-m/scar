// By Navid M (c)
// Date: 2025
// License: GPL3
//
// Contains the function signature validation logic for the scar programming language.

package lexer

import (
	"fmt"
	"slices"
	"strings"
)

type FunctionSignature struct {
	Name       string
	Parameters []*MethodParameter
	ReturnType string
	Module     string
}

// Converts C-style or runtime type names to Scar source type names
//
// Examples:
//   - "char*" -> "string"
//   - "lstring" -> "string"
//   - "ref math::Vector2" -> "math::Vector2"
//   - "math_Vector3" -> "math::Vector3"
func normalizeTypeName(t string) string {
	s := strings.TrimSpace(t)
	if s == "" {
		return s
	}
	if after, ok := strings.CutPrefix(s, "ref "); ok {
		s = strings.TrimSpace(after)
	}
	switch s {
	case "char*", "lstring":
		return "string"
	}
	if strings.Contains(s, "::") {
		return s
	}
	if idx := strings.Index(s, "_"); idx > 0 {
		mod := s[:idx]
		rest := s[idx+1:]
		if mod != "" && rest != "" {
			return mod + "::" + rest
		}
	}
	return s
}

func tryInferNewType(expr string) (string, bool) {
	s := strings.TrimSpace(expr)
	if s == "" {
		return "", false
	}
	if after, ok := strings.CutPrefix(s, "new "); ok {
		after = strings.TrimSpace(after)
		if idx := strings.Index(after, "("); idx > 0 {
			typ := strings.TrimSpace(after[:idx])
			if typ != "" {
				return typ, true
			}
		}
	}
	return "", false
}

func (av *ArgumentValidator) inferReturnTypeWithSymbols(expr string, varTypes map[string]string) (string, bool) {
	s := strings.TrimSpace(expr)
	if s == "" || !strings.Contains(s, "(") {
		return "", false
	}
	open := strings.Index(s, "(")
	close := findMatchingParen(s, open)
	if close == -1 {
		return "", false
	}
	callee := strings.TrimSpace(s[:open])
	if isValidType(callee) {
		return callee, true
	}
	if dot := strings.LastIndex(callee, "."); dot != -1 {
		obj := strings.TrimSpace(callee[:dot])
		meth := strings.TrimSpace(callee[dot+1:])
		if objType, ok := varTypes[obj]; ok {
			if after, ok0 := strings.CutPrefix(objType, "ref "); ok0 {
				objType = strings.TrimSpace(after)
			}
			objType = normalizeTypeName(objType)
			if sig, ok := av.functions[objType+"."+meth]; ok {
				return sig.ReturnType, true
			}
			if strings.Contains(objType, "::") {
				if sig, ok := av.functions[objType+"."+meth]; ok {
					return sig.ReturnType, true
				}
				parts := strings.Split(objType, "::")
				base := parts[len(parts)-1]
				if sig, ok := av.functions[base+"."+meth]; ok {
					return sig.ReturnType, true
				}
			}
		}
		return "", false
	}
	return av.inferReturnType(s)
}

func (av *ArgumentValidator) RegisterMethod(
	className string,
	name string,
	params []*MethodParameter,
	returnType string,
	module string,
) {
	if className != "" {
		key := className + "." + name
		sig := &FunctionSignature{Name: key, Parameters: params, ReturnType: returnType, Module: module}
		av.functions[key] = sig
		if module != "" {
			modKey := module + "::" + className + "." + name
			av.functions[modKey] = sig
			usKey := module + "_" + className + "_" + name
			av.functions[usKey] = sig
		}
		av.functions[className+"_"+name] = sig
	}
}

type ArgumentValidator struct {
	functions map[string]*FunctionSignature
}

func NewArgumentValidator() *ArgumentValidator {
	av := &ArgumentValidator{
		functions: make(map[string]*FunctionSignature),
	}

	builtins := []string{"fmt!", "cat!", "has!", "put!", "len"}
	for _, builtin := range builtins {
		ret := "void"
		switch builtin {
		case "fmt!":
			ret = "string"
		case "len":
			ret = "int"
		}
		av.functions[builtin] = &FunctionSignature{
			Name:       builtin,
			Parameters: []*MethodParameter{},
			ReturnType: ret,
			Module:     "",
		}
	}

	return av
}

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

func isBuiltinFunction(name string) bool {
	builtins := []string{"fmt!", "cat!", "has!", "put!", "len"}
	for _, builtin := range builtins {
		if name == builtin {
			return true
		}
	}
	return false
}

func isMethodCall(expr string) bool {
	s := strings.TrimSpace(expr)
	parenIdx := strings.Index(s, "(")
	if parenIdx == -1 {
		return false
	}
	beforeParen := s[:parenIdx]
	return strings.Contains(beforeParen, ".")
}

func isNewExpression(expr string) bool {
	s := strings.TrimSpace(expr)
	return strings.HasPrefix(s, "new ")
}

func (av *ArgumentValidator) ValidateFunctionCall(funcCall *FunctionCallStmt, line int) error {
	var (
		funcName          = funcCall.Name
		signature, exists = av.functions[funcName]
	)

	if isBuiltinFunction(funcName) {
		return nil
	}

	if strings.Contains(funcName, ".") {
		return nil
	}

	if strings.HasPrefix(funcName, "new ") {
		return nil
	}

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
		return fmt.Errorf("line %d: function '%s' is not defined", line, funcName)
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

	for _, arg := range funcCall.Args {
		if err := av.ValidateStringFunctionCall(arg, line); err != nil {
			return err
		}
	}

	return nil
}

// Infers the return type of a simple call expression.
//
// Supports:
//   - func(args)
//   - module::func(args)
//   - obj.method(args) [uses method name only]
//   - type casts like float(expr) -> returns "float"
func (av *ArgumentValidator) inferReturnType(expr string) (string, bool) {
	s := strings.TrimSpace(expr)
	if s == "" {
		return "", false
	}
	open := strings.Index(s, "(")
	if open == -1 {
		return "", false
	}
	close := findMatchingParen(s, open)
	if close == -1 {
		return "", false
	}
	before := strings.TrimSpace(s[:open])
	after := strings.TrimSpace(s[close+1:])
	if before == "" || after != "" {
		return "", false
	}
	callee := before
	if isValidType(callee) {
		return callee, true
	}
	if strings.Contains(callee, ".") {
		parts := strings.Split(callee, ".")
		callee = parts[len(parts)-1]
	}
	if sig, ok := av.functions[callee]; ok {
		return sig.ReturnType, true
	}
	for key, sig := range av.functions {
		if key == callee {
			return sig.ReturnType, true
		}
		if strings.Contains(key, "::") {
			kp := strings.Split(key, "::")
			if len(kp) == 2 && kp[1] == callee {
				return sig.ReturnType, true
			}
		}
	}
	return "", false
}

// TODO: Implement method call validation
func (av *ArgumentValidator) ValidateMethodCall(methodCall *MethodCallStmt, line int) error {
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
	if containsAlias(expr) {
		return nil
	}

	if isNewExpression(expr) {
		return nil
	}

	if isMethodCall(expr) {
		return nil
	}

	if !strings.Contains(expr, "(") || !strings.Contains(expr, ")") {
		return nil
	}
	parenStart := findFirstParenOutsideString(expr)
	if parenStart == -1 {
		return nil
	}
	parenEnd := findMatchingParen(expr, parenStart)
	if parenEnd == -1 || parenEnd < parenStart {
		return nil
	}
	funcName := strings.TrimSpace(expr[:parenStart])

	if isBuiltinFunction(funcName) {
		return nil
	}

	// Treat constructor-like calls (Type() or mod::Type()) as valid and skip
	last := funcName
	if strings.Contains(funcName, "::") {
		parts := strings.Split(funcName, "::")
		last = parts[len(parts)-1]
	}
	if last != "" {
		first := last[0]
		if (first >= 'A' && first <= 'Z') || isValidType(last) {
			return nil
		}
	}

	argsStr := strings.TrimSpace(expr[parenStart+1 : parenEnd])
	args := parseArguments(argsStr)
	tempCall := &FunctionCallStmt{
		Name: funcName,
		Args: args,
	}
	if err := av.ValidateFunctionCall(tempCall, line); err != nil {
		return err
	}
	for _, arg := range args {
		if err := av.ValidateStringFunctionCall(arg, line); err != nil {
			return err
		}
	}

	return nil
}

func parseArguments(argsStr string) []string {
	if strings.TrimSpace(argsStr) == "" {
		return []string{}
	}

	var (
		args            []string
		current         strings.Builder
		parenDepth      = 0
		inString        = false
		stringDelimiter byte
		escapeNext      = false
	)

	for i := 0; i < len(argsStr); i++ {
		ch := argsStr[i]

		if inString {
			if escapeNext {
				escapeNext = false
				current.WriteByte(ch)
				continue
			}
			if ch == '\\' {
				escapeNext = true
				current.WriteByte(ch)
				continue
			}
			if ch == stringDelimiter {
				inString = false
				current.WriteByte(ch)
				continue
			}
			current.WriteByte(ch)
			continue
		}

		switch ch {
		case '"', '\'':
			inString = true
			stringDelimiter = ch
			current.WriteByte(ch)
		case '(':
			parenDepth++
			current.WriteByte(ch)
		case ')':
			parenDepth--
			current.WriteByte(ch)
		case ',':
			if parenDepth == 0 {
				args = append(args, strings.TrimSpace(current.String()))
				current.Reset()
			} else {
				current.WriteByte(ch)
			}
		default:
			current.WriteByte(ch)
		}
	}

	if current.Len() > 0 {
		args = append(args, strings.TrimSpace(current.String()))
	}

	return args
}

func scanUnknownVars(expr string, varTypes map[string]string) []string {
	s := strings.TrimSpace(expr)
	if s == "" {
		return nil
	}

	if containsAlias(s) {
		return nil
	}

	if strings.Contains(s, "=") && !strings.Contains(s, "==") && !strings.Contains(s, "!=") && !strings.Contains(s, "<=") && !strings.Contains(s, ">=") {
		return nil
	}

	var (
		unknown  []string
		inString bool
		esc      bool
		quote    byte
		i        int
	)
	isIdentStart := func(ch byte) bool { return (ch == '_') || (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') }
	isIdent := func(ch byte) bool { return isIdentStart(ch) || (ch >= '0' && ch <= '9') }
	for i < len(s) {
		ch := s[i]
		if inString {
			if esc {
				esc = false
				i++
				continue
			}
			if ch == '\\' {
				esc = true
				i++
				continue
			}
			if ch == quote {
				inString = false
				i++
				continue
			}
			i++
			continue
		}
		if ch == '"' || ch == '\'' {
			inString = true
			quote = ch
			i++
			continue
		}
		if isIdentStart(ch) {
			start := i
			i++
			for i < len(s) && isIdent(s[i]) {
				i++
			}
			baseTok := s[start:i]

			if i < len(s) && s[i] == '.' {
				for i < len(s) && (isIdent(s[i]) || s[i] == '.') {
					i++
				}

				j := i
				for j < len(s) && s[j] == ' ' {
					j++
				}
				if j < len(s) && s[j] == '(' {
					continue
				} // function call

				if _, ok := varTypes[baseTok]; ok {
					continue
				}

				if !slices.Contains(unknown, baseTok) {
					unknown = append(unknown, baseTok)
				}
				continue
			}

			tok := baseTok

			j := i
			for j < len(s) && s[j] == ' ' {
				j++
			}
			if j < len(s) && s[j] == '(' {
				continue
			} // function call
			jj := j
			if jj < len(s) && s[jj] == '!' {
				jj++
				for jj < len(s) && s[jj] == ' ' {
					jj++
				}
				if jj < len(s) && s[jj] == '(' {
					continue
				}
			}
			if j+1 < len(s) && s[j] == ':' && s[j+1] == ':' {
				continue
			}
			if tok == "new" {
				continue
			}
			if tok == "or" || tok == "and" || tok == "not" {
				continue
			}
			if tok == "true" || tok == "false" {
				continue
			}
			if isValidType(tok) {
				continue
			}
			if _, ok := varTypes[tok]; ok {
				continue
			}

			if !slices.Contains(unknown, tok) {
				unknown = append(unknown, tok)
			}
			continue
		}
		i++
	}
	return unknown
}

func findFirstParenOutsideString(s string) int {
	inString := false
	esc := false
	var quote byte
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inString {
			if esc {
				esc = false
				continue
			}
			if ch == '\\' {
				esc = true
				continue
			}
			if ch == quote {
				inString = false
				continue
			}
			continue
		}
		if ch == '"' || ch == '\'' {
			inString = true
			quote = ch
			continue
		}
		if ch == '(' {
			return i
		}
	}
	return -1
}

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
			if stmt.TopLevelFuncDecl.Name == "main" {
				errors = append(errors, fmt.Errorf("function name 'main' is reserved and cannot be used. use top-level statements."))
			}
			if stmt.TopLevelFuncDecl.Name == "min" {
				errors = append(errors, fmt.Errorf("function name 'min' is reserved and cannot be used."))
			}
			validator.RegisterFunction(
				stmt.TopLevelFuncDecl.Name,
				stmt.TopLevelFuncDecl.Parameters,
				stmt.TopLevelFuncDecl.ReturnType,
				"",
			)
		}
		if stmt.PubTopLevelFuncDecl != nil {
			if stmt.PubTopLevelFuncDecl.Name == "main" {
				errors = append(errors, fmt.Errorf("function name 'main' is reserved and cannot be used. use top-level statements."))
			}
			if stmt.PubTopLevelFuncDecl.Name == "min" {
				errors = append(errors, fmt.Errorf("function name 'min' is reserved and cannot be used."))
			}
			validator.RegisterFunction(
				stmt.PubTopLevelFuncDecl.Name,
				stmt.PubTopLevelFuncDecl.Parameters,
				stmt.PubTopLevelFuncDecl.ReturnType,
				"",
			)
		}
		if stmt.ClassDecl != nil {
			for _, m := range stmt.ClassDecl.Methods {
				validator.RegisterMethod(stmt.ClassDecl.Name, m.Name, m.Parameters, m.ReturnType, "")
			}
		}
		if stmt.PubClassDecl != nil {
			for _, m := range stmt.PubClassDecl.Methods {
				validator.RegisterMethod(stmt.PubClassDecl.Name, m.Name, m.Parameters, m.ReturnType, "")
			}
		}
	}

	for _, module := range LoadedModules {
		for funcName, funcDecl := range module.PublicFuncs {
			validator.RegisterFunction(funcName, funcDecl.Parameters, funcDecl.ReturnType, module.Name)
		}
		for className, classDecl := range module.PublicClasses {
			for _, m := range classDecl.Methods {
				validator.RegisterMethod(className, m.Name, m.Parameters, m.ReturnType, module.Name)
			}
		}
	}
	lineNum := 1
	var varTypes = make(map[string]string)
	for _, stmt := range program.Statements {
		stmtErrors := validateStatementRecursive(stmt, validator, lineNum, "", varTypes)
		errors = append(errors, stmtErrors...)
		lineNum++
	}

	seen := make(map[string]bool)
	var dedup []error
	for _, err := range errors {
		if err == nil {
			continue
		}
		msg := err.Error()
		if !seen[msg] {
			seen[msg] = true
			dedup = append(dedup, err)
		}
	}

	return dedup
}

// Validates function calls in a statement and its nested statements
func validateStatementRecursive(stmt *Statement, validator *ArgumentValidator, line int, expectedReturn string, varTypes map[string]string) []error {
	var errors []error

	if stmt.FunctionCall != nil {
		if err := validator.ValidateFunctionCall(stmt.FunctionCall, line); err != nil {
			errors = append(errors, err)
		}
	} else if stmt.MethodCall != nil {
		if err := validator.ValidateMethodCall(stmt.MethodCall, line); err != nil {
			errors = append(errors, err)
		}
	} else if stmt.VarDecl != nil {
		if stmt.VarDecl.Value != "" {
			if err := validator.ValidateStringFunctionCall(stmt.VarDecl.Value, line); err != nil {
				errors = append(errors, err)
			}
			for _, u := range scanUnknownVars(stmt.VarDecl.Value, varTypes) {
				errors = append(errors, fmt.Errorf("line %d: variable '%s' is not defined", line, u))
			}
			if newTyp, ok := tryInferNewType(stmt.VarDecl.Value); ok {
				declared := normalizeTypeName(stmt.VarDecl.Type)
				inst := normalizeTypeName(newTyp)
				if declared != "" && declared != inst {
					errors = append(errors, fmt.Errorf("line %d: cannot assign 'new %s' to variable '%s' of type '%s'", line, newTyp, stmt.VarDecl.Name, stmt.VarDecl.Type))
				}
			} else if rt, ok := validator.inferReturnTypeWithSymbols(stmt.VarDecl.Value, varTypes); ok && rt != "" {
				declared := normalizeTypeName(stmt.VarDecl.Type)
				inferred := normalizeTypeName(rt)
				if declared != "" && declared != inferred {
					errors = append(errors, fmt.Errorf("line %d: cannot assign call returning '%s' to variable '%s' of type '%s'", line, rt, stmt.VarDecl.Name, stmt.VarDecl.Type))
				}
			}
		}
		if stmt.VarDecl.Name != "" && stmt.VarDecl.Type != "" {
			normalizedType := normalizeTypeName(stmt.VarDecl.Type)
			varTypes[stmt.VarDecl.Name] = normalizedType
		}
	} else if stmt.ListDecl != nil {
		if stmt.ListDecl.Name != "" {
			elemType := strings.TrimSpace(stmt.ListDecl.Type)
			t := "list[" + elemType + "]"
			if elemType == "" {
				t = "list[]"
			}
			varTypes[stmt.ListDecl.Name] = t
		}
	} else if stmt.ListOfDecl != nil {
		if stmt.ListOfDecl.Name != "" {
			varTypes[stmt.ListOfDecl.Name] = strings.TrimSpace(stmt.ListOfDecl.Type)
		}
	} else if stmt.MapDecl != nil {
		if stmt.MapDecl.Name != "" {
			key := strings.TrimSpace(stmt.MapDecl.KeyType)
			val := strings.TrimSpace(stmt.MapDecl.ValueType)
			t := "map[" + key + ":" + val + "]"
			if key == "" || val == "" {
				t = "map[]"
			}
			varTypes[stmt.MapDecl.Name] = t
		}
	} else if stmt.VarDeclInferred != nil {
		if stmt.VarDeclInferred.Name != "" {
			varTypes[stmt.VarDeclInferred.Name] = ""
		}
	} else if stmt.PubVarDecl != nil {
		if stmt.PubVarDecl.Name != "" {
			varTypes[stmt.PubVarDecl.Name] = normalizeTypeName(stmt.PubVarDecl.Type)
		}
	} else if stmt.Run != nil {
		s := strings.TrimSpace(stmt.Run.FunctionCall)
		if eq := strings.Index(s, "="); eq != -1 {
			lhs := strings.TrimSpace(s[:eq])
			// LHS may look like: "void* loaded" or "auto loaded" or just "loaded"
			fields := strings.Fields(lhs)
			if len(fields) > 0 {
				name := fields[len(fields)-1]
				if name != "" {
					if _, ok := varTypes[name]; !ok {
						varTypes[name] = ""
					}
				}
			}
		}
	} else if stmt.VarAssign != nil {
		if err := validator.ValidateStringFunctionCall(stmt.VarAssign.Value, line); err != nil {
			errors = append(errors, err)
		}
		if stmt.VarAssign.Name != "" {
			target := strings.TrimSpace(stmt.VarAssign.Name)
			if dot := strings.Index(target, "."); dot != -1 {
				base := strings.TrimSpace(target[:dot])
				if _, ok := varTypes[base]; !ok {
					errors = append(errors, fmt.Errorf("line %d: variable '%s' is not defined", line, base))
				}
			} else {
				if _, ok := varTypes[target]; !ok {
					errors = append(errors, fmt.Errorf("line %d: variable '%s' is not defined", line, target))
				}
			}
		}
		for _, u := range scanUnknownVars(stmt.VarAssign.Value, varTypes) {
			errors = append(errors, fmt.Errorf("line %d: variable '%s' is not defined", line, u))
		}
	} else if stmt.ObjectDecl != nil {
		if stmt.ObjectDecl.Name != "" && stmt.ObjectDecl.Type != "" {
			normalizedType := normalizeTypeName(stmt.ObjectDecl.Type)
			varTypes[stmt.ObjectDecl.Name] = normalizedType
		}
	}
	if stmt.Print != nil {
		for _, variable := range stmt.Print.Variables {
			if err := validator.ValidateStringFunctionCall(variable, line); err != nil {
				errors = append(errors, err)
			}
			for _, u := range scanUnknownVars(variable, varTypes) {
				errors = append(errors, fmt.Errorf("line %d: variable '%s' is not defined", line, u))
			}
		}
	}

	if stmt.Return != nil {
		if err := validator.ValidateStringFunctionCall(stmt.Return.Value, line); err != nil {
			errors = append(errors, err)
		}
		for _, u := range scanUnknownVars(stmt.Return.Value, varTypes) {
			errors = append(errors, fmt.Errorf("line %d: variable '%s' is not defined", line, u))
		}
		if expectedReturn != "" {
			if rt, ok := validator.inferReturnTypeWithSymbols(stmt.Return.Value, varTypes); ok && rt != "" {
				exp := normalizeTypeName(expectedReturn)
				inferred := normalizeTypeName(rt)
				if inferred != exp {
					errors = append(errors, fmt.Errorf("line %d: return type mismatch: expected '%s' but expression returns '%s'", line, expectedReturn, rt))
				}
			}
		}
	}
	if stmt.If != nil {
		for _, nestedStmt := range stmt.If.Body {
			nestedErrors := validateStatementRecursive(nestedStmt, validator, line, expectedReturn, varTypes)
			errors = append(errors, nestedErrors...)
		}
		for _, elif := range stmt.If.ElseIfs {
			for _, nestedStmt := range elif.Body {
				nestedErrors := validateStatementRecursive(nestedStmt, validator, line, expectedReturn, varTypes)
				errors = append(errors, nestedErrors...)
			}
		}
		if stmt.If.Else != nil {
			for _, nestedStmt := range stmt.If.Else.Body {
				nestedErrors := validateStatementRecursive(nestedStmt, validator, line, expectedReturn, varTypes)
				errors = append(errors, nestedErrors...)
			}
		}
	}

	if stmt.While != nil {
		for _, nestedStmt := range stmt.While.Body {
			nestedErrors := validateStatementRecursive(nestedStmt, validator, line, expectedReturn, varTypes)
			errors = append(errors, nestedErrors...)
		}
	}

	if stmt.For != nil {
		child := make(map[string]string, len(varTypes)+1)
		for k, v := range varTypes {
			child[k] = v
		}
		if stmt.For.Var != "" {
			child[stmt.For.Var] = "int"
		}
		for _, nestedStmt := range stmt.For.Body {
			nestedErrors := validateStatementRecursive(nestedStmt, validator, line, expectedReturn, child)
			errors = append(errors, nestedErrors...)
		}
	}

	if stmt.ReverseFor != nil {
		child := make(map[string]string, len(varTypes)+1)
		for k, v := range varTypes {
			child[k] = v
		}
		if stmt.ReverseFor.Var != "" {
			child[stmt.ReverseFor.Var] = "int"
		}
		for _, nestedStmt := range stmt.ReverseFor.Body {
			nestedErrors := validateStatementRecursive(nestedStmt, validator, line, expectedReturn, child)
			errors = append(errors, nestedErrors...)
		}
	}

	if stmt.VerboseFor != nil {
		child := make(map[string]string, len(varTypes)+1)
		for k, v := range varTypes {
			child[k] = v
		}
		if stmt.VerboseFor.VarName != "" {
			t := strings.TrimSpace(stmt.VerboseFor.VarType)
			if t == "" {
				t = "int"
			}
			child[stmt.VerboseFor.VarName] = normalizeTypeName(t)
		}
		for _, nestedStmt := range stmt.VerboseFor.Body {
			nestedErrors := validateStatementRecursive(nestedStmt, validator, line, expectedReturn, child)
			errors = append(errors, nestedErrors...)
		}
	}

	if stmt.TopLevelFuncDecl != nil {
		for _, nestedStmt := range stmt.TopLevelFuncDecl.Body {
			// Child scope with parameters
			child := make(map[string]string, len(varTypes)+len(stmt.TopLevelFuncDecl.Parameters))
			for k, v := range varTypes {
				child[k] = v
			}
			for _, p := range stmt.TopLevelFuncDecl.Parameters {
				if p != nil && p.Name != "" {
					typ := strings.TrimSpace(p.Type)
					if p.IsList && p.ListType != "" {
						typ = "list[" + strings.TrimSpace(p.ListType) + "]"
					}
					child[p.Name] = normalizeTypeName(typ)
				}
			}
			nestedErrors := validateStatementRecursive(nestedStmt, validator, line, stmt.TopLevelFuncDecl.ReturnType, child)
			errors = append(errors, nestedErrors...)
		}
	}

	if stmt.PubTopLevelFuncDecl != nil {
		for _, nestedStmt := range stmt.PubTopLevelFuncDecl.Body {
			child := make(map[string]string, len(varTypes)+len(stmt.PubTopLevelFuncDecl.Parameters))
			for k, v := range varTypes {
				child[k] = v
			}
			for _, p := range stmt.PubTopLevelFuncDecl.Parameters {
				if p != nil && p.Name != "" {
					typ := strings.TrimSpace(p.Type)
					if p.IsList && p.ListType != "" {
						typ = "list[" + strings.TrimSpace(p.ListType) + "]"
					}
					child[p.Name] = normalizeTypeName(typ)
				}
			}
			nestedErrors := validateStatementRecursive(nestedStmt, validator, line, stmt.PubTopLevelFuncDecl.ReturnType, child)
			errors = append(errors, nestedErrors...)
		}
	}

	if stmt.ClassDecl != nil {
		for _, m := range stmt.ClassDecl.Methods {
			for _, nestedStmt := range m.Body {
				// Child scope with method parameters
				child := make(map[string]string, len(varTypes)+len(m.Parameters))
				for k, v := range varTypes {
					child[k] = v
				}
				for _, p := range m.Parameters {
					if p != nil && p.Name != "" {
						typ := strings.TrimSpace(p.Type)
						if p.IsList && p.ListType != "" {
							typ = "list[" + strings.TrimSpace(p.ListType) + "]"
						}
						child[p.Name] = normalizeTypeName(typ)
					}
				}
				nestedErrors := validateStatementRecursive(nestedStmt, validator, line, m.ReturnType, child)
				errors = append(errors, nestedErrors...)
			}
		}
	}

	if stmt.PubClassDecl != nil {
		for _, m := range stmt.PubClassDecl.Methods {
			for _, nestedStmt := range m.Body {
				child := make(map[string]string, len(varTypes)+len(m.Parameters))
				for k, v := range varTypes {
					child[k] = v
				}
				for _, p := range m.Parameters {
					if p != nil && p.Name != "" {
						typ := strings.TrimSpace(p.Type)
						if p.IsList && p.ListType != "" {
							typ = "list[" + strings.TrimSpace(p.ListType) + "]"
						}
						child[p.Name] = normalizeTypeName(typ)
					}
				}
				nestedErrors := validateStatementRecursive(nestedStmt, validator, line, m.ReturnType, child)
				errors = append(errors, nestedErrors...)
			}
		}
	}

	if stmt.Foreach != nil {
		child := make(map[string]string, len(varTypes)+1)
		for k, v := range varTypes {
			child[k] = v
		}
		if stmt.Foreach.VarName != "" {
			t := strings.TrimSpace(stmt.Foreach.VarType)
			if t == "" {
				t = "auto"
			}
			child[stmt.Foreach.VarName] = normalizeTypeName(t)
		}
		for _, nestedStmt := range stmt.Foreach.Body {
			nestedErrors := validateStatementRecursive(nestedStmt, validator, line, expectedReturn, child)
			errors = append(errors, nestedErrors...)
		}
	}

	return errors
}

func containsAlias(expr string) bool {
	s := strings.TrimSpace(expr)
	if s == "" {
		return false
	}
	for k, v := range Aliases {
		if k != "" && strings.Contains(s, k) {
			return true
		}
		if v != "" && strings.Contains(s, v) {
			return true
		}
	}
	for k, v := range UnsafeAliases {
		if k != "" && strings.Contains(s, k) {
			return true
		}
		if v != "" && strings.Contains(s, v) {
			return true
		}
	}
	return false
}
