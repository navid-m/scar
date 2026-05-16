use std::collections::{HashMap, HashSet};

use crate::{
    CompileError,
    ast::{BinaryOp, Expr, FieldDef, Function, Program, Stmt, Type},
};

#[derive(Debug, Clone)]
pub struct FunctionSig {
    pub params: Vec<Type>,
    pub return_type: Type,
}

#[derive(Debug, Clone)]
pub struct TypeDefInfo {
    pub fields: Vec<FieldDef>,
    pub field_map: HashMap<String, Type>,
}

#[derive(Debug, Clone)]
pub struct ProgramInfo {
    pub functions: HashMap<String, FunctionSig>,
    pub types: HashMap<String, TypeDefInfo>,
    pub locals: HashMap<String, HashMap<String, Type>>,
}

#[derive(Debug, Clone)]
struct LocalBinding {
    ty: Type,
    mutable: bool,
}

pub fn analyze(program: &Program) -> Result<ProgramInfo, CompileError> {
    let types = collect_types(program)?;
    let mut functions = HashMap::new();
    for function in &program.functions {
        if functions.contains_key(&function.name) {
            return Err(CompileError::new(format!(
                "duplicate function definition `{}`",
                function.name
            )));
        }
        if function.name == "main" && !function.is_pub {
            return Err(CompileError::new(
                "function `main` must be declared as `pub def main`",
            ));
        }
        validate_type(&function.return_type, &types)?;
        functions.insert(
            function.name.clone(),
            FunctionSig {
                params: function
                    .params
                    .iter()
                    .map(|param| {
                        validate_type(&param.ty, &types)?;
                        Ok(param.ty.clone())
                    })
                    .collect::<Result<Vec<_>, CompileError>>()?,
                return_type: function.return_type.clone(),
            },
        );
    }

    let mut locals = HashMap::new();
    for function in &program.functions {
        analyze_function(function, &functions, &types, &mut locals)?;
    }

    Ok(ProgramInfo {
        functions,
        types,
        locals,
    })
}

fn collect_types(program: &Program) -> Result<HashMap<String, TypeDefInfo>, CompileError> {
    let known_type_names: HashSet<String> = program.type_defs.iter().map(|def| def.name.clone()).collect();
    let mut types = HashMap::new();

    for type_def in &program.type_defs {
        if types.contains_key(&type_def.name) {
            return Err(CompileError::new(format!(
                "duplicate type definition `{}`",
                type_def.name
            )));
        }

        let mut field_map = HashMap::new();
        for field in &type_def.fields {
            if field_map.contains_key(&field.name) {
                return Err(CompileError::new(format!(
                    "duplicate field `{}` in type `{}`",
                    field.name, type_def.name
                )));
            }
            validate_type_with_known_names(&field.ty, &known_type_names)?;
            field_map.insert(field.name.clone(), field.ty.clone());
        }

        types.insert(
            type_def.name.clone(),
            TypeDefInfo {
                fields: type_def.fields.clone(),
                field_map,
            },
        );
    }

    Ok(types)
}

fn analyze_function(
    function: &Function,
    functions: &HashMap<String, FunctionSig>,
    types: &HashMap<String, TypeDefInfo>,
    locals: &mut HashMap<String, HashMap<String, Type>>,
) -> Result<(), CompileError> {
    let mut scope = HashMap::new();
    let mut function_locals = HashMap::new();

    for param in &function.params {
        validate_type(&param.ty, types)?;
        if scope.contains_key(&param.name) {
            return Err(CompileError::new(format!(
                "duplicate parameter `{}` in function `{}`",
                param.name, function.name
            )));
        }
        scope.insert(
            param.name.clone(),
            LocalBinding {
                ty: param.ty.clone(),
                mutable: true,
            },
        );
    }

    for stmt in &function.body {
        analyze_stmt(
            stmt,
            &function.name,
            &function.return_type,
            functions,
            types,
            &mut scope,
            &mut function_locals,
        )?;
    }

    locals.insert(function.name.clone(), function_locals);
    Ok(())
}

