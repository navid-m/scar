use std::collections::{HashMap, HashSet};

use crate::{
    CompileError,
    ast::{BinaryOp, Expr, FieldDef, Function, Program, Stmt, Type, UnaryOp},
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
    pub alias: Option<Type>,
}

#[derive(Debug, Clone)]
pub struct ProgramInfo {
    pub functions: HashMap<String, FunctionSig>,
    pub function_symbols: HashMap<String, String>,
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
    let mut function_symbols = HashMap::new();

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
        function_symbols.insert(
            function.name.clone(),
            function
                .extern_name
                .clone()
                .unwrap_or_else(|| function.name.clone()),
        );
    }

    let mut locals = HashMap::new();
    for function in &program.functions {
        analyze_function(function, &functions, &types, &mut locals)?;
    }

    Ok(ProgramInfo {
        functions,
        function_symbols,
        types,
        locals,
    })
}

fn collect_types(program: &Program) -> Result<HashMap<String, TypeDefInfo>, CompileError> {
    let known_type_names: HashSet<String> =
        program.type_defs.iter().map(|def| def.name.clone()).collect();
    let mut types = HashMap::new();

    for type_def in &program.type_defs {
        if types.contains_key(&type_def.name) {
            return Err(CompileError::new(format!(
                "duplicate type definition `{}`",
                type_def.name
            )));
        }

        let mut field_map = HashMap::new();
        if let Some(alias) = &type_def.alias {
            if !type_def.fields.is_empty() {
                return Err(CompileError::new(format!(
                    "type `{}` cannot declare both an alias and fields",
                    type_def.name
                )));
            }
            validate_type_with_known_names(alias, &known_type_names)?;
        } else {
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
        }

        types.insert(
            type_def.name.clone(),
            TypeDefInfo {
                fields: type_def.fields.clone(),
                field_map,
                alias: type_def.alias.clone(),
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
            false,
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
    in_loop: bool,
) -> Result<(), CompileError> {
    match stmt {
        Stmt::VarDecl {
            line,
            column,
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
                        infer_list_literal_type(values, Some(expected), functions, types, scope)
                            .map_err(|error| error.with_location(*line, *column))?
                    } else {
                        let actual = infer_expr_type(init, functions, types, scope)
                            .map_err(|error| error.with_location(*line, *column))?;
                        expect_same_type(expected, &actual, types, "variable initializer")
                            .map_err(|error| error.with_location(*line, *column))?;
                        expected.clone()
                    }
                }
                None => infer_expr_type(init, functions, types, scope)
                    .map_err(|error| error.with_location(*line, *column))?,
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
        Stmt::Assign {
            line,
            column,
            target,
            value,
        } => {
            let target_ty = infer_mutable_target(target, functions, types, scope)
                .map_err(|error| error.with_location(*line, *column))?;
            let value_ty = infer_expr_type(value, functions, types, scope)
                .map_err(|error| error.with_location(*line, *column))?;
            expect_same_type(&target_ty, &value_ty, types, "assignment")
                .map_err(|error| error.with_location(*line, *column))?;
        }
        Stmt::AddAssign {
            line,
            column,
            target,
            value,
        } => {
            analyze_arithmetic_assignment(target, value, "+=", *line, *column, functions, types, scope)?;
        }
        Stmt::MulAssign {
            line,
            column,
            target,
            value,
        } => {
            analyze_arithmetic_assignment(target, value, "*=", *line, *column, functions, types, scope)?;
        }
        Stmt::SubAssign {
            line,
            column,
            target,
            value,
        } => {
            analyze_arithmetic_assignment(target, value, "-=", *line, *column, functions, types, scope)?;
        }
        Stmt::DivAssign {
            line,
            column,
            target,
            value,
        } => {
            analyze_arithmetic_assignment(target, value, "/=", *line, *column, functions, types, scope)?;
        }
        Stmt::BitAndAssign {
            line,
            column,
            target,
            value,
        } => {
            analyze_bitwise_assignment(target, value, "&=", *line, *column, functions, types, scope)?;
        }
        Stmt::BitOrAssign {
            line,
            column,
            target,
            value,
        } => {
            analyze_bitwise_assignment(target, value, "|=", *line, *column, functions, types, scope)?;
        }
        Stmt::BitXorAssign {
            line,
            column,
            target,
            value,
        } => {
            analyze_bitwise_assignment(target, value, "^=", *line, *column, functions, types, scope)?;
        }
        Stmt::Return {
            line,
            column,
            value,
        } => match (expected_return, value) {
            (Type::Void, None) => {}
            (Type::Void, Some(_)) => {
                return Err(
                    CompileError::new("void functions cannot return a value")
                        .with_location(*line, *column),
                );
            }
            (expected, Some(expr)) => {
                let actual = infer_expr_type(expr, functions, types, scope)
                    .map_err(|error| error.with_location(*line, *column))?;
                expect_same_type(expected, &actual, types, "return")
                    .map_err(|error| error.with_location(*line, *column))?;
            }
            (_, None) => {
                return Err(
                    CompileError::new("non-void functions must return a value")
                        .with_location(*line, *column),
                );
            }
        },
        Stmt::If {
            line,
            column,
            condition,
            then_body,
            else_body,
        } => {
            let condition_ty = infer_expr_type(condition, functions, types, scope)
                .map_err(|error| error.with_location(*line, *column))?;
            if !is_condition_type(&condition_ty, types)? {
                return Err(
                    CompileError::new(format!(
                        "`if` conditions must be bool or integer-compatible, got {}",
                        describe_type(&condition_ty)
                    ))
                    .with_location(*line, *column),
                );
            }
            let mut then_scope = scope.clone();
            for stmt in then_body {
                analyze_stmt(
                    stmt,
                    function_name,
                    expected_return,
                    functions,
                    types,
                    &mut then_scope,
                    function_locals,
                    in_loop,
                )?;
            }
            let mut else_scope = scope.clone();
            for stmt in else_body {
                analyze_stmt(
                    stmt,
                    function_name,
                    expected_return,
                    functions,
                    types,
                    &mut else_scope,
                    function_locals,
                    in_loop,
                )?;
            }
        }
        Stmt::Expr { line, column, expr } => {
            infer_expr_type(expr, functions, types, scope)
                .map_err(|error| error.with_location(*line, *column))?;
        }
        Stmt::ForRange {
            line,
            column,
            start,
            end,
            var_name,
            body,
            ..
        } => {
            let start_ty = resolve_aliases(
                &infer_expr_type(start, functions, types, scope)
                    .map_err(|error| error.with_location(*line, *column))?,
                types,
            )?;
            let end_ty = resolve_aliases(
                &infer_expr_type(end, functions, types, scope)
                    .map_err(|error| error.with_location(*line, *column))?,
                types,
            )?;
            if !(start_ty == Type::I32 && end_ty == Type::I32) {
                return Err(
                    CompileError::new("`for` bounds must have type i32")
                        .with_location(*line, *column),
                );
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
                    true,
                )?;
            }
        }
        Stmt::ForEach {
            line,
            column,
            var_name,
            iterable,
            body,
        } => {
            let iterable_ty = resolve_aliases(
                &infer_expr_type(iterable, functions, types, scope)
                    .map_err(|error| error.with_location(*line, *column))?,
                types,
            )?;
            let Type::List(element_ty) = deref_refs(&iterable_ty) else {
                return Err(
                    CompileError::new(format!(
                        "`for ... in ...` requires a list iterable, got {}",
                        describe_type(&iterable_ty)
                    ))
                    .with_location(*line, *column),
                );
            };
            let mut nested = scope.clone();
            nested.insert(
                var_name.clone(),
                LocalBinding {
                    ty: (**element_ty).clone(),
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
                    true,
                )?;
            }
        }
        Stmt::Loop { body, .. } => {
            let mut nested = scope.clone();
            for stmt in body {
                analyze_stmt(
                    stmt,
                    function_name,
                    expected_return,
                    functions,
                    types,
                    &mut nested,
                    function_locals,
                    true,
                )?;
            }
        }
        Stmt::Continue { line, column } => {
            if !in_loop {
                return Err(
                    CompileError::new("`continue` may only appear inside a loop")
                        .with_location(*line, *column),
                );
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
        Expr::String(_) => Ok(Type::U8),
        Expr::ListLiteral(values) => infer_list_literal_type(values, None, functions, types, scope),
        Expr::Index { base, index } => {
            let base_ty = infer_expr_type(base, functions, types, scope)?;
            let index_ty = infer_expr_type(index, functions, types, scope)?;
            expect_same_type(&Type::I32, &index_ty, types, "list index")?;
            infer_index_type(&base_ty, types)
        }
        Expr::StructInit { name, fields } => {
            let type_info = types
                .get(name)
                .ok_or_else(|| CompileError::new(format!("unknown type `{name}`")))?;
            if type_info.alias.is_some() {
                return Err(CompileError::new(format!(
                    "type `{name}` is an alias and cannot be initialized like a struct"
                )));
            }
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
                expect_same_type(expected, &actual, types, &format!("field `{}`", field.name))?;
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
        } => {
            let receiver_ty = match method.as_str() {
                "capacity" => infer_expr_type(receiver, functions, types, scope)?,
                _ => infer_lvalue_type(receiver, functions, types, scope)?,
            };
            analyze_list_method_with_receiver(method, &receiver_ty, args, functions, types, scope)
        }
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
        Expr::Call { callee, args } => analyze_call(callee, args, functions, types, scope),
        Expr::Cast { expr, ty } => {
            let source_ty = infer_expr_type(expr, functions, types, scope)?;
            validate_type(ty, types)?;
            if is_numeric_type(&source_ty, types)? && is_numeric_type(ty, types)? {
                Ok(ty.clone())
            } else {
                Err(CompileError::new(format!(
                    "cannot cast {} to {}",
                    describe_type(&source_ty),
                    describe_type(ty)
                )))
            }
        }
        Expr::Unary { op, expr } => {
            let inner_ty = resolve_aliases(&infer_expr_type(expr, functions, types, scope)?, types)?;
            match op {
                UnaryOp::Neg if is_signed_numeric_type(&inner_ty) => Ok(inner_ty),
                UnaryOp::Neg => Err(CompileError::new(
                    "unary `-` currently requires a signed numeric operand",
                )),
                UnaryOp::LogicalNot if is_condition_primitive_type(&inner_ty) => Ok(Type::Bool),
                UnaryOp::LogicalNot => Err(CompileError::new(
                    "unary `!` currently requires a bool or integer operand",
                )),
            }
        }
        Expr::Pack(_) => Err(CompileError::new(
            "packed `{...}` expressions are only valid as @print arguments",
        )),
        Expr::Binary { lhs, op, rhs } => {
            let lhs_ty = resolve_aliases(&infer_expr_type(lhs, functions, types, scope)?, types)?;
            let rhs_ty = resolve_aliases(&infer_expr_type(rhs, functions, types, scope)?, types)?;
            match op {
                BinaryOp::Add | BinaryOp::Subtract | BinaryOp::Divide | BinaryOp::Multiply
                    if common_numeric_type(&lhs_ty, &rhs_ty).is_some() =>
                {
                    Ok(common_numeric_type(&lhs_ty, &rhs_ty).unwrap())
                }
                BinaryOp::Modulo if common_integer_type(&lhs_ty, &rhs_ty).is_some() => {
                    Ok(common_integer_type(&lhs_ty, &rhs_ty).unwrap())
                }
                BinaryOp::Add => Err(CompileError::new(
                    "`+` currently requires compatible numeric operands",
                )),
                BinaryOp::Subtract => Err(CompileError::new(
                    "`-` currently requires compatible numeric operands",
                )),
                BinaryOp::Divide => Err(CompileError::new(
                    "`/` currently requires compatible numeric operands",
                )),
                BinaryOp::Multiply => Err(CompileError::new(
                    "`*` currently requires compatible numeric operands",
                )),
                BinaryOp::Modulo => Err(CompileError::new(
                    "`%` currently requires compatible integer operands",
                )),
                BinaryOp::LogicalAnd | BinaryOp::LogicalOr
                    if is_condition_primitive_type(&lhs_ty)
                        && is_condition_primitive_type(&rhs_ty) =>
                {
                    Ok(Type::Bool)
                }
                BinaryOp::LogicalAnd | BinaryOp::LogicalOr => Err(CompileError::new(
                    "logical operators currently require bool or integer operands",
                )),
                BinaryOp::BitAnd | BinaryOp::BitOr | BinaryOp::BitXor
                    if common_integer_type(&lhs_ty, &rhs_ty).is_some() =>
                {
                    Ok(common_integer_type(&lhs_ty, &rhs_ty).unwrap())
                }
                BinaryOp::BitAnd | BinaryOp::BitOr | BinaryOp::BitXor => Err(CompileError::new(
                    "bitwise operators currently require compatible integer operands",
                )),
                BinaryOp::ShiftLeft | BinaryOp::ShiftRight
                    if is_integer_type(&lhs_ty, types)? && is_integer_type(&rhs_ty, types)? =>
                {
                    Ok(lhs_ty)
                }
                BinaryOp::ShiftLeft | BinaryOp::ShiftRight => Err(CompileError::new(
                    "shift operators currently require integer operands",
                )),
                BinaryOp::LessThan
                | BinaryOp::LessEqual
                | BinaryOp::GreaterThan
                | BinaryOp::GreaterEqual
                | BinaryOp::Equal
                    if common_numeric_type(&lhs_ty, &rhs_ty).is_some() =>
                {
                    Ok(Type::Bool)
                }
                BinaryOp::Equal if lhs_ty == Type::Bool && rhs_ty == Type::Bool => Ok(Type::Bool),
                BinaryOp::LessThan
                | BinaryOp::LessEqual
                | BinaryOp::GreaterThan
                | BinaryOp::GreaterEqual
                | BinaryOp::Equal => Err(
                    CompileError::new(
                        "comparison operators currently require compatible numeric operands",
                    ),
                ),
            }
        }
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
            if let Some(signature) = functions.get(function_name) {
                if signature.params.len() != args.len() {
                    return Err(CompileError::new(format!(
                        "function `{function_name}` expects {} arguments but received {}",
                        signature.params.len(),
                        args.len()
                    )));
                }
                for (arg, expected) in args.iter().zip(&signature.params) {
                    let actual = infer_expr_type(arg, functions, types, scope)?;
                    expect_same_type(expected, &actual, types, "function argument")?;
                }
                return Ok(signature.return_type.clone());
            }

            if let Some(type_info) = types.get(function_name) {
                if type_info.alias.is_some() {
                    return Err(CompileError::new(format!(
                        "type `{function_name}` is an alias and cannot be initialized like a struct"
                    )));
                }
                if type_info.fields.len() != args.len() {
                    return Err(CompileError::new(format!(
                        "type `{function_name}` expects {} constructor arguments but received {}",
                        type_info.fields.len(),
                        args.len()
                    )));
                }
                for (arg, field) in args.iter().zip(&type_info.fields) {
                    let actual = infer_expr_type(arg, functions, types, scope)?;
                    expect_same_type(&field.ty, &actual, types, &format!("field `{}`", field.name))?;
                }
                return Ok(Type::Named(function_name.clone()));
            }

            return Err(CompileError::new(format!("unknown function `{function_name}`")));
        }
    }

    Err(CompileError::new(
        "only direct function calls and @builtin calls are currently supported",
    ))
}

fn analyze_builtin(
    name: &str,
    args: &[Expr],
    functions: &HashMap<String, FunctionSig>,
    types: &HashMap<String, TypeDefInfo>,
    scope: &HashMap<String, LocalBinding>,
) -> Result<Type, CompileError> {
    match name {
        "append" | "capacity" | "reserve" | "set" | "insert" | "remove" | "clear" => {
            analyze_builtin_list_method(name, args, functions, types, scope)
        }
        "puts" => {
            if args.len() != 1 {
                return Err(CompileError::new("@puts expects exactly one argument"));
            }
            let arg_ty = infer_expr_type(&args[0], functions, types, scope)?;
            expect_same_type(&Type::U8, &arg_ty, types, "@puts")?;
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
                if !format_type_matches(*marker, &resolve_aliases(&arg_ty, types)?) {
                    return Err(CompileError::new(format!(
                        "format marker `{{{}}}` does not accept value of type {}",
                        print_marker_name(*marker),
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
            match resolve_aliases(&arg_ty, types)? {
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

fn analyze_bitwise_assignment(
    target: &Expr,
    value: &Expr,
    operator: &str,
    line: usize,
    column: usize,
    functions: &HashMap<String, FunctionSig>,
    types: &HashMap<String, TypeDefInfo>,
    scope: &HashMap<String, LocalBinding>,
) -> Result<(), CompileError> {
    let target_ty = resolve_aliases(
        &infer_mutable_target(target, functions, types, scope)
            .map_err(|error| error.with_location(line, column))?,
        types,
    )?;
    let value_ty = resolve_aliases(
        &infer_expr_type(value, functions, types, scope)
            .map_err(|error| error.with_location(line, column))?,
        types,
    )?;
    if common_integer_type(&target_ty, &value_ty) == Some(target_ty.clone()) {
        Ok(())
    } else {
        Err(
            CompileError::new(format!(
                "`{operator}` currently requires compatible integer operands"
            ))
            .with_location(line, column),
        )
    }
}

fn analyze_arithmetic_assignment(
    target: &Expr,
    value: &Expr,
    operator: &str,
    line: usize,
    column: usize,
    functions: &HashMap<String, FunctionSig>,
    types: &HashMap<String, TypeDefInfo>,
    scope: &HashMap<String, LocalBinding>,
) -> Result<(), CompileError> {
    let target_ty = resolve_aliases(
        &infer_mutable_target(target, functions, types, scope)
            .map_err(|error| error.with_location(line, column))?,
        types,
    )?;
    let value_ty = resolve_aliases(
        &infer_expr_type(value, functions, types, scope)
            .map_err(|error| error.with_location(line, column))?,
        types,
    )?;
    if common_numeric_type(&target_ty, &value_ty) == Some(target_ty.clone()) {
        Ok(())
    } else {
        Err(
            CompileError::new(format!(
                "`{operator}` currently requires compatible numeric operands"
            ))
            .with_location(line, column),
        )
    }
}

fn infer_lvalue_type(
    expr: &Expr,
    functions: &HashMap<String, FunctionSig>,
    types: &HashMap<String, TypeDefInfo>,
    scope: &HashMap<String, LocalBinding>,
) -> Result<Type, CompileError> {
    match expr {
        Expr::Path(path) if path.len() == 1 => scope
            .get(&path[0])
            .map(|binding| binding.ty.clone())
            .ok_or_else(|| CompileError::new(format!("unknown name `{}`", path[0]))),
        Expr::FieldAccess { base, field } => {
            let base_ty = infer_lvalue_type(base, functions, types, scope)?;
            infer_field_type(&base_ty, field, types)
        }
        Expr::Index { base, index } => {
            let base_ty = infer_lvalue_type(base, functions, types, scope)?;
            let index_ty = infer_expr_type(index, functions, types, scope)?;
            expect_same_type(&Type::I32, &index_ty, types, "list index")?;
            infer_index_type(&base_ty, types)
        }
        Expr::BuiltinCall { name, args } if name == "deref" => {
            analyze_builtin("deref", args, functions, types, scope)
        }
        _ => Err(CompileError::new(
            "only names, field references, index references, and @deref(...) may appear as lvalues",
        )),
    }
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
        Expr::Index { base, index } => {
            let base_ty = infer_mutable_target(base, functions, types, scope)?;
            let index_ty = infer_expr_type(index, functions, types, scope)?;
            expect_same_type(&Type::I32, &index_ty, types, "list index")?;
            infer_index_type(&base_ty, types)
        }
        Expr::BuiltinCall { name, args } if name == "deref" => {
            analyze_builtin("deref", args, functions, types, scope)
        }
        _ => Err(CompileError::new(
            "only names, field references, index references, and @deref(...) may appear on the left-hand side of an assignment",
        )),
    }
}

fn infer_field_type(
    base_ty: &Type,
    field: &str,
    types: &HashMap<String, TypeDefInfo>,
) -> Result<Type, CompileError> {
    let resolved = resolve_aliases(base_ty, types)?;
    match deref_refs(&resolved) {
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

fn infer_index_type(base_ty: &Type, types: &HashMap<String, TypeDefInfo>) -> Result<Type, CompileError> {
    let resolved = resolve_aliases(base_ty, types)?;
    match deref_refs(&resolved) {
        Type::List(inner) => Ok((**inner).clone()),
        other => Err(CompileError::new(format!(
            "indexing requires a list type, got {}",
            describe_type(other)
        ))),
    }
}

fn analyze_builtin_list_method(
    name: &str,
    args: &[Expr],
    functions: &HashMap<String, FunctionSig>,
    types: &HashMap<String, TypeDefInfo>,
    scope: &HashMap<String, LocalBinding>,
) -> Result<Type, CompileError> {
    if args.is_empty() {
        return Err(CompileError::new(format!(
            "@{name} expects a list receiver as its first argument",
        )));
    }
    let receiver_ty = match name {
        "capacity" => infer_expr_type(&args[0], functions, types, scope)?,
        _ => infer_lvalue_type(&args[0], functions, types, scope)?,
    };
    analyze_list_method_with_receiver(name, &receiver_ty, &args[1..], functions, types, scope)
}

fn analyze_list_method_with_receiver(
    method: &str,
    receiver_ty: &Type,
    args: &[Expr],
    functions: &HashMap<String, FunctionSig>,
    types: &HashMap<String, TypeDefInfo>,
    scope: &HashMap<String, LocalBinding>,
) -> Result<Type, CompileError> {
    let resolved_receiver = resolve_aliases(receiver_ty, types)?;
    let element_ty = match deref_refs(&resolved_receiver) {
        Type::List(inner) => inner,
        other => {
            return Err(CompileError::new(format!(
                "`@{method}` requires a list receiver, got {}",
                describe_type(other)
            )))
        }
    };

    match method {
        "append" => {
            if args.len() != 1 {
                return Err(CompileError::new("@append expects exactly one argument"));
            }
            let value_ty = infer_expr_type(&args[0], functions, types, scope)?;
            expect_same_type(element_ty, &value_ty, types, "@append")?;
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
            expect_same_type(&Type::I32, &capacity_ty, types, "@reserve")?;
            Ok(Type::Void)
        }
        "set" => {
            if args.len() != 2 {
                return Err(CompileError::new("@set expects exactly two arguments"));
            }
            let index_ty = infer_expr_type(&args[0], functions, types, scope)?;
            expect_same_type(&Type::I32, &index_ty, types, "@set index")?;
            let value_ty = infer_expr_type(&args[1], functions, types, scope)?;
            expect_same_type(element_ty, &value_ty, types, "@set value")?;
            Ok(Type::Void)
        }
        "insert" => {
            if args.len() != 2 {
                return Err(CompileError::new("@insert expects exactly two arguments"));
            }
            let index_ty = infer_expr_type(&args[0], functions, types, scope)?;
            expect_same_type(&Type::I32, &index_ty, types, "@insert index")?;
            let value_ty = infer_expr_type(&args[1], functions, types, scope)?;
            expect_same_type(element_ty, &value_ty, types, "@insert value")?;
            Ok(Type::Void)
        }
        "remove" => {
            if args.len() != 1 {
                return Err(CompileError::new("@remove expects exactly one argument"));
            }
            let index_ty = infer_expr_type(&args[0], functions, types, scope)?;
            expect_same_type(&Type::I32, &index_ty, types, "@remove index")?;
            Ok((**element_ty).clone())
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
                expect_same_type(element_ty, &actual, types, "list element")?;
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
                expect_same_type(&first_ty, &actual, types, "list element")?;
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

#[derive(Clone, Copy)]
enum PrintMarker {
    Int,
    String,
    Pointer,
    Float,
    Double,
}

fn parse_format_markers(format: &str) -> Result<Vec<PrintMarker>, CompileError> {
    let mut markers = Vec::new();
    let chars: Vec<char> = format.chars().collect();
    let mut index = 0;
    while index < chars.len() {
        if chars[index] == '{' {
            index += 1;
            let start = index;
            while index < chars.len() && chars[index] != '}' {
                index += 1;
            }
            if index >= chars.len() {
                return Err(CompileError::new("unterminated format marker in @print"));
            }
            let marker: String = chars[start..index].iter().collect();
            let parsed = match marker.as_str() {
                "d" => PrintMarker::Int,
                "s" => PrintMarker::String,
                "p" => PrintMarker::Pointer,
                "f" => PrintMarker::Float,
                "lf" => PrintMarker::Double,
                _ => {
                    return Err(CompileError::new(format!(
                        "unsupported @print marker `{{{marker}}}`"
                    )));
                }
            };
            markers.push(parsed);
        }
        index += 1;
    }
    Ok(markers)
}

fn format_type_matches(marker: PrintMarker, ty: &Type) -> bool {
    match marker {
        PrintMarker::Int => is_integer_primitive_type(ty),
        PrintMarker::String => is_string_compatible(ty),
        PrintMarker::Pointer => matches!(ty, Type::Ref(_) | Type::Named(_)) || is_string_compatible(ty),
        PrintMarker::Float => matches!(ty, Type::F32),
        PrintMarker::Double => matches!(ty, Type::F64),
    }
}

fn print_marker_name(marker: PrintMarker) -> &'static str {
    match marker {
        PrintMarker::Int => "d",
        PrintMarker::String => "s",
        PrintMarker::Pointer => "p",
        PrintMarker::Float => "f",
        PrintMarker::Double => "lf",
    }
}

fn validate_type(ty: &Type, types: &HashMap<String, TypeDefInfo>) -> Result<(), CompileError> {
    match ty {
        Type::Void
        | Type::Bool
        | Type::I8
        | Type::I16
        | Type::I32
        | Type::I64
        | Type::Isize
        | Type::U16
        | Type::U32
        | Type::U64
        | Type::Usize
        | Type::U8
        | Type::F32
        | Type::F64 => Ok(()),
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
        Type::Void
        | Type::Bool
        | Type::I8
        | Type::I16
        | Type::I32
        | Type::I64
        | Type::Isize
        | Type::U16
        | Type::U32
        | Type::U64
        | Type::Usize
        | Type::U8
        | Type::F32
        | Type::F64 => Ok(()),
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

fn expect_same_type(
    expected: &Type,
    actual: &Type,
    types: &HashMap<String, TypeDefInfo>,
    context: &str,
) -> Result<(), CompileError> {
    if types_compatible(expected, actual, types)? {
        Ok(())
    } else {
        Err(CompileError::new(format!(
            "{context} expects type {} but found {}",
            describe_type(expected),
            describe_type(actual)
        )))
    }
}

fn types_compatible(
    expected: &Type,
    actual: &Type,
    types: &HashMap<String, TypeDefInfo>,
) -> Result<bool, CompileError> {
    let expected = resolve_aliases(expected, types)?;
    let actual = resolve_aliases(actual, types)?;
    Ok(expected == actual
        || (is_string_compatible(&expected) && is_string_compatible(&actual))
        || can_implicitly_convert_numeric(&actual, &expected))
}

fn is_string_compatible(ty: &Type) -> bool {
    matches!(ty, Type::U8) || matches!(ty, Type::Ref(inner) if inner.as_ref() == &Type::U8)
}

fn is_condition_type(ty: &Type, types: &HashMap<String, TypeDefInfo>) -> Result<bool, CompileError> {
    Ok(is_condition_primitive_type(&resolve_aliases(ty, types)?))
}

fn is_numeric_type(ty: &Type, types: &HashMap<String, TypeDefInfo>) -> Result<bool, CompileError> {
    Ok(is_numeric_primitive_type(&resolve_aliases(ty, types)?))
}

fn is_integer_type(ty: &Type, types: &HashMap<String, TypeDefInfo>) -> Result<bool, CompileError> {
    let ty = resolve_aliases(ty, types)?;
    Ok(is_integer_primitive_type(&ty))
}

fn is_numeric_primitive_type(ty: &Type) -> bool {
    is_integer_primitive_type(ty) || matches!(ty, Type::F32 | Type::F64)
}

fn is_condition_primitive_type(ty: &Type) -> bool {
    matches!(ty, Type::Bool) || is_integer_primitive_type(ty)
}

fn is_integer_primitive_type(ty: &Type) -> bool {
    integer_rank(ty).is_some()
}

fn is_signed_numeric_type(ty: &Type) -> bool {
    is_signed_integer_primitive_type(ty) || matches!(ty, Type::F32 | Type::F64)
}

fn is_signed_integer_primitive_type(ty: &Type) -> bool {
    matches!(ty, Type::I8 | Type::I16 | Type::I32 | Type::I64 | Type::Isize)
}

fn is_unsigned_integer_primitive_type(ty: &Type) -> bool {
    matches!(ty, Type::U16 | Type::U32 | Type::U64 | Type::Usize)
}

fn integer_rank(ty: &Type) -> Option<u8> {
    match ty {
        Type::I8 => Some(1),
        Type::I16 => Some(2),
        Type::I32 => Some(3),
        Type::I64 | Type::Isize => Some(4),
        Type::U16 => Some(2),
        Type::U32 => Some(3),
        Type::U64 | Type::Usize => Some(4),
        _ => None,
    }
}

fn float_rank(ty: &Type) -> Option<u8> {
    match ty {
        Type::F32 => Some(1),
        Type::F64 => Some(2),
        _ => None,
    }
}

fn common_integer_type(lhs: &Type, rhs: &Type) -> Option<Type> {
    if lhs == rhs && is_integer_primitive_type(lhs) {
        return Some(lhs.clone());
    }
    if is_signed_integer_primitive_type(lhs) && is_signed_integer_primitive_type(rhs) {
        return Some(if integer_rank(lhs)? >= integer_rank(rhs)? {
            lhs.clone()
        } else {
            rhs.clone()
        });
    }
    if is_unsigned_integer_primitive_type(lhs) && is_unsigned_integer_primitive_type(rhs) {
        return Some(if integer_rank(lhs)? >= integer_rank(rhs)? {
            lhs.clone()
        } else {
            rhs.clone()
        });
    }
    None
}

fn common_numeric_type(lhs: &Type, rhs: &Type) -> Option<Type> {
    if let Some(common) = common_integer_type(lhs, rhs) {
        return Some(common);
    }
    if lhs == rhs && matches!(lhs, Type::F32 | Type::F64) {
        return Some(lhs.clone());
    }
    if matches!(lhs, Type::F32 | Type::F64) && matches!(rhs, Type::F32 | Type::F64) {
        return Some(if float_rank(lhs)? >= float_rank(rhs)? {
            lhs.clone()
        } else {
            rhs.clone()
        });
    }
    None
}

fn can_implicitly_convert_numeric(actual: &Type, expected: &Type) -> bool {
    common_numeric_type(actual, expected)
        .map(|common| common == *expected)
        .unwrap_or(false)
}

fn deref_refs(mut ty: &Type) -> &Type {
    while let Type::Ref(inner) = ty {
        ty = inner;
    }
    ty
}

fn resolve_aliases(ty: &Type, types: &HashMap<String, TypeDefInfo>) -> Result<Type, CompileError> {
    match ty {
        Type::Named(name) => {
            let Some(type_info) = types.get(name) else {
                return Ok(Type::Named(name.clone()));
            };
            if let Some(alias) = &type_info.alias {
                resolve_aliases(alias, types)
            } else {
                Ok(Type::Named(name.clone()))
            }
        }
        Type::Ref(inner) => Ok(Type::Ref(Box::new(resolve_aliases(inner, types)?))),
        Type::List(inner) => Ok(Type::List(Box::new(resolve_aliases(inner, types)?))),
        other => Ok(other.clone()),
    }
}

fn describe_type(ty: &Type) -> String {
    match ty {
        Type::Void => "void".to_string(),
        Type::Bool => "bool".to_string(),
        Type::I8 => "i8".to_string(),
        Type::I16 => "i16".to_string(),
        Type::I32 => "i32".to_string(),
        Type::I64 => "i64".to_string(),
        Type::Isize => "isize".to_string(),
        Type::U16 => "u16".to_string(),
        Type::U32 => "u32".to_string(),
        Type::U64 => "u64".to_string(),
        Type::Usize => "usize".to_string(),
        Type::U8 => "u8".to_string(),
        Type::F32 => "f32".to_string(),
        Type::F64 => "f64".to_string(),
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
    fn accepts_string_literals_as_ref_u8() {
        let source = "pub def main() void\n\tval message ref(u8) = \"hello\"\n\t@puts(message)\n\t@print(\"{s}\", {message})\nend\n";
        let program = parse_program(lex(source).unwrap()).unwrap();

        analyze(&program).unwrap();
    }

    #[test]
    fn accepts_indexing_and_extern_calls() {
        let source = "type Grid\n\tcells list[list[i32]]\nend\nextern def sleep(t u32) void :: \"sleep\"\npub def main() void\n\tvar grid = Grid(cells: [[1]])\n\tgrid.cells[0][0] = 2\n\tsleep(100 as u32)\nend\n";
        let program = parse_program(lex(source).unwrap()).unwrap();

        analyze(&program).unwrap();
    }

    #[test]
    fn accepts_regular_type_aliases() {
        let source = "type Count = i32\npub def main() void\n\tval count Count = 1\n\t@print(\"{d}\", {count})\nend\n";
        let program = parse_program(lex(source).unwrap()).unwrap();

        analyze(&program).unwrap();
    }

    #[test]
    fn accepts_bitwise_and_shift_operators() {
        let source = "type Pair\n\tleft i32\n\tright i32\nend\npub def main() void\n\tvar mask i32 = (1 << 3) | (2 & 7) ^ (8 >> 1)\n\tmask &= 15\n\tmask |= 1\n\tmask ^= 2\n\tmask -= 1\n\tval product = (6 * 7) % 5\n\tval pair = Pair(mask, product)\n\tif !(1 == 0) && pair.left - pair.right == 10\n\t\t@print(\"{d} {d}\", {mask, product})\n\tend\nend\n";
        let program = parse_program(lex(source).unwrap()).unwrap();

        analyze(&program).unwrap();
    }

    #[test]
    fn accepts_multiply_assignment() {
        let source = "pub def main() void\n\tvar value i32 = 3\n\tvalue *= 2\nend\n";
        let program = parse_program(lex(source).unwrap()).unwrap();

        analyze(&program).unwrap();
    }

    #[test]
    fn accepts_numeric_widening_and_float_print_markers() {
        let source = "pub def main() void\n\tval wide i64 = 1\n\tval sum i64 = wide + 2\n\tval small f32 = 3 as f32\n\tval big f64 = 4 as f64\n\t@print(\"{d} {f} {lf}\", {sum, small, big})\nend\n";
        let program = parse_program(lex(source).unwrap()).unwrap();

        analyze(&program).unwrap();
    }

    #[test]
    fn accepts_bool_return_types_from_comparisons() {
        let source = "pub def same(x i32, y i32) bool\n\treturn x == y\nend\n";
        let program = parse_program(lex(source).unwrap()).unwrap();

        analyze(&program).unwrap();
    }
}
