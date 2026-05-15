use std::collections::HashMap;

use crate::{
    CompileError,
    ast::{BinaryOp, Expr, Function, Program, Stmt, Type},
};

#[derive(Debug, Clone)]
pub struct FunctionSig {
    pub params: Vec<Type>,
    pub return_type: Type,
}

#[derive(Debug, Clone)]
pub struct ProgramInfo {
    pub functions: HashMap<String, FunctionSig>,
    pub locals: HashMap<String, HashMap<String, Type>>,
}

#[derive(Debug, Clone)]
struct LocalBinding {
    ty: Type,
    mutable: bool,
}

pub fn analyze(program: &Program) -> Result<ProgramInfo, CompileError> {
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
        functions.insert(
            function.name.clone(),
            FunctionSig {
                params: function
                    .params
                    .iter()
                    .map(|param| param.ty.clone())
                    .collect(),
                return_type: function.return_type.clone(),
            },
        );
    }

    let mut locals = HashMap::new();
    for function in &program.functions {
        analyze_function(function, &functions, &mut locals)?;
    }

    Ok(ProgramInfo { functions, locals })
}

fn analyze_function(
    function: &Function,
    functions: &HashMap<String, FunctionSig>,
    locals: &mut HashMap<String, HashMap<String, Type>>,
) -> Result<(), CompileError> {
    let mut scope = HashMap::new();
    let mut function_locals = HashMap::new();

    for param in &function.params {
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
    scope: &mut HashMap<String, LocalBinding>,
    function_locals: &mut HashMap<String, Type>,
) -> Result<(), CompileError> {
    match stmt {
        Stmt::VarDecl {
            mutable,
            name,
            init,
        } => {
            if scope.contains_key(name) {
                return Err(CompileError::new(format!(
                    "duplicate local binding `{name}` in function `{function_name}`"
                )));
            }
            let ty = infer_expr_type(init, functions, scope)?;
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
            let target_ty = infer_lvalue_type(target, functions, scope)?;
            let value_ty = infer_expr_type(value, functions, scope)?;
            expect_same_type(&target_ty, &value_ty, "assignment")?;
        }
        Stmt::AddAssign { target, value } => {
            let target_ty = infer_mutable_target(target, functions, scope)?;
            let value_ty = infer_expr_type(value, functions, scope)?;
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
                let actual = infer_expr_type(expr, functions, scope)?;
                expect_same_type(expected, &actual, "return")?;
            }
            (_, None) => {
                return Err(CompileError::new("non-void functions must return a value"));
            }
        },
        Stmt::Expr(expr) => {
            infer_expr_type(expr, functions, scope)?;
        }
        Stmt::For {
            pragma: _,
            var_name,
            start,
            end,
            body,
        } => {
            let start_ty = infer_expr_type(start, functions, scope)?;
            let end_ty = infer_expr_type(end, functions, scope)?;
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
    scope: &HashMap<String, LocalBinding>,
) -> Result<Type, CompileError> {
    match expr {
        Expr::Int(_) => Ok(Type::I32),
        Expr::String(_) => Ok(Type::U8),
        Expr::Path(path) => match path.as_slice() {
            [name] => scope
                .get(name)
                .map(|binding| binding.ty.clone())
                .ok_or_else(|| CompileError::new(format!("unknown name `{name}`"))),
            _ => Err(CompileError::new(format!(
                "qualified path `{}` is only valid as a builtin callee",
                path.join(".")
            ))),
        },
        Expr::Binary { lhs, op, rhs } => {
            let lhs_ty = infer_expr_type(lhs, functions, scope)?;
            let rhs_ty = infer_expr_type(rhs, functions, scope)?;
            match op {
                BinaryOp::Add if lhs_ty == Type::I32 && rhs_ty == Type::I32 => Ok(Type::I32),
                BinaryOp::Add => Err(CompileError::new(
                    "`+` currently requires both operands to have type i32",
                )),
            }
        }
        Expr::Pack(_) => Err(CompileError::new(
            "packed `{...}` expressions are only valid as builtin.print arguments",
        )),
        Expr::Call { callee, args } => analyze_call(callee, args, functions, scope),
    }
}

fn analyze_call(
    callee: &Expr,
    args: &[Expr],
    functions: &HashMap<String, FunctionSig>,
    scope: &HashMap<String, LocalBinding>,
) -> Result<Type, CompileError> {
    if let Some(path) = callee.as_path() {
        if path.len() == 2 && path[0] == "builtin" {
            return analyze_builtin(&path[1], args, functions, scope);
        }
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
                let actual = infer_expr_type(arg, functions, scope)?;
                expect_same_type(expected, &actual, "function argument")?;
            }
            return Ok(signature.return_type.clone());
        }
    }

    Err(CompileError::new(
        "only direct function calls and builtin calls are currently supported",
    ))
}