fn analyze_stmt(
    stmt: &Stmt,
    function_name: &str,
    expected_return: &Type,
    functions: &HashMap<String, FunctionSig>,
    types: &HashMap<String, TypeDefInfo>,
    scope: &mut HashMap<String, LocalBinding>,
    function_locals: &mut HashMap<String, Type>,
) -> Result<(), CompileError> {
    match stmt {
        Stmt::VarDecl {
            mutable,
            name,
            declared_type,
            init,
        } => {
            if scope.contains_key(name) {
                return Err(CompileError::new(format!(
                    "duplicate local binding `{name}` in function `{function_name}`"
                )));
            }
            let ty = match declared_type {
                Some(expected) => {
                    validate_type(expected, types)?;
                    if let Expr::ListLiteral(values) = init {
                        infer_list_literal_type(values, Some(expected), functions, types, scope)?
                    } else {
                        let actual = infer_expr_type(init, functions, types, scope)?;
                        expect_same_type(expected, &actual, "variable initializer")?;
                        expected.clone()
                    }
                }
                None => infer_expr_type(init, functions, types, scope)?,
            };
            scope.insert(
                name.clone(),
                LocalBinding {
                    ty: ty.clone(),
                    mutable: *mutable,
                },
            );
            function_locals.insert(name.clone(), ty);
        }
        Stmt::Assign { target, value } => {
            let target_ty = infer_lvalue_type(target, functions, types, scope)?;
            let value_ty = infer_expr_type(value, functions, types, scope)?;
            expect_same_type(&target_ty, &value_ty, "assignment")?;
        }
        Stmt::AddAssign { target, value } => {
            let target_ty = infer_mutable_target(target, functions, types, scope)?;
            let value_ty = infer_expr_type(value, functions, types, scope)?;
            if target_ty != Type::I32 || value_ty != Type::I32 {
                return Err(CompileError::new(
                    "`+=` currently requires both operands to have type i32",
                ));
            }
        }
        Stmt::Return(value) => match (expected_return, value) {
            (Type::Void, None) => {}
            (Type::Void, Some(_)) => {
                return Err(CompileError::new("void functions cannot return a value"));
            }
            (expected, Some(expr)) => {
                let actual = infer_expr_type(expr, functions, types, scope)?;
                expect_same_type(expected, &actual, "return")?;
            }
            (_, None) => {
                return Err(CompileError::new("non-void functions must return a value"));
            }
        },
        Stmt::Expr(expr) => {
            infer_expr_type(expr, functions, types, scope)?;
        }
        Stmt::ForRange {
            pragma: _,
            var_name,
            start,
            end,
            body,
        } => {
            let start_ty = infer_expr_type(start, functions, types, scope)?;
            let end_ty = infer_expr_type(end, functions, types, scope)?;
            if start_ty != Type::I32 || end_ty != Type::I32 {
                return Err(CompileError::new("`for` bounds must have type i32"));
            }

            let mut nested = scope.clone();
            nested.insert(
                var_name.clone(),
                LocalBinding {
                    ty: Type::I32,
                    mutable: true,
                },
            );
            for stmt in body {
                analyze_stmt(
                    stmt,
                    function_name,
                    expected_return,
                    functions,
                    types,
                    &mut nested,
                    function_locals,
                )?;
            }
        }
        Stmt::ForEach {
            var_name,
            iterable,
            body,
        } => {
            let iterable_ty = infer_expr_type(iterable, functions, types, scope)?;
            let Type::List(element_ty) = iterable_ty else {
                return Err(CompileError::new(format!(
                    "`for ... in ...` requires a list iterable, got {}",
                    describe_type(&iterable_ty)
                )));
            };

            let mut nested = scope.clone();
            nested.insert(
                var_name.clone(),
                LocalBinding {
                    ty: (*element_ty).clone(),
                    mutable: true,
                },
            );
            for stmt in body {
                analyze_stmt(
                    stmt,
                    function_name,
                    expected_return,
                    functions,
                    types,
                    &mut nested,
                    function_locals,
                )?;
            }
        }
    }
    Ok(())
}

