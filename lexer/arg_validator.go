// By Navid M (c)
// Date: 2025
// License: GPL3
//
// Contains the function signature validation logic for the scar programming language.

package lexer

import (
	"fmt"
	"strings"
)

type FunctionSignature struct {
	Name       string
	Parameters []*MethodParameter
	ReturnType string
	Module     string
}

func preRegisterVars(stmts []*Statement, varTypes map[string]string) {
	for _, stmt := range stmts {
		if stmt == nil {
			continue
		}
		if stmt.VarDecl != nil && stmt.VarDecl.Name != "" {
			varTypes[stmt.VarDecl.Name] = normalizeTypeName(strings.TrimSpace(stmt.VarDecl.Type))
		}
		if stmt.VarDeclInferred != nil && stmt.VarDeclInferred.Name != "" {
			varTypes[stmt.VarDeclInferred.Name] = ""
		}
		if stmt.PubVarDecl != nil && stmt.PubVarDecl.Name != "" {
			varTypes[stmt.PubVarDecl.Name] = normalizeTypeName(strings.TrimSpace(stmt.PubVarDecl.Type))
		}
		if stmt.ObjectDecl != nil && stmt.ObjectDecl.Name != "" {
			varTypes[stmt.ObjectDecl.Name] = normalizeTypeName(strings.TrimSpace(stmt.ObjectDecl.Type))
		}
		if stmt.ListDecl != nil && stmt.ListDecl.Name != "" {
			et := strings.TrimSpace(stmt.ListDecl.Type)
			if et == "" {
				varTypes[stmt.ListDecl.Name] = "list[]"
			} else {
				varTypes[stmt.ListDecl.Name] = "list[" + et + "]"
			}
		}
		if stmt.ListOfDecl != nil && stmt.ListOfDecl.Name != "" {
			varTypes[stmt.ListOfDecl.Name] = strings.TrimSpace(stmt.ListOfDecl.Type)
		}
		if stmt.MapDecl != nil && stmt.MapDecl.Name != "" {
			key := strings.TrimSpace(stmt.MapDecl.KeyType)
			val := strings.TrimSpace(stmt.MapDecl.ValueType)
			if key == "" || val == "" {
				varTypes[stmt.MapDecl.Name] = "map[]"
			} else {
				varTypes[stmt.MapDecl.Name] = "map[" + key + ":" + val + "]"
			}
		}
		if stmt.CatString != nil && stmt.CatString.Target != "" {
			varTypes[stmt.CatString.Target] = "string"
		}
		if stmt.CatList != nil && stmt.CatList.Target != "" {
			if _, ok := varTypes[stmt.CatList.Target]; !ok {
				varTypes[stmt.CatList.Target] = "list[]"
			}
		}
		if stmt.VarDeclRead != nil && stmt.VarDeclRead.Name != "" {
			varTypes[stmt.VarDeclRead.Name] = normalizeTypeName(strings.TrimSpace(stmt.VarDeclRead.Type))
		}
		if stmt.Run != nil {
			s := strings.TrimSpace(stmt.Run.FunctionCall)
			if eq := strings.Index(s, "="); eq != -1 {
				lhs := strings.TrimSpace(s[:eq])
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
		}
		if stmt.For != nil {
			if stmt.For.Var != "" {
				varTypes[stmt.For.Var] = "int"
			}
			preRegisterVars(stmt.For.Body, varTypes)
		}
		if stmt.ReverseFor != nil {
			if stmt.ReverseFor.Var != "" {
				varTypes[stmt.ReverseFor.Var] = "int"
			}
			preRegisterVars(stmt.ReverseFor.Body, varTypes)
		}
		if stmt.VerboseFor != nil {
			if stmt.VerboseFor.VarName != "" {
				t := strings.TrimSpace(stmt.VerboseFor.VarType)
				if t == "" {
					t = "int"
				}
				varTypes[stmt.VerboseFor.VarName] = normalizeTypeName(t)
			}
			preRegisterVars(stmt.VerboseFor.Body, varTypes)
		}
		if stmt.Foreach != nil {
			if stmt.Foreach.VarName != "" {
				t := strings.TrimSpace(stmt.Foreach.VarType)
				if t == "" {
					t = "auto"
				}
				varTypes[stmt.Foreach.VarName] = normalizeTypeName(t)
			}
			preRegisterVars(stmt.Foreach.Body, varTypes)
		}
		if stmt.If != nil {
			preRegisterVars(stmt.If.Body, varTypes)
			for _, e := range stmt.If.ElseIfs {
				preRegisterVars(e.Body, varTypes)
			}
			if stmt.If.Else != nil {
				preRegisterVars(stmt.If.Else.Body, varTypes)
			}
		}
		if stmt.While != nil {
			preRegisterVars(stmt.While.Body, varTypes)
		}
		if stmt.TopLevelFuncDecl != nil {
			preRegisterVars(stmt.TopLevelFuncDecl.Body, varTypes)
		}
		if stmt.PubTopLevelFuncDecl != nil {
			preRegisterVars(stmt.PubTopLevelFuncDecl.Body, varTypes)
		}
		if stmt.ClassDecl != nil {
			for _, m := range stmt.ClassDecl.Methods {
				preRegisterVars(m.Body, varTypes)
			}
		}
		if stmt.PubClassDecl != nil {
			for _, m := range stmt.PubClassDecl.Methods {
				preRegisterVars(m.Body, varTypes)
			}
		}
	}
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

func validateStatementRecursive(stmt *Statement, validator *ArgumentValidator, line int, expectedReturn string, varTypes map[string]string) []error {
	if stmt == nil {
		return nil
	}

	errors := []error{}

	cloneScope := func() map[string]string {
		child := make(map[string]string, len(varTypes))
		for k, v := range varTypes {
			child[k] = v
		}
		return child
	}

	if stmt.VarDecl != nil && stmt.VarDecl.Name != "" {
		varTypes[stmt.VarDecl.Name] = normalizeTypeName(strings.TrimSpace(stmt.VarDecl.Type))
	}
	if stmt.VarDeclInferred != nil && stmt.VarDeclInferred.Name != "" {
		varTypes[stmt.VarDeclInferred.Name] = ""
	}
	if stmt.PubVarDecl != nil && stmt.PubVarDecl.Name != "" {
		varTypes[stmt.PubVarDecl.Name] = normalizeTypeName(strings.TrimSpace(stmt.PubVarDecl.Type))
	}
	if stmt.ListDecl != nil && stmt.ListDecl.Name != "" {
		et := strings.TrimSpace(stmt.ListDecl.Type)
		if et == "" {
			varTypes[stmt.ListDecl.Name] = "list[]"
		} else {
			varTypes[stmt.ListDecl.Name] = et
		}
	}
	if stmt.ListOfDecl != nil && stmt.ListOfDecl.Name != "" {
		varTypes[stmt.ListOfDecl.Name] = strings.TrimSpace(stmt.ListOfDecl.Type)
	}
	if stmt.MapDecl != nil && stmt.MapDecl.Name != "" {
		key := strings.TrimSpace(stmt.MapDecl.KeyType)
		val := strings.TrimSpace(stmt.MapDecl.ValueType)
		if key == "" || val == "" {
			varTypes[stmt.MapDecl.Name] = "map[]"
		} else {
			varTypes[stmt.MapDecl.Name] = "map[" + key + ":" + val + "]"
		}
	}
	if stmt.CatString != nil && stmt.CatString.Target != "" {
		varTypes[stmt.CatString.Target] = "string"
	}
	if stmt.CatList != nil && stmt.CatList.Target != "" {
		if _, ok := varTypes[stmt.CatList.Target]; !ok {
			varTypes[stmt.CatList.Target] = "list[]"
		}
	}
	if stmt.VarDeclRead != nil && stmt.VarDeclRead.Name != "" {
		varTypes[stmt.VarDeclRead.Name] = normalizeTypeName(strings.TrimSpace(stmt.VarDeclRead.Type))
	}

	if stmt.TopLevelFuncDecl != nil {
		child := cloneScope()
		for _, p := range stmt.TopLevelFuncDecl.Parameters {
			if p != nil && p.Name != "" {
				typ := strings.TrimSpace(p.Type)
				if p.IsList && p.ListType != "" {
					typ = "list[" + strings.TrimSpace(p.ListType) + "]"
				}
				child[p.Name] = normalizeTypeName(typ)
			}
		}
		for _, nested := range stmt.TopLevelFuncDecl.Body {
			nestedErrors := validateStatementRecursive(nested, validator, line, stmt.TopLevelFuncDecl.ReturnType, child)
			errors = append(errors, nestedErrors...)
		}
	}

	if stmt.PubTopLevelFuncDecl != nil {
		child := cloneScope()
		for _, p := range stmt.PubTopLevelFuncDecl.Parameters {
			if p != nil && p.Name != "" {
				typ := strings.TrimSpace(p.Type)
				if p.IsList && p.ListType != "" {
					typ = "list[" + strings.TrimSpace(p.ListType) + "]"
				}
				child[p.Name] = normalizeTypeName(typ)
			}
		}
		for _, nested := range stmt.PubTopLevelFuncDecl.Body {
			nestedErrors := validateStatementRecursive(nested, validator, line, stmt.PubTopLevelFuncDecl.ReturnType, child)
			errors = append(errors, nestedErrors...)
		}
	}

	if stmt.ClassDecl != nil {
		for _, m := range stmt.ClassDecl.Methods {
			child := cloneScope()
			for _, p := range m.Parameters {
				if p != nil && p.Name != "" {
					typ := strings.TrimSpace(p.Type)
					if p.IsList && p.ListType != "" {
						typ = "list[" + strings.TrimSpace(p.ListType) + "]"
					}
					child[p.Name] = normalizeTypeName(typ)
				}
			}
			for _, nestedStmt := range m.Body {
				nestedErrors := validateStatementRecursive(nestedStmt, validator, line, m.ReturnType, child)
				errors = append(errors, nestedErrors...)
			}
		}
	}

	if stmt.PubClassDecl != nil {
		for _, m := range stmt.PubClassDecl.Methods {
			child := cloneScope()
			for _, p := range m.Parameters {
				if p != nil && p.Name != "" {
					typ := strings.TrimSpace(p.Type)
					if p.IsList && p.ListType != "" {
						typ = "list[" + strings.TrimSpace(p.ListType) + "]"
					}
					child[p.Name] = normalizeTypeName(typ)
				}
			}
			for _, nestedStmt := range m.Body {
				nestedErrors := validateStatementRecursive(nestedStmt, validator, line, m.ReturnType, child)
				errors = append(errors, nestedErrors...)
			}
		}
	}

	if stmt.For != nil {
		child := cloneScope()
		if stmt.For.Var != "" {
			child[stmt.For.Var] = "int"
		}
		for _, nested := range stmt.For.Body {
			nestedErrors := validateStatementRecursive(nested, validator, line, expectedReturn, child)
			errors = append(errors, nestedErrors...)
		}
	}

	if stmt.ReverseFor != nil {
		child := cloneScope()
		if stmt.ReverseFor.Var != "" {
			child[stmt.ReverseFor.Var] = "int"
		}
		for _, nested := range stmt.ReverseFor.Body {
			nestedErrors := validateStatementRecursive(nested, validator, line, expectedReturn, child)
			errors = append(errors, nestedErrors...)
		}
	}

	if stmt.VerboseFor != nil {
		child := cloneScope()
		if stmt.VerboseFor.VarName != "" {
			t := strings.TrimSpace(stmt.VerboseFor.VarType)
			if t == "" {
				t = "int"
			}
			child[stmt.VerboseFor.VarName] = normalizeTypeName(t)
		}
		for _, nested := range stmt.VerboseFor.Body {
			nestedErrors := validateStatementRecursive(nested, validator, line, expectedReturn, child)
			errors = append(errors, nestedErrors...)
		}
	}

	if stmt.Foreach != nil {
		child := cloneScope()
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

	if stmt.While != nil {
		child := cloneScope()
		for _, nested := range stmt.While.Body {
			nestedErrors := validateStatementRecursive(nested, validator, line, expectedReturn, child)
			errors = append(errors, nestedErrors...)
		}
	}

	if stmt.If != nil {
		child := cloneScope()
		for _, nested := range stmt.If.Body {
			nestedErrors := validateStatementRecursive(nested, validator, line, expectedReturn, child)
			errors = append(errors, nestedErrors...)
		}
		for _, e := range stmt.If.ElseIfs {
			for _, nested := range e.Body {
				nestedErrors := validateStatementRecursive(nested, validator, line, expectedReturn, child)
				errors = append(errors, nestedErrors...)
			}
		}
		if stmt.If.Else != nil {
			for _, nested := range stmt.If.Else.Body {
				nestedErrors := validateStatementRecursive(nested, validator, line, expectedReturn, child)
				errors = append(errors, nestedErrors...)
			}
		}
	}

	return errors
}

func ValidateProgram(program *Program) []error {
	if program == nil {
		return []error{fmt.Errorf("nil program")}
	}
	validator := NewArgumentValidator()
	varTypes := make(map[string]string)
	preRegisterVars(program.Statements, varTypes)

	var allErrors []error
	line := 1
	for _, stmt := range program.Statements {
		errs := validateStatementRecursive(stmt, validator, line, "", varTypes)
		allErrors = append(allErrors, errs...)
		line++
	}
	return allErrors
}

func findFirstParenOutsideString(expr string) int {
	inString := false
	esc := false
	var quote byte
	for i := 0; i < len(expr); i++ {
		ch := expr[i]
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