fn analyze_builtin(
    name: &str,
    args: &[Expr],
    functions: &HashMap<String, FunctionSig>,
    scope: &HashMap<String, LocalBinding>,
) -> Result<Type, CompileError> {
    match name {
        "puts" => {
            if args.len() != 1 {
                return Err(CompileError::new(
                    "builtin.puts expects exactly one argument",
                ));
            }
            let arg_ty = infer_expr_type(&args[0], functions, scope)?;
            expect_same_type(&Type::U8, &arg_ty, "builtin.puts")?;
            Ok(Type::Void)
        }
        "print" => {
            if args.is_empty() {
                return Err(CompileError::new(
                    "builtin.print expects at least a format string argument",
                ));
            }
            let Expr::String(format) = &args[0] else {
                return Err(CompileError::new(
                    "builtin.print requires a string literal as its first argument",
                ));
            };
            let flattened = flatten_print_args(&args[1..]);
            let markers = parse_format_markers(format)?;
            if markers.len() != flattened.len() {
                return Err(CompileError::new(format!(
                    "builtin.print format expects {} values but received {}",
                    markers.len(),
                    flattened.len()
                )));
            }
            for (marker, arg) in markers.iter().zip(flattened.iter()) {
                let arg_ty = infer_expr_type(arg, functions, scope)?;
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
                return Err(CompileError::new(
                    "builtin.addr expects exactly one argument",
                ));
            }
            let inner = infer_lvalue_type(&args[0], functions, scope)?;
            Ok(Type::Ref(Box::new(inner)))
        }
        "deref" => {
            if args.len() != 1 {
                return Err(CompileError::new(
                    "builtin.deref expects exactly one argument",
                ));
            }
            let arg_ty = infer_expr_type(&args[0], functions, scope)?;
            match arg_ty {
                Type::Ref(inner) => Ok(*inner),
                other => Err(CompileError::new(format!(
                    "builtin.deref requires a ref(...) argument, got {}",
                    describe_type(&other)
                ))),
            }
        }
        _ => Err(CompileError::new(format!(
            "unsupported builtin intrinsic `builtin.{name}`"
        ))),
    }
}

fn infer_lvalue_type(
    expr: &Expr,
    functions: &HashMap<String, FunctionSig>,
    scope: &HashMap<String, LocalBinding>,
) -> Result<Type, CompileError> {
    infer_mutable_target(expr, functions, scope)
}

fn infer_mutable_target(
    expr: &Expr,
    functions: &HashMap<String, FunctionSig>,
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
        Expr::Call { callee, args } => {
            if let Some(path) = callee.as_path() {
                if path.len() == 2 && path[0] == "builtin" && path[1] == "deref" {
                    return analyze_builtin("deref", args, functions, scope);
                }
            }
            Err(CompileError::new(
                "only names and builtin.deref(...) may appear on the left-hand side of an assignment",
            ))
        }
        _ => Err(CompileError::new(
            "expected an assignable expression on the left-hand side",
        )),
    }
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
                    "unterminated format marker in builtin.print",
                ));
            };
            let Some('}') = chars.next() else {
                return Err(CompileError::new(
                    "unterminated format marker in builtin.print",
                ));
            };
            if !matches!(marker, 'd' | 'p' | 's') {
                return Err(CompileError::new(format!(
                    "unsupported builtin.print marker `{{{marker}}}`"
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
        's' => matches!(ty, Type::U8),
        'p' => matches!(ty, Type::U8 | Type::Ref(_)),
        _ => false,
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
        Type::Ref(inner) => format!("ref({})", describe_type(inner)),
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
}