fn infer_expr_type(
    expr: &Expr,
    functions: &HashMap<String, FunctionSig>,
    types: &HashMap<String, TypeDefInfo>,
    scope: &HashMap<String, LocalBinding>,
) -> Result<Type, CompileError> {
    match expr {
        Expr::Int(_) => Ok(Type::I32),
        Expr::String(_) => Ok(Type::Ref(Box::new(Type::U8))),
        Expr::ListLiteral(values) => infer_list_literal_type(values, None, functions, types, scope),
        Expr::StructInit { name, fields } => {
            let type_info = types
                .get(name)
                .ok_or_else(|| CompileError::new(format!("unknown type `{name}`")))?;
            let mut seen = HashSet::new();
            for field in fields {
                let expected = type_info.field_map.get(&field.name).ok_or_else(|| {
                    CompileError::new(format!("type `{name}` has no field `{}`", field.name))
                })?;
                if !seen.insert(field.name.clone()) {
                    return Err(CompileError::new(format!(
                        "duplicate field initializer `{}` for type `{name}`",
                        field.name
                    )));
                }
                let actual = infer_expr_type(&field.value, functions, types, scope)?;
                expect_same_type(expected, &actual, &format!("field `{}`", field.name))?;
            }
            if seen.len() != type_info.fields.len() {
                let missing = type_info
                    .fields
                    .iter()
                    .find(|field| !seen.contains(&field.name))
                    .map(|field| field.name.clone())
                    .unwrap_or_else(|| "<unknown>".to_string());
                return Err(CompileError::new(format!(
                    "missing field `{missing}` in initializer for type `{name}`"
                )));
            }
            Ok(Type::Named(name.clone()))
        }
        Expr::FieldAccess { base, field } => {
            let base_ty = infer_expr_type(base, functions, types, scope)?;
            infer_field_type(&base_ty, field, types)
        }
        Expr::BuiltinCall { name, args } => analyze_builtin(name, args, functions, types, scope),
        Expr::MethodCall {
            receiver,
            method,
            args,
        } => analyze_method_call(receiver, method, args, functions, types, scope),
        Expr::Path(path) => match path.as_slice() {
            [name] => scope
                .get(name)
                .map(|binding| binding.ty.clone())
                .ok_or_else(|| CompileError::new(format!("unknown name `{name}`"))),
            _ => Err(CompileError::new(format!(
                "qualified path `{}` is not a valid expression",
                path.join(".")
            ))),
        },
        Expr::Binary { lhs, op, rhs } => {
            let lhs_ty = infer_expr_type(lhs, functions, types, scope)?;
            let rhs_ty = infer_expr_type(rhs, functions, types, scope)?;
            match op {
                BinaryOp::Add if lhs_ty == Type::I32 && rhs_ty == Type::I32 => Ok(Type::I32),
                BinaryOp::Add => Err(CompileError::new(
                    "`+` currently requires both operands to have type i32",
                )),
            }
        }
        Expr::Pack(_) => Err(CompileError::new(
            "packed `{...}` expressions are only valid as @print arguments",
        )),
        Expr::Call { callee, args } => analyze_call(callee, args, functions, types, scope),
    }
}

fn analyze_call(
    callee: &Expr,
    args: &[Expr],
    functions: &HashMap<String, FunctionSig>,
    types: &HashMap<String, TypeDefInfo>,
    scope: &HashMap<String, LocalBinding>,
) -> Result<Type, CompileError> {
    if let Some(path) = callee.as_path() {
        if path.len() == 1 {
            let function_name = &path[0];
            let signature = functions
                .get(function_name)
                .ok_or_else(|| CompileError::new(format!("unknown function `{function_name}`")))?;
            if signature.params.len() != args.len() {
                return Err(CompileError::new(format!(
                    "function `{function_name}` expects {} arguments but received {}",
                    signature.params.len(),
                    args.len()
                )));
            }
            for (arg, expected) in args.iter().zip(&signature.params) {
                let actual = infer_expr_type(arg, functions, types, scope)?;
                expect_same_type(expected, &actual, "function argument")?;
            }
            return Ok(signature.return_type.clone());
        }
    }

    Err(CompileError::new(
        "only direct function calls and @builtin calls are currently supported",
    ))
}

