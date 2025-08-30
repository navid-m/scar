// By Navid M (c)
// Date: 2025
// License: GPL3
//
// Contains the function signature validation logic for the scar programming language.

package lexer

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"
)

type FunctionSignature struct {
	Name       string
	Parameters []*MethodParameter
	ReturnType string
	Module     string
}

func isBuiltinFunction(name string) bool {
	switch strings.TrimSpace(name) {
	case "fmt!", "cat!", "has!", "put!", "len":
		return true
	default:
		return false
	}
}

func resolveReceiverType(object string, varTypes map[string]string) (string, bool) {
	s := strings.TrimSpace(object)
	if s == "" {
		return "", false
	}
	if strings.ContainsAny(s, ".() ") {
		return "", false
	}
	if t, ok := varTypes[s]; ok {
		t = normalizeTypeName(strings.TrimSpace(t))
		if after, has := strings.CutPrefix(t, "ref "); has {
			t = strings.TrimSpace(after)
		}
		if t == "" {
			return "", false
		}
		if strings.Contains(t, "::") {
			parts := strings.Split(t, "::")
			t = parts[len(parts)-1]
		}
		return t, true
	}
	if strings.Contains(s, "::") {
		parts := strings.Split(s, "::")
		cls := parts[len(parts)-1]
		if cls != "" {
			return cls, true
		}
	}
	if len(s) > 0 {
		c := s[0]
		if c >= 'A' && c <= 'Z' {
			return s, true
		}
	}
	return "", false
}

func isNewExpression(expr string) bool {
	return strings.HasPrefix(strings.TrimSpace(expr), "new ")
}

func isMethodCall(expr string) bool {
	s := strings.TrimSpace(expr)
	if !strings.Contains(s, "(") {
		return false
	}
	open := findFirstParenOutsideString(s)
	if open == -1 {
		return false
	}
	head := strings.TrimSpace(s[:open])
	return strings.Contains(head, ".")
}