fn analyze_method_call(
    receiver: &Expr,
    method: &str,
    args: &[Expr],
    functions: &HashMap<String, FunctionSig>,
    types: &HashMap<String, TypeDefInfo>,
    scope: &HashMap<String, LocalBinding>,
) -> Result<Type, CompileError> {
    let receiver_ty = match method {
        "capacity" => infer_expr_type(receiver, functions, types, scope)?,
        _ => infer_mutable_target(receiver, functions, types, scope)?,
    };
    let Type::List(element_ty) = receiver_ty else {
        return Err(CompileError::new(format!(
            "`@{method}` requires a list receiver, got {}",
            describe_type(&receiver_ty)
        )));
    };

    match method {
        "append" => {
            if args.len() != 1 {
                return Err(CompileError::new("@append expects exactly one argument"));
            }
            let value_ty = infer_expr_type(&args[0], functions, types, scope)?;
            expect_same_type(&element_ty, &value_ty, "@append")?;
            Ok(Type::Void)
        }
        "capacity" => {
            if !args.is_empty() {
                return Err(CompileError::new("@capacity expects no arguments"));
            }
            Ok(Type::I32)
        }
        "reserve" => {
            if args.len() != 1 {
                return Err(CompileError::new("@reserve expects exactly one argument"));
            }
            let capacity_ty = infer_expr_type(&args[0], functions, types, scope)?;
            expect_same_type(&Type::I32, &capacity_ty, "@reserve")?;
            Ok(Type::Void)
        }
        "set" => {
            if args.len() != 2 {
                return Err(CompileError::new("@set expects exactly two arguments"));
            }
            let index_ty = infer_expr_type(&args[0], functions, types, scope)?;
            expect_same_type(&Type::I32, &index_ty, "@set index")?;
            let value_ty = infer_expr_type(&args[1], functions, types, scope)?;
            expect_same_type(&element_ty, &value_ty, "@set value")?;
            Ok(Type::Void)
        }
        "insert" => {
            if args.len() != 2 {
                return Err(CompileError::new("@insert expects exactly two arguments"));
            }
            let index_ty = infer_expr_type(&args[0], functions, types, scope)?;
            expect_same_type(&Type::I32, &index_ty, "@insert index")?;
            let value_ty = infer_expr_type(&args[1], functions, types, scope)?;
            expect_same_type(&element_ty, &value_ty, "@insert value")?;
            Ok(Type::Void)
        }
        "remove" => {
            if args.len() != 1 {
                return Err(CompileError::new("@remove expects exactly one argument"));
            }
            let index_ty = infer_expr_type(&args[0], functions, types, scope)?;
            expect_same_type(&Type::I32, &index_ty, "@remove index")?;
            Ok((*element_ty).clone())
        }
        "clear" => {
            if !args.is_empty() {
                return Err(CompileError::new("@clear expects no arguments"));
            }
            Ok(Type::Void)
        }
        _ => Err(CompileError::new(format!(
            "unsupported list accessor `@{method}`"
        ))),
    }
}

fn analyze_builtin(
    name: &str,
    args: &[Expr],
    functions: &HashMap<String, FunctionSig>,
    types: &HashMap<String, TypeDefInfo>,
    scope: &HashMap<String, LocalBinding>,
) -> Result<Type, CompileError> {
    match name {
        "puts" => {
            if args.len() != 1 {
                return Err(CompileError::new(
                    "@puts expects exactly one argument",
                ));
            }
            let arg_ty = infer_expr_type(&args[0], functions, types, scope)?;
            expect_same_type(&Type::Ref(Box::new(Type::U8)), &arg_ty, "@puts")?;
            Ok(Type::Void)
        }
        "print" => {
            if args.is_empty() {
                return Err(CompileError::new(
                    "@print expects at least a format string argument",
                ));
            }
            let Expr::String(format) = &args[0] else {
                return Err(CompileError::new(
                    "@print requires a string literal as its first argument",
                ));
            };
            let flattened = flatten_print_args(&args[1..]);
            let markers = parse_format_markers(format)?;
            if markers.len() != flattened.len() {
                return Err(CompileError::new(format!(
                    "@print format expects {} values but received {}",
                    markers.len(),
                    flattened.len()
                )));
            }
            for (marker, arg) in markers.iter().zip(flattened.iter()) {
                let arg_ty = infer_expr_type(arg, functions, types, scope)?;
                if !format_type_matches(*marker, &arg_ty) {
                    return Err(CompileError::new(format!(
                        "format marker `{{{marker}}}` does not accept value of type {}",
                        describe_type(&arg_ty)
                    )));
                }
            }
            Ok(Type::Void)
        }
        "addr" => {
            if args.len() != 1 {
                return Err(CompileError::new("@addr expects exactly one argument"));
            }
            let inner = infer_lvalue_type(&args[0], functions, types, scope)?;
            Ok(Type::Ref(Box::new(inner)))
        }
        "deref" => {
            if args.len() != 1 {
                return Err(CompileError::new("@deref expects exactly one argument"));
            }
            let arg_ty = infer_expr_type(&args[0], functions, types, scope)?;
            match arg_ty {
                Type::Ref(inner) => Ok(*inner),
                other => Err(CompileError::new(format!(
                    "@deref requires a ref(...) argument, got {}",
                    describe_type(&other)
                ))),
            }
        }
        _ => Err(CompileError::new(format!(
            "unsupported builtin intrinsic `@{name}`"
        ))),
    }
}

fn infer_lvalue_type(
    expr: &Expr,
    functions: &HashMap<String, FunctionSig>,
    types: &HashMap<String, TypeDefInfo>,
    scope: &HashMap<String, LocalBinding>,
) -> Result<Type, CompileError> {
    infer_mutable_target(expr, functions, types, scope)
}

fn infer_mutable_target(
    expr: &Expr,
    functions: &HashMap<String, FunctionSig>,
    types: &HashMap<String, TypeDefInfo>,
    scope: &HashMap<String, LocalBinding>,
) -> Result<Type, CompileError> {
    match expr {
        Expr::Path(path) if path.len() == 1 => {
            let name = &path[0];
            let binding = scope
                .get(name)
                .ok_or_else(|| CompileError::new(format!("unknown name `{name}`")))?;
            if !binding.mutable {
                return Err(CompileError::new(format!(
                    "cannot assign to immutable binding `{name}`"
                )));
            }
            Ok(binding.ty.clone())
        }
        Expr::FieldAccess { base, field } => {
            let base_ty = infer_mutable_target(base, functions, types, scope)?;
            infer_field_type(&base_ty, field, types)
        }
        Expr::BuiltinCall { name, args } if name == "deref" => {
            analyze_builtin("deref", args, functions, types, scope)
        }
        _ => Err(CompileError::new(
            "only names, field references, and @deref(...) may appear on the left-hand side of an assignment",
        )),
    }
}

fn infer_field_type(
    base_ty: &Type,
    field: &str,
    types: &HashMap<String, TypeDefInfo>,
) -> Result<Type, CompileError> {
    match base_ty {
        Type::Named(name) => types
            .get(name)
            .and_then(|type_info| type_info.field_map.get(field))
            .cloned()
            .ok_or_else(|| CompileError::new(format!("type `{name}` has no field `{field}`"))),
        other => Err(CompileError::new(format!(
            "field access requires a named type, got {}",
            describe_type(other)
        ))),
    }
}

fn infer_list_literal_type(
    values: &[Expr],
    expected: Option<&Type>,
    functions: &HashMap<String, FunctionSig>,
    types: &HashMap<String, TypeDefInfo>,
    scope: &HashMap<String, LocalBinding>,
) -> Result<Type, CompileError> {
    if values.is_empty() {
        return match expected {
            Some(Type::List(element_ty)) => Ok(Type::List(element_ty.clone())),
            Some(other) => Err(CompileError::new(format!(
                "empty list literals require a list type, got {}",
                describe_type(other)
            ))),
            None => Err(CompileError::new(
                "cannot infer the element type of an empty list literal",
            )),
        };
    }

    let element_ty = match expected {
        Some(Type::List(element_ty)) => {
            for value in values {
                let actual = infer_expr_type(value, functions, types, scope)?;
                expect_same_type(element_ty, &actual, "list element")?;
            }
            element_ty.clone()
        }
        Some(other) => {
            return Err(CompileError::new(format!(
                "list literal cannot initialize non-list type {}",
                describe_type(other)
            )));
        }
        None => {
            let first_ty = infer_expr_type(&values[0], functions, types, scope)?;
            for value in &values[1..] {
                let actual = infer_expr_type(value, functions, types, scope)?;
                expect_same_type(&first_ty, &actual, "list element")?;
            }
            Box::new(first_ty)
        }
    };

    Ok(Type::List(element_ty))
}

fn flatten_print_args<'a>(args: &'a [Expr]) -> Vec<&'a Expr> {
    let mut flattened = Vec::new();
    for arg in args {
        match arg {
            Expr::Pack(values) => flattened.extend(values.iter()),
            other => flattened.push(other),
        }
    }
    flattened
}