func findMatchingParen(s string, open int) int {
	depth := 0
	inString := false
	var q byte
	escape := false
	for i := open; i < len(s); i++ {
		ch := s[i]
		if inString {
			if escape {
				escape = false
				continue
			}
			if ch == '\\' {
				escape = true
				continue
			}
			if ch == q {
				inString = false
			}
			continue
		}
		switch ch {
		case '\'', '"':
			inString = true
			q = ch
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

type ArgumentValidator struct {
	functions     map[string]*FunctionSignature
	excludedBases map[string]bool
	macros        []string
}

func NewArgumentValidator() *ArgumentValidator {
	av := &ArgumentValidator{
		functions:     make(map[string]*FunctionSignature),
		excludedBases: make(map[string]bool),
		macros:        nil,
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

func (av *ArgumentValidator) SetMacros(names []string) {
	av.macros = append(av.macros[:0], names...)
}

func (av *ArgumentValidator) looksLikeMacroName(name string) bool {
	if name == "" || len(av.macros) == 0 {
		return false
	}
	low := strings.ToLower(name)
	for _, m := range av.macros {
		if m == "" {
			continue
		}
		if strings.Contains(low, strings.ToLower(m)) {
			return true
		}
	}
	return false
}

func (av *ArgumentValidator) RegisterMethod(
	className string,
	name string,
	params []*MethodParameter,
	returnType string,
	module string,
) {
	if className == "" || name == "" {
		return
	}
	key := className + "." + name
	sig := &FunctionSignature{Name: key, Parameters: params, ReturnType: returnType, Module: module}
	av.functions[key] = sig
	if module != "" {
		modKey := module + "::" + className + "." + name
		av.functions[modKey] = sig
		usKey := module + "_" + className + "_" + name
		av.functions[usKey] = sig
	}
}

func (av *ArgumentValidator) RegisterFunction(
	name string,
	params []*MethodParameter,
	returnType string,
	module string,
) {
	if name == "" {
		return
	}
	sig := &FunctionSignature{Name: name, Parameters: params, ReturnType: returnType, Module: module}
	av.functions[name] = sig
	if module != "" {
		modKey := module + "::" + name
		av.functions[modKey] = sig
		usKey := module + "_" + name
		av.functions[usKey] = sig
	}
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
		if stmt.ListDecl != nil && stmt.ListDecl.Name != "" {
			et := strings.TrimSpace(stmt.ListDecl.Type)
			if et == "" {
				varTypes[stmt.ListDecl.Name] = "list[]"
			} else {
				varTypes[stmt.ListDecl.Name] = "list[" + et + "]"
			}
		}
		if stmt.ListDeclFunctionCall != nil && stmt.ListDeclFunctionCall.Name != "" {
			et := strings.TrimSpace(stmt.ListDeclFunctionCall.Type)
			if et == "" {
				varTypes[stmt.ListDeclFunctionCall.Name] = "list[]"
			} else {
				varTypes[stmt.ListDeclFunctionCall.Name] = "list[" + et + "]"
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

		// var T name = obj.method(...)
		if stmt.VarDeclMethodCall != nil && stmt.VarDeclMethodCall.Name != "" {
			varTypes[stmt.VarDeclMethodCall.Name] = normalizeTypeName(strings.TrimSpace(stmt.VarDeclMethodCall.Type))
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

		// Do not pre-register assignment targets for method calls either.
		if stmt.IndexAssign != nil && stmt.IndexAssign.ListName != "" {
			if _, ok := varTypes[stmt.IndexAssign.ListName]; !ok {
				varTypes[stmt.IndexAssign.ListName] = "list[]"
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

func (av *ArgumentValidator) ValidateFunctionCall(funcCall *FunctionCallStmt, line int) error {
	var (
		funcName          = funcCall.Name
		signature, exists = av.functions[funcName]
	)

	if av.looksLikeMacroName(funcName) {
		return nil
	}

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
		}
	}

	if !exists {
		if idx := strings.Index(funcName, "_"); idx > 0 {
			mod := funcName[:idx]
			if mod != "" && av.excludedBases != nil && av.excludedBases[mod] {
				return nil
			}
		}
		if funcName == "" {
			return nil
		}
		if strings.Contains(funcName, "_") { //TODO: REPLACE LATER WITH MACRO DETECTION
			return nil
		}
		return fmt.Errorf("line %d: function '%s' is not defined", line, funcName)
	}

	params := signature.Parameters
	actualCount := len(funcCall.Args)
	hasVarargs := len(params) > 0 && params[len(params)-1] != nil && params[len(params)-1].IsVarargs
	fixedCount := len(params)
	if hasVarargs {
		fixedCount = len(params) - 1
	}
	if (!hasVarargs && actualCount != fixedCount) || (hasVarargs && actualCount < fixedCount) {
		suffix := ""
		if hasVarargs {
			suffix = "+"
		}
		return fmt.Errorf("line %d: function '%s' expects %d%s arguments, but %d were provided", line, funcName, fixedCount, suffix, actualCount)
	}

	for _, arg := range funcCall.Args {
		if err := av.ValidateStringFunctionCall(arg, line); err != nil {
			return err
		}
	}

	return nil
}

func (av *ArgumentValidator) ValidateMethodCall(methodCall *MethodCallStmt, line int, varTypes map[string]string) error {
	recv, ok := resolveReceiverType(methodCall.Object, varTypes)
	if !ok {
		return nil
	}

	var (
		key         = recv + "." + methodCall.Method
		sig, exists = av.functions[key]
	)

	// try underscore variant here (Class_Method).
	if !exists {
		if s2, ok2 := av.functions[recv+"_"+methodCall.Method]; ok2 {
			sig, exists = s2, true
		}
	}

	if !exists {
		return fmt.Errorf("line %d: method '%s.%s' is not defined", line, recv, methodCall.Method)
	}

	if !av.looksLikeMacroName(methodCall.Method) {
		params := sig.Parameters
		actual := len(methodCall.Args)
		hasVarargs := len(params) > 0 && params[len(params)-1] != nil && params[len(params)-1].IsVarargs
		fixed := len(params)
		if hasVarargs {
			fixed = len(params) - 1
		}
		if (!hasVarargs && actual != fixed) || (hasVarargs && actual < fixed) {
			suffix := ""
			if hasVarargs {
				suffix = "+"
			}
			return fmt.Errorf("line %d: method '%s.%s' expects %d%s arguments, but %d were provided", line, recv, methodCall.Method, fixed, suffix, actual)
		}
	}

	for _, a := range methodCall.Args {
		if err := av.ValidateStringFunctionCall(a, line); err != nil {
			return err
		}
	}
	return nil
}

func (av *ArgumentValidator) ValidateStaticMethodCall(call *StaticMethodCallStmt, line int) error {
	recv := strings.TrimSpace(call.Class)
	if recv == "" {
		return nil
	}
	// Strip module if present
	if strings.Contains(recv, "::") {
		parts := strings.Split(recv, "::")
		recv = parts[len(parts)-1]
	}
	key := recv + "." + call.Method
	sig, exists := av.functions[key]
	if !exists {
		if s2, ok2 := av.functions[recv+"_"+call.Method]; ok2 {
			sig, exists = s2, true
		}
	}
	if !exists {
		return fmt.Errorf("line %d: static method '%s.%s' is not defined", line, recv, call.Method)
	}
	if !av.looksLikeMacroName(call.Method) {
		params := sig.Parameters
		actual := len(call.Args)
		hasVarargs := len(params) > 0 && params[len(params)-1] != nil && params[len(params)-1].IsVarargs
		fixed := len(params)
		if hasVarargs {
			fixed = len(params) - 1
		}
		if (!hasVarargs && actual != fixed) || (hasVarargs && actual < fixed) {
			suffix := ""
			if hasVarargs {
				suffix = "+"
			}
			return fmt.Errorf("line %d: static method '%s.%s' expects %d%s arguments, but %d were provided", line, recv, call.Method, fixed, suffix, actual)
		}
	}
	return nil
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

func scanUnknownVars(expr string, varTypes map[string]string, excludedBases map[string]bool) []string {
	s := strings.TrimSpace(expr)

	if s == "" {
		return nil
	}
	if (strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"")) || (strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'")) {
		return nil
	}
	if strings.Contains(s, "\"") {
		return nil
	}
	if strings.ContainsAny(s, "'") && !strings.ContainsAny(s, "\"'()::.+-*/%<>=!&|[]{}") {
		return nil
	}

	var (
		hasRegexOrLines = strings.Contains(s, "regex::") || strings.Contains(s, "read_lines") || strings.Contains(s, "split(")
		specialScratch  = map[string]bool{"matches": true, "lines": true, "mm": true}
		reserved        = map[string]bool{"true": true, "false": true, "new": true, "or": true, "and": true, "not": true, "in": true}
		unknown         = []string{}
	)

	for i := 0; i < len(s); {
		ch := s[i]
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			i++
			continue
		}
		if ch == '"' || ch == '\'' {
			q := ch
			i++
			esc := false
			for i < len(s) {
				c := s[i]
				if esc {
					esc = false
					i++
					continue
				}
				if c == '\\' {
					esc = true
					i++
					continue
				}
				if c == q {
					i++
					break
				}
				i++
			}
			continue
		}
		// identifier (possibly a dotted/indexed chain)
		if ch == '_' || (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') {
			start := i
			i++
			for i < len(s) {
				c := s[i]
				if c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
					i++
					continue
				}
				break
			}
			base := s[start:i]
			j := i
			for j < len(s) && s[j] == ' ' {
				j++
			}
			if j < len(s) && s[j] == '(' {
				continue
			}
			if j+1 < len(s) && s[j] == ':' && s[j+1] == ':' {
				j += 2
				for j < len(s) && s[j] == ' ' {
					j++
				}
				k := j
				for k < len(s) {
					c := s[k]
					if c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
						k++
						continue
					}
					break
				}
				for k < len(s) && s[k] == ' ' {
					k++
				}
				if k < len(s) && s[k] == '(' {
					i = k
					continue
				}
			}
			hadDot := false
			for {
				for i < len(s) && s[i] == ' ' {
					i++
				}
				if i < len(s) && s[i] == '.' {
					hadDot = true
					i++
					for i < len(s) && s[i] == ' ' {
						i++
					}
					for i < len(s) {
						c := s[i]
						if c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
							i++
							continue
						}
						i++
					}
					continue
				}
				break
			}
			if hadDot {
				continue
			}
			if reserved[base] {
				continue
			}
			if excludedBases != nil && excludedBases[base] {
				continue
			}
			if idx := strings.Index(base, "_"); idx > 0 {
				mod := base[:idx]
				if (excludedBases != nil && excludedBases[mod]) || reserved[mod] {
					continue
				}
			}
			if hasRegexOrLines && specialScratch[base] {
				continue
			}
			if _, ok := varTypes[base]; !ok {
				exists := slices.Contains(unknown, base)
				if !exists {
					unknown = append(unknown, base)
				}
			}
			continue
		}
		i++
	}
	return unknown
}

func validateStatementRecursive(stmt *Statement, validator *ArgumentValidator, line int, expectedReturn string, varTypes map[string]string, excludedBases map[string]bool) []error {
	if stmt == nil {
		return nil
	}

	errors := []error{}

	cloneScope := func() map[string]string {
		child := make(map[string]string, len(varTypes))
		maps.Copy(child, varTypes)
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
			varTypes[stmt.ListDecl.Name] = "list[" + et + "]"
		}
	}
	if stmt.ListDeclFunctionCall != nil && stmt.ListDeclFunctionCall.Name != "" {
		et := strings.TrimSpace(stmt.ListDeclFunctionCall.Type)
		if et == "" {
			varTypes[stmt.ListDeclFunctionCall.Name] = "list[]"
		} else {
			varTypes[stmt.ListDeclFunctionCall.Name] = "list[" + et + "]"
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

	if stmt.FunctionCall != nil {
		if err := validator.ValidateFunctionCall(stmt.FunctionCall, line); err != nil {
			return []error{err}
		}
	}

	primitiveTypes := []string{
		"int", "float", "bool", "string", "i32", "i8", "u8", "u32", "u64", "i64", "f32", "f64", "char", "byte", "void", "nil",
	}

	if stmt.VarAssign != nil {
		if _, ok := varTypes[stmt.VarAssign.Name]; !ok {
			if strings.Contains(stmt.VarAssign.Name, "this") || strings.Contains(stmt.VarAssign.Name, ".") || strings.Contains(stmt.VarAssign.Name, ":") || strings.Contains(stmt.VarAssign.Name, "->>") {
				return nil
			}
			if !slices.Contains(primitiveTypes, stmt.VarAssign.Name) {
				return []error{fmt.Errorf("line %d: variable '%s' is not defined", line, stmt.VarAssign.Name)}
			}
		}
		for _, u := range scanUnknownVars(stmt.VarAssign.Value, varTypes, excludedBases) {
			if !slices.Contains(primitiveTypes, u) && u != "" {
				return []error{fmt.Errorf("line %d: variable '%s' is not defined", line, u)}
			}
		}
	}

	if stmt.MethodCall != nil {
		if err := validator.ValidateMethodCall(stmt.MethodCall, line, varTypes); err != nil {
			return []error{err}
		}
		// TODO: implement this correctly.
		//
		// for _, a := range stmt.MethodCall.Args {
		// 	for _, u := range scanUnknownVars(a, varTypes, excludedBases) {
		// 		fmt.Println(varTypes)
		// 		return []error{fmt.Errorf("line %d: variable '%s' is not defined", line, u)}
		// 	}
		// }
	}

	if stmt.StaticMethodCall != nil {
		if err := validator.ValidateStaticMethodCall(stmt.StaticMethodCall, line); err != nil {
			return []error{err}
		}
		for _, a := range stmt.StaticMethodCall.Args {
			for _, u := range scanUnknownVars(a, varTypes, excludedBases) {
				return []error{fmt.Errorf("line %d: variable '%s' is not defined", line, u)}
			}
		}
	}

	if stmt.VarDeclMethodCall != nil {
		mc := &MethodCallStmt{Object: stmt.VarDeclMethodCall.Object, Method: stmt.VarDeclMethodCall.Method, Args: stmt.VarDeclMethodCall.Args}
		if err := validator.ValidateMethodCall(mc, line, varTypes); err != nil {
			return []error{err}
		}
		for _, a := range stmt.VarDeclMethodCall.Args {
			for _, u := range scanUnknownVars(a, varTypes, excludedBases) {
				return []error{fmt.Errorf("line %d: variable '%s' is not defined", line, u)}
			}
		}
	}

	if stmt.VarAssignMethodCall != nil {
		if _, ok := varTypes[stmt.VarAssignMethodCall.Name]; !ok {
			return []error{fmt.Errorf("line %d: variable '%s' is not defined", line, stmt.VarAssignMethodCall.Name)}
		}
		mc := &MethodCallStmt{Object: stmt.VarAssignMethodCall.Object, Method: stmt.VarAssignMethodCall.Method, Args: stmt.VarAssignMethodCall.Args}
		if err := validator.ValidateMethodCall(mc, line, varTypes); err != nil {
			return []error{err}
		}
		for _, a := range stmt.VarAssignMethodCall.Args {
			for _, u := range scanUnknownVars(a, varTypes, excludedBases) {
				return []error{fmt.Errorf("line %d: variable '%s' is not defined", line, u)}
			}
		}
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
			nestedErrors := validateStatementRecursive(nested, validator, line, stmt.TopLevelFuncDecl.ReturnType, child, excludedBases)
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
			nestedErrors := validateStatementRecursive(nested, validator, line, stmt.PubTopLevelFuncDecl.ReturnType, child, excludedBases)
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
				nestedErrors := validateStatementRecursive(nestedStmt, validator, line, m.ReturnType, child, excludedBases)
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
				nestedErrors := validateStatementRecursive(nestedStmt, validator, line, m.ReturnType, child, excludedBases)
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
			nestedErrors := validateStatementRecursive(nested, validator, line, expectedReturn, child, excludedBases)
			errors = append(errors, nestedErrors...)
		}
	}

	if stmt.ReverseFor != nil {
		child := cloneScope()
		if stmt.ReverseFor.Var != "" {
			child[stmt.ReverseFor.Var] = "int"
		}
		for _, nested := range stmt.ReverseFor.Body {
			nestedErrors := validateStatementRecursive(nested, validator, line, expectedReturn, child, excludedBases)
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
			nestedErrors := validateStatementRecursive(nested, validator, line, expectedReturn, child, excludedBases)
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
			nestedErrors := validateStatementRecursive(nestedStmt, validator, line, expectedReturn, child, excludedBases)
			errors = append(errors, nestedErrors...)
		}
	}

	if stmt.While != nil {
		child := cloneScope()
		for _, nested := range stmt.While.Body {
			nestedErrors := validateStatementRecursive(nested, validator, line, expectedReturn, child, excludedBases)
			errors = append(errors, nestedErrors...)
		}
	}

	if stmt.If != nil {
		child := cloneScope()
		for _, nested := range stmt.If.Body {
			nestedErrors := validateStatementRecursive(nested, validator, line, expectedReturn, child, excludedBases)
			errors = append(errors, nestedErrors...)
		}
		for _, e := range stmt.If.ElseIfs {
			for _, nested := range e.Body {
				nestedErrors := validateStatementRecursive(nested, validator, line, expectedReturn, child, excludedBases)
				errors = append(errors, nestedErrors...)
			}
		}
		if stmt.If.Else != nil {
			for _, nested := range stmt.If.Else.Body {
				nestedErrors := validateStatementRecursive(nested, validator, line, expectedReturn, child, excludedBases)
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
	return validateProgramInternal(program, nil)
}

func ValidateProgramWithMacros(program *Program, macros []string) []error {
	if program == nil {
		return []error{fmt.Errorf("nil program")}
	}
	return validateProgramInternal(program, macros)
}

func validateProgramInternal(program *Program, macros []string) []error {
	validator := NewArgumentValidator()
	varTypes := make(map[string]string)
	preRegisterVars(program.Statements, varTypes)

	if len(macros) > 0 {
		validator.SetMacros(macros)
	}

	if len(program.Imports) > 0 {
		baseDir := filepath.Dir(CurrentSourceFile)
		for _, imp := range program.Imports {
			if imp == nil || strings.TrimSpace(imp.Module) == "" {
				continue
			}
			name := strings.TrimSpace(imp.Module)
			if _, ok := LoadedModules[name]; !ok {
				if _, err := LoadModule(name, baseDir); err != nil {
					_ = err
				}
			}
		}
	}

	var registerFuncs func(stmts []*Statement)
	registerFuncs = func(stmts []*Statement) {
		for _, st := range stmts {
			if st == nil {
				continue
			}
			if st.TopLevelFuncDecl != nil {
				validator.RegisterFunction(st.TopLevelFuncDecl.Name, st.TopLevelFuncDecl.Parameters, st.TopLevelFuncDecl.ReturnType, "")
				registerFuncs(st.TopLevelFuncDecl.Body)
			}
			if st.PubTopLevelFuncDecl != nil {
				validator.RegisterFunction(st.PubTopLevelFuncDecl.Name, st.PubTopLevelFuncDecl.Parameters, st.PubTopLevelFuncDecl.ReturnType, "")
				registerFuncs(st.PubTopLevelFuncDecl.Body)
			}
			if st.ClassDecl != nil {
				for _, m := range st.ClassDecl.Methods {
					name := st.ClassDecl.Name + "." + m.Name
					validator.RegisterMethod(st.ClassDecl.Name, m.Name, m.Parameters, m.ReturnType, "")
					registerFuncs(m.Body)
					_ = name // here the name's kept for clarity; RegisterMethod already keys multiple formats
				}
			}
			if st.PubClassDecl != nil {
				for _, m := range st.PubClassDecl.Methods {
					name := st.PubClassDecl.Name + "." + m.Name
					validator.RegisterMethod(st.PubClassDecl.Name, m.Name, m.Parameters, m.ReturnType, "")
					registerFuncs(m.Body)
					_ = name
				}
			}
			if st.If != nil {
				registerFuncs(st.If.Body)
				for _, e := range st.If.ElseIfs {
					registerFuncs(e.Body)
				}
				if st.If.Else != nil {
					registerFuncs(st.If.Else.Body)
				}
			}
			if st.For != nil {
				registerFuncs(st.For.Body)
			}
			if st.ReverseFor != nil {
				registerFuncs(st.ReverseFor.Body)
			}
			if st.VerboseFor != nil {
				registerFuncs(st.VerboseFor.Body)
			}
			if st.Foreach != nil {
				registerFuncs(st.Foreach.Body)
			}
			if st.While != nil {
				registerFuncs(st.While.Body)
			}
			if st.TryCatch != nil {
				registerFuncs(st.TryCatch.TryBody)
				registerFuncs(st.TryCatch.CatchBody)
			}
		}
	}

	registerFuncs(program.Statements)
	excludedBases := make(map[string]bool)

	for _, imp := range program.Imports {
		if imp == nil || strings.TrimSpace(imp.Module) == "" {
			continue
		}
		full := strings.TrimSpace(imp.Module)
		excludedBases[full] = true
		if idx := strings.LastIndex(full, "/"); idx != -1 && idx+1 < len(full) {
			short := full[idx+1:]
			if short != "" {
				excludedBases[short] = true
			}
		}
	}
	for name := range LoadedModules {
		if name == "" {
			continue
		}
		excludedBases[name] = true
		if idx := strings.LastIndex(name, "/"); idx != -1 && idx+1 < len(name) {
			short := name[idx+1:]
			if short != "" {
				excludedBases[short] = true
			}
		}
	}

	validator.excludedBases = excludedBases

	var allErrors []error
	line := 1
	for _, stmt := range program.Statements {
		errs := validateStatementRecursive(stmt, validator, line, "", varTypes, excludedBases)
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