fn parse_format_markers(format: &str) -> Result<Vec<char>, CompileError> {
    let mut markers = Vec::new();
    let mut chars = format.chars().peekable();
    while let Some(ch) = chars.next() {
        if ch == '{' {
            let Some(marker) = chars.next() else {
                return Err(CompileError::new(
                    "unterminated format marker in @print",
                ));
            };
            let Some('}') = chars.next() else {
                return Err(CompileError::new("unterminated format marker in @print"));
            };
            if !matches!(marker, 'd' | 'p' | 's') {
                return Err(CompileError::new(format!(
                    "unsupported @print marker `{{{marker}}}`"
                )));
            }
            markers.push(marker);
        }
    }
    Ok(markers)
}

fn format_type_matches(marker: char, ty: &Type) -> bool {
    match marker {
        'd' => matches!(ty, Type::I32),
        's' => matches!(ty, Type::Ref(inner) if inner.as_ref() == &Type::U8),
        'p' => matches!(ty, Type::Ref(_) | Type::Named(_)),
        _ => false,
    }
}

fn validate_type(ty: &Type, types: &HashMap<String, TypeDefInfo>) -> Result<(), CompileError> {
    match ty {
        Type::Void | Type::I32 | Type::U8 => Ok(()),
        Type::Named(name) => {
            if types.contains_key(name) {
                Ok(())
            } else {
                Err(CompileError::new(format!("unknown type `{name}`")))
            }
        }
        Type::Ref(inner) => validate_type(inner, types),
        Type::List(inner) => validate_type(inner, types),
    }
}

fn validate_type_with_known_names(ty: &Type, known: &HashSet<String>) -> Result<(), CompileError> {
    match ty {
        Type::Void | Type::I32 | Type::U8 => Ok(()),
        Type::Named(name) => {
            if known.contains(name) {
                Ok(())
            } else {
                Err(CompileError::new(format!("unknown type `{name}`")))
            }
        }
        Type::Ref(inner) => validate_type_with_known_names(inner, known),
        Type::List(inner) => validate_type_with_known_names(inner, known),
    }
}

fn expect_same_type(expected: &Type, actual: &Type, context: &str) -> Result<(), CompileError> {
    if expected == actual {
        Ok(())
    } else {
        Err(CompileError::new(format!(
            "{context} expects type {} but found {}",
            describe_type(expected),
            describe_type(actual)
        )))
    }
}

fn describe_type(ty: &Type) -> String {
    match ty {
        Type::Void => "void".to_string(),
        Type::I32 => "i32".to_string(),
        Type::U8 => "u8".to_string(),
        Type::Named(name) => name.clone(),
        Type::Ref(inner) => format!("ref({})", describe_type(inner)),
        Type::List(inner) => format!("list[{}]", describe_type(inner)),
    }
}

#[cfg(test)]
mod tests {
    use super::analyze;
    use crate::{lexer::lex, parser::parse_program};

    #[test]
    fn rejects_non_pub_main() {
        let source = "def main() void\nend\n";
        let program = parse_program(lex(source).unwrap()).unwrap();

        let error = analyze(&program).unwrap_err();
        assert_eq!(
            error.to_string(),
            "function `main` must be declared as `pub def main`"
        );
    }

    #[test]
    fn accepts_pub_main() {
        let source = "pub def main() void\nend\n";
        let program = parse_program(lex(source).unwrap()).unwrap();

        analyze(&program).unwrap();
    }

    #[test]
    fn analyzes_struct_init_and_field_access() {
        let source =
            "type SomeType\n\tx i32\n\ty i32\nend\npub def main() void\n\tval st = SomeType(x: 10, y: 12)\n\t@print(\"{d}\", {st.x})\nend\n";
        let program = parse_program(lex(source).unwrap()).unwrap();

        analyze(&program).unwrap();
    }

    #[test]
    fn accepts_string_literals_as_ref_u8() {
        let source = "pub def main() void\n\tval message ref(u8) = \"hello\"\n\t@puts(message)\n\t@print(\"{s}\", {message})\nend\n";
        let program = parse_program(lex(source).unwrap()).unwrap();

        analyze(&program).unwrap();
    }
}
