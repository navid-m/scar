use std::collections::HashSet;

use crate::{
    CompileError,
    ast::{BinaryOp, Expr, Function, Program, Stmt, Type, TypeDef, UnaryOp},
    sema::ProgramInfo,
};

pub fn generate_c(
    program: &Program,
    info: &ProgramInfo,
    install_debug_handlers: bool,
) -> Result<String, CompileError> {
    let mut output = String::new();
    output.push_str("#include <inttypes.h>\n");
    output.push_str("#include <signal.h>\n");
    output.push_str("#include <stdint.h>\n");
    output.push_str("#include <stdio.h>\n");
    output.push_str("#include <stdlib.h>\n");
    output.push_str("#include <string.h>\n\n");

    if install_debug_handlers {
        output.push_str("#if !defined(_WIN32)\n");
        output.push_str("#include <execinfo.h>\n");
        output.push_str("#include <unistd.h>\n");
        output.push_str("#endif\n\n");
    }

    output.push_str(&render_runtime_prelude(install_debug_handlers));
    let list_types = collect_list_types(program, info);
    if !list_types.is_empty() {
        for list_ty in &list_types {
            output.push_str(&render_list_support(list_ty)?);
            output.push('\n');
        }
    }

    for type_def in &program.type_defs {
        output.push_str(&render_type_def(type_def));
        output.push('\n');
    }

    for function in &program.functions {
        if function.extern_name.is_some() {
            continue;
        }
        output.push_str(&render_signature(function, info));
        output.push_str(";\n");
    }
    output.push('\n');

    for function in &program.functions {
        if function.extern_name.is_some() {
            continue;
        }
        render_function(&mut output, function, info, install_debug_handlers)?;
        output.push('\n');
    }

    Ok(output)
}

fn render_runtime_prelude(install_debug_handlers: bool) -> String {
    let mut output = String::new();
    output.push_str("static void scar_runtime_panic(const char *message) {\n");
    output.push_str("    fprintf(stderr, \"scar: panic: %s\\n\", message);\n");
    output.push_str("    exit(1);\n");
    output.push_str("}\n\n");
    output.push_str("static void *scar_runtime_alloc(size_t size) {\n");
    output.push_str("    void *ptr = malloc(size);\n");
    output.push_str("    if (ptr == NULL) {\n");
    output.push_str("        scar_runtime_panic(\"allocation failed\");\n");
    output.push_str("    }\n");
    output.push_str("    return ptr;\n");
    output.push_str("}\n\n");
    output.push_str("static void *scar_runtime_realloc(void *ptr, size_t size) {\n");
    output.push_str("    void *resized = realloc(ptr, size);\n");
    output.push_str("    if (resized == NULL) {\n");
    output.push_str("        scar_runtime_panic(\"reallocation failed\");\n");
    output.push_str("    }\n");
    output.push_str("    return resized;\n");
    output.push_str("}\n\n");
    output.push_str("static void scar_runtime_free(void *ptr) {\n");
    output.push_str("    free(ptr);\n");
    output.push_str("}\n\n");
    if install_debug_handlers {
        output.push_str("static void scar_runtime_signal_handler(int signal_number) {\n");
        output.push_str("    fprintf(stderr, \"scar: fatal signal %d\\n\", signal_number);\n");
        output.push_str("#if !defined(_WIN32)\n");
        output.push_str("    void *frames[64];\n");
        output.push_str("    int frame_count = backtrace(frames, 64);\n");
        output.push_str("    backtrace_symbols_fd(frames, frame_count, STDERR_FILENO);\n");
        output.push_str("#endif\n");
        output.push_str("    _Exit(128 + signal_number);\n");
        output.push_str("}\n\n");
        output.push_str("static void scar_runtime_install_signal_handlers(void) {\n");
        output.push_str("    signal(SIGSEGV, scar_runtime_signal_handler);\n");
        output.push_str("#ifdef SIGBUS\n");
        output.push_str("    signal(SIGBUS, scar_runtime_signal_handler);\n");
        output.push_str("#endif\n");
        output.push_str("#ifdef SIGILL\n");
        output.push_str("    signal(SIGILL, scar_runtime_signal_handler);\n");
        output.push_str("#endif\n");
        output.push_str("#ifdef SIGABRT\n");
        output.push_str("    signal(SIGABRT, scar_runtime_signal_handler);\n");
        output.push_str("#endif\n");
        output.push_str("}\n\n");
    }
    output
}

fn render_function(
    output: &mut String,
    function: &Function,
    info: &ProgramInfo,
    install_debug_handlers: bool,
) -> Result<(), CompileError> {
    let mut next_temp_id = 0usize;
    output.push_str(&render_signature(function, info));
    output.push_str(" {\n");
    if function.name == "main" && install_debug_handlers {
        indent(output, 1);
        output.push_str("scar_runtime_install_signal_handlers();\n");
    }
    for stmt in &function.body {
        render_stmt(output, stmt, function, info, 1, &mut next_temp_id)?;
    }
    if function.name == "main" && function.return_type == Type::Void {
        indent(output, 1);
        output.push_str("return 0;\n");
    }
    output.push_str("}\n");
    Ok(())
}

fn render_type_def(type_def: &TypeDef) -> String {
    if let Some(alias) = &type_def.alias {
        return format!("typedef {} {};\n", c_type(alias), type_def.name);
    }

    let mut output = String::new();
    if type_def.is_extern {
        output.push_str("typedef struct __attribute__((packed)) {\n");
    } else {
        output.push_str("typedef struct {\n");
    }
    for field in &type_def.fields {
        output.push_str("    ");
        output.push_str(&c_type(&field.ty));
        output.push(' ');
        output.push_str(&field.name);
        output.push_str(";\n");
    }
    output.push_str("} ");
    output.push_str(&type_def.name);
    output.push_str(";\n");
    output
}

fn render_signature(function: &Function, info: &ProgramInfo) -> String {
    let return_type = if function.name == "main" {
        "int".to_string()
    } else {
        c_type(&function.return_type)
    };
    let symbol = info
        .function_symbols
        .get(&function.name)
        .cloned()
        .unwrap_or_else(|| function.name.clone());
    let params = if function.params.is_empty() {
        "void".to_string()
    } else {
        function
            .params
            .iter()
            .map(|param| format!("{} {}", c_type(&param.ty), param.name))
            .collect::<Vec<_>>()
            .join(", ")
    };
    let prefix = if function.extern_name.is_some() {
        "extern "
    } else {
        ""
    };
    format!("{prefix}{return_type} {symbol}({params})")
}

fn render_stmt(
    output: &mut String,
    stmt: &Stmt,
    function: &Function,
    info: &ProgramInfo,
    level: usize,
    next_temp_id: &mut usize,
) -> Result<(), CompileError> {
    match stmt {
        Stmt::VarDecl {
            mutable,
            name,
            init,
            ..
        } => {
            let ty = info
                .locals
                .get(&function.name)
                .and_then(|locals| locals.get(name))
                .ok_or_else(|| {
                    CompileError::new(format!(
                        "missing inferred type for local `{name}` in `{}`",
                        function.name
                    ))
                })?;
            indent(output, level);
            if !mutable {
                output.push_str("const ");
            }
            output.push_str(&c_type(ty));
            output.push(' ');
            output.push_str(name);
            output.push_str(" = ");
            output.push_str(&render_expr_with_hint(init, function, info, Some(ty))?);
            output.push_str(";\n");
        }
        Stmt::Assign { target, value, .. } => {
            indent(output, level);
            output.push_str(&render_expr(target, function, info)?);
            output.push_str(" = ");
            output.push_str(&render_expr(value, function, info)?);
            output.push_str(";\n");
        }
        Stmt::AddAssign { target, value, .. } => {
            indent(output, level);
            output.push_str(&render_expr(target, function, info)?);
            output.push_str(" += ");
            output.push_str(&render_expr(value, function, info)?);
            output.push_str(";\n");
        }
        Stmt::MulAssign { target, value, .. } => {
            indent(output, level);
            output.push_str(&render_expr(target, function, info)?);
            output.push_str(" *= ");
            output.push_str(&render_expr(value, function, info)?);
            output.push_str(";\n");
        }
        Stmt::SubAssign { target, value, .. } => {
            indent(output, level);
            output.push_str(&render_expr(target, function, info)?);
            output.push_str(" -= ");
            output.push_str(&render_expr(value, function, info)?);
            output.push_str(";\n");
        }
        Stmt::DivAssign { target, value, .. } => {
            indent(output, level);
            output.push_str(&render_expr(target, function, info)?);
            output.push_str(" /= ");
            output.push_str(&render_expr(value, function, info)?);
            output.push_str(";\n");
        }
        Stmt::BitAndAssign { target, value, .. } => {
            indent(output, level);
            output.push_str(&render_expr(target, function, info)?);
            output.push_str(" &= ");
            output.push_str(&render_expr(value, function, info)?);
            output.push_str(";\n");
        }
        Stmt::BitOrAssign { target, value, .. } => {
            indent(output, level);
            output.push_str(&render_expr(target, function, info)?);
            output.push_str(" |= ");
            output.push_str(&render_expr(value, function, info)?);
            output.push_str(";\n");
        }
        Stmt::BitXorAssign { target, value, .. } => {
            indent(output, level);
            output.push_str(&render_expr(target, function, info)?);
            output.push_str(" ^= ");
            output.push_str(&render_expr(value, function, info)?);
            output.push_str(";\n");
        }
        Stmt::Assert {
            line,
            column,
            condition,
        } => {
            indent(output, level);
            output.push_str("if (!(");
            output.push_str(&render_expr(condition, function, info)?);
            output.push_str(")) {\n");
            indent(output, level + 1);
            output.push_str(&format!(
                "scar_runtime_panic(\"assertion failed in {} at {}:{}\");\n",
                escape_c_string(&function.name),
                line,
                column
            ));
            indent(output, level);
            output.push_str("}\n");
        }
        Stmt::Return { value: None, .. } => {
            indent(output, level);
            output.push_str("return;\n");
        }
        Stmt::Return {
            value: Some(value), ..
        } => {
            indent(output, level);
            output.push_str("return ");
            output.push_str(&render_expr(value, function, info)?);
            output.push_str(";\n");
        }
        Stmt::If {
            condition,
            then_body,
            else_body,
            ..
        } => {
            indent(output, level);
            output.push_str("if (");
            output.push_str(&render_expr(condition, function, info)?);
            output.push_str(") {\n");
            for stmt in then_body {
                render_stmt(output, stmt, function, info, level + 1, next_temp_id)?;
            }
            indent(output, level);
            output.push('}');
            if else_body.is_empty() {
                output.push('\n');
            } else {
                output.push_str(" else {\n");
                for stmt in else_body {
                    render_stmt(output, stmt, function, info, level + 1, next_temp_id)?;
                }
                indent(output, level);
                output.push_str("}\n");
            }
        }
        Stmt::Expr { expr, .. } => {
            indent(output, level);
            output.push_str(&render_expr(expr, function, info)?);
            output.push_str(";\n");
        }
        Stmt::ForRange {
            pragma,
            var_name,
            start,
            end,
            body,
            ..
        } => {
            indent(output, level);
            if let Some(pragma) = pragma {
                output.push_str("#pragma ");
                output.push_str(pragma);
                output.push('\n');
                indent(output, level);
            }
            output.push_str("for (int32_t ");
            output.push_str(var_name);
            output.push_str(" = ");
            output.push_str(&render_expr(start, function, info)?);
            output.push_str("; ");
            output.push_str(var_name);
            output.push_str(" <= ");
            output.push_str(&render_expr(end, function, info)?);
            output.push_str("; ++");
            output.push_str(var_name);
            output.push_str(") {\n");
            for stmt in body {
                render_stmt(output, stmt, function, info, level + 1, next_temp_id)?;
            }
            indent(output, level);
            output.push_str("}\n");
        }
        Stmt::ForEach {
            var_name,
            iterable,
            body,
            ..
        } => {
            let iterable_ty = infer_codegen_expr_type(iterable, function, info)?;
            let Type::List(element_ty) = deref_refs(&iterable_ty) else {
                return Err(CompileError::new(format!(
                    "expected list iterable during code generation, got {}",
                    describe_type(&iterable_ty)
                )));
            };
            let temp_id = *next_temp_id;
            *next_temp_id += 1;
            let iter_name = format!("__scar_iter_{temp_id}");
            let index_name = format!("__scar_index_{temp_id}");

            indent(output, level);
            output.push_str("{\n");
            indent(output, level + 1);
            output.push_str(&c_type(&iterable_ty));
            output.push(' ');
            output.push_str(&iter_name);
            output.push_str(" = ");
            output.push_str(&render_expr(iterable, function, info)?);
            output.push_str(";\n");
            indent(output, level + 1);
            output.push_str("for (int32_t ");
            output.push_str(&index_name);
            output.push_str(" = 0; ");
            output.push_str(&index_name);
            output.push_str(" < ");
            output.push_str(&iter_name);
            output.push_str(".len; ++");
            output.push_str(&index_name);
            output.push_str(") {\n");
            indent(output, level + 2);
            output.push_str(&c_type(element_ty));
            output.push(' ');
            output.push_str(var_name);
            output.push_str(" = ");
            output.push_str(&iter_name);
            output.push_str(".data[");
            output.push_str(&index_name);
            output.push_str("];\n");
            for stmt in body {
                render_stmt(output, stmt, function, info, level + 2, next_temp_id)?;
            }
            indent(output, level + 1);
            output.push_str("}\n");
            indent(output, level);
            output.push_str("}\n");
        }
        Stmt::Loop { body, .. } => {
            indent(output, level);
            output.push_str("for (;;) {\n");
            for stmt in body {
                render_stmt(output, stmt, function, info, level + 1, next_temp_id)?;
            }
            indent(output, level);
            output.push_str("}\n");
        }
        Stmt::Continue { .. } => {
            indent(output, level);
            output.push_str("continue;\n");
        }
    }
    Ok(())
}

fn render_expr(
    expr: &Expr,
    function: &Function,
    info: &ProgramInfo,
) -> Result<String, CompileError> {
    render_expr_with_hint(expr, function, info, None)
}

fn render_expr_with_hint(
    expr: &Expr,
    function: &Function,
    info: &ProgramInfo,
    hint: Option<&Type>,
) -> Result<String, CompileError> {
    match expr {
        Expr::Int(value) => Ok(value.to_string()),
        Expr::String(value) => Ok(format!("\"{}\"", escape_c_string(value))),
        Expr::ListLiteral(values) => {
            let list_ty = match hint {
                Some(Type::List(_)) => hint.cloned().ok_or_else(|| {
                    CompileError::new("missing list type hint during code generation")
                })?,
                _ => infer_codegen_expr_type(expr, function, info)?,
            };
            render_list_literal(values, &list_ty, function, info)
        }
        Expr::Index { base, index } => render_index_expr(base, index, function, info),
        Expr::FieldAccess { base, field } => render_field_access(base, field, function, info),
        Expr::StructInit { name, fields } => {
            let rendered_fields = fields
                .iter()
                .map(|field| {
                    Ok(format!(
                        ".{} = {}",
                        field.name,
                        render_expr(&field.value, function, info)?
                    ))
                })
                .collect::<Result<Vec<_>, CompileError>>()?;
            Ok(format!("({name}){{ {} }}", rendered_fields.join(", ")))
        }
        Expr::BuiltinCall { name, args } => render_builtin_call(name, args, function, info),
        Expr::MethodCall {
            receiver,
            method,
            args,
        } => render_list_method_call(method, receiver, args, function, info),
        Expr::Path(path) => match path.as_slice() {
            [name] => Ok(name.clone()),
            _ => Err(CompileError::new(format!(
                "unsupported qualified expression `{}` in code generation",
                path.join(".")
            ))),
        },
        Expr::Call { callee, args } => render_call(callee, args, function, info),
        Expr::Cast { expr, ty } => {
            if matches!(expr.as_ref(), Expr::ListLiteral(_)) {
                return render_expr_with_hint(expr, function, info, Some(ty));
            }
            Ok(format!(
                "(({})({}))",
                c_type(ty),
                render_expr(expr, function, info)?
            ))
        }
        Expr::Unary { op, expr } => match op {
            UnaryOp::Neg => Ok(format!("(-({}))", render_expr(expr, function, info)?)),
            UnaryOp::LogicalNot => Ok(format!("(!({}))", render_expr(expr, function, info)?)),
        },
        Expr::Pack(_) => Err(CompileError::new(
            "packed `{...}` expressions are only valid inside @print",
        )),
        Expr::Binary { lhs, op, rhs } => {
            if *op == BinaryOp::ShiftRight {
                return render_logical_shift_right(lhs, rhs, function, info);
            }
            let operator = match op {
                BinaryOp::Add => "+",
                BinaryOp::Subtract => "-",
                BinaryOp::Divide => "/",
                BinaryOp::Multiply => "*",
                BinaryOp::Modulo => "%",
                BinaryOp::LogicalAnd => "&&",
                BinaryOp::LogicalOr => "||",
                BinaryOp::BitAnd => "&",
                BinaryOp::BitOr => "|",
                BinaryOp::BitXor => "^",
                BinaryOp::ShiftLeft => "<<",
                BinaryOp::LessThan => "<",
                BinaryOp::LessEqual => "<=",
                BinaryOp::GreaterThan => ">",
                BinaryOp::GreaterEqual => ">=",
                BinaryOp::Equal => "==",
                BinaryOp::ShiftRight => unreachable!("handled above"),
            };
            Ok(format!(
                "(({}) {} ({}))",
                render_expr(lhs, function, info)?,
                operator,
                render_expr(rhs, function, info)?
            ))
        }
    }
}

fn render_field_access(
    base: &Expr,
    field: &str,
    function: &Function,
    info: &ProgramInfo,
) -> Result<String, CompileError> {
    let base_ty = infer_codegen_expr_type(base, function, info)?;
    let rendered_base = render_expr(base, function, info)?;
    if is_reference_like(&base_ty) {
        Ok(format!("({rendered_base})->{field}"))
    } else {
        Ok(format!("({rendered_base}).{field}"))
    }
}

fn render_index_expr(
    base: &Expr,
    index: &Expr,
    function: &Function,
    info: &ProgramInfo,
) -> Result<String, CompileError> {
    let base_ty = infer_codegen_expr_type(base, function, info)?;
    let element_ty = list_element_type(&base_ty)?;
    let helper_prefix = list_helper_prefix(element_ty);
    Ok(format!(
        "(*{helper_prefix}_at({}, {}))",
        render_list_pointer(base, &base_ty, function, info)?,
        render_expr(index, function, info)?
    ))
}

fn render_call(
    callee: &Expr,
    args: &[Expr],
    function: &Function,
    info: &ProgramInfo,
) -> Result<String, CompileError> {
        if let Some(path) = callee.as_path() {
            if path.len() == 1 {
                let rendered_args = args
                    .iter()
                    .map(|arg| render_expr(arg, function, info))
                    .collect::<Result<Vec<_>, _>>()?;
                if let Some(symbol) = info.function_symbols.get(&path[0]).cloned() {
                    return Ok(format!("{symbol}({})", rendered_args.join(", ")));
                }
                if let Some(type_info) = info.types.get(&path[0]) {
                    if type_info.alias.is_some() {
                        return Err(CompileError::new(format!(
                            "type `{}` is an alias and cannot be initialized like a struct",
                            path[0]
                        )));
                    }
                    if type_info.fields.len() != args.len() {
                        return Err(CompileError::new(format!(
                            "type `{}` expects {} constructor arguments but received {}",
                            path[0],
                            type_info.fields.len(),
                            args.len()
                        )));
                    }
                    return Ok(format!("({}){{ {} }}", path[0], rendered_args.join(", ")));
                }
                return Err(CompileError::new(format!("unknown function `{}`", path[0])));
            }
        }

    Err(CompileError::new(
        "only direct function calls and @builtin calls are currently supported in codegen",
    ))
}

fn render_builtin_call(
    name: &str,
    args: &[Expr],
    function: &Function,
    info: &ProgramInfo,
) -> Result<String, CompileError> {
    match name {
        "append" | "capacity" | "reserve" | "set" | "insert" | "remove" | "clear" => {
            if args.is_empty() {
                return Err(CompileError::new(format!(
                    "@{name} expects a list receiver as its first argument",
                )));
            }
            render_list_method_call(name, &args[0], &args[1..], function, info)
        }
        "puts" => Ok(format!("puts({})", render_expr(&args[0], function, info)?)),
        "print" => {
            let Expr::String(format) = &args[0] else {
                return Err(CompileError::new(
                    "@print requires a string literal as its first argument",
                ));
            };
            let flattened = flatten_print_args(&args[1..]);
            let arg_types = flattened
                .iter()
                .map(|arg| resolve_codegen_aliases(&infer_codegen_expr_type(arg, function, info)?, info))
                .collect::<Result<Vec<_>, CompileError>>()?;
            let (converted, markers) = convert_format_string(format, &arg_types)?;
            let mut rendered_args = vec![format!("\"{}\"", escape_c_string(&converted))];
            for ((marker, expr), arg_ty) in markers
                .into_iter()
                .zip(flattened.into_iter())
                .zip(arg_types.into_iter())
            {
                let rendered = render_expr(expr, function, info)?;
                rendered_args.push(render_print_value(marker, &arg_ty, &rendered)?);
            }
            Ok(format!("printf({})", rendered_args.join(", ")))
        }
        "addr" => Ok(format!("(&{})", render_expr(&args[0], function, info)?)),
        "alloc" => Ok(format!("scar_runtime_alloc((size_t)({}))", render_expr(&args[0], function, info)?)),
        "realloc" => Ok(format!(
            "scar_runtime_realloc((void *)({}), (size_t)({}))",
            render_expr(&args[0], function, info)?,
            render_expr(&args[1], function, info)?
        )),
        "free" => Ok(format!("scar_runtime_free((void *)({}))", render_expr(&args[0], function, info)?)),
        "deref" => {
            let arg_ty = infer_codegen_expr_type(&args[0], function, info)?;
            if is_codegen_string_compatible(&arg_ty) {
                Ok(render_expr(&args[0], function, info)?)
            } else {
                Ok(format!("(*({}))", render_expr(&args[0], function, info)?))
            }
        }
        _ => Err(CompileError::new(format!(
            "unsupported builtin intrinsic `@{name}` during code generation"
        ))),
    }
}

fn render_list_method_call(
    method: &str,
    receiver: &Expr,
    args: &[Expr],
    function: &Function,
    info: &ProgramInfo,
) -> Result<String, CompileError> {
    let receiver_ty = infer_codegen_expr_type(receiver, function, info)?;
    let element_ty = list_element_type(&receiver_ty)?;
    let helper_prefix = list_helper_prefix(element_ty);
    let receiver_ptr = render_list_pointer(receiver, &receiver_ty, function, info)?;

    match method {
        "append" => Ok(format!(
            "{helper_prefix}_append({receiver_ptr}, {})",
            render_expr(&args[0], function, info)?
        )),
        "capacity" => Ok(format!("{helper_prefix}_capacity({receiver_ptr})")),
        "reserve" => Ok(format!(
            "{helper_prefix}_reserve({receiver_ptr}, {})",
            render_expr(&args[0], function, info)?
        )),
        "set" => Ok(format!(
            "{helper_prefix}_set({receiver_ptr}, {}, {})",
            render_expr(&args[0], function, info)?,
            render_expr(&args[1], function, info)?
        )),
        "insert" => Ok(format!(
            "{helper_prefix}_insert({receiver_ptr}, {}, {})",
            render_expr(&args[0], function, info)?,
            render_expr(&args[1], function, info)?
        )),
        "remove" => Ok(format!(
            "{helper_prefix}_remove({receiver_ptr}, {})",
            render_expr(&args[0], function, info)?
        )),
        "clear" => Ok(format!("{helper_prefix}_clear({receiver_ptr})")),
        _ => Err(CompileError::new(format!(
            "unsupported list accessor `@{method}` during code generation"
        ))),
    }
}

fn render_list_pointer(
    receiver: &Expr,
    receiver_ty: &Type,
    function: &Function,
    info: &ProgramInfo,
) -> Result<String, CompileError> {
    let rendered = render_expr(receiver, function, info)?;
    match deref_refs(receiver_ty) {
        Type::List(_) => {
            if matches!(receiver_ty, Type::Ref(inner) if matches!(inner.as_ref(), Type::List(_))) {
                Ok(format!("({rendered})"))
            } else {
                Ok(format!("&({rendered})"))
            }
        }
        other => Err(CompileError::new(format!(
            "expected list receiver during code generation, got {}",
            describe_type(other)
        ))),
    }
}

fn render_list_literal(
    values: &[Expr],
    ty: &Type,
    function: &Function,
    info: &ProgramInfo,
) -> Result<String, CompileError> {
    let element_ty = list_element_type(ty)?;
    let helper_prefix = list_helper_prefix(element_ty);
    if values.is_empty() {
        return Ok(format!("{helper_prefix}_new()"));
    }
    let rendered_values = values
        .iter()
        .map(|value| render_expr(value, function, info))
        .collect::<Result<Vec<_>, _>>()?;
    Ok(format!(
        "{helper_prefix}_from_array(({}[]){{ {} }}, {})",
        c_type(element_ty),
        rendered_values.join(", "),
        values.len()
    ))
}

fn infer_codegen_expr_type(
    expr: &Expr,
    function: &Function,
    info: &ProgramInfo,
) -> Result<Type, CompileError> {
    match expr {
        Expr::Int(_) => Ok(Type::I32),
        Expr::String(_) => Ok(Type::U8),
        Expr::ListLiteral(values) => {
            if values.is_empty() {
                return Err(CompileError::new(
                    "cannot infer the element type of an empty list literal during code generation",
                ));
            }
            let first_ty = infer_codegen_expr_type(&values[0], function, info)?;
            for value in &values[1..] {
                let ty = infer_codegen_expr_type(value, function, info)?;
                if ty != first_ty {
                    return Err(CompileError::new(format!(
                        "list literal contains mixed element types {} and {} during code generation",
                        describe_type(&first_ty),
                        describe_type(&ty)
                    )));
                }
            }
            Ok(Type::List(Box::new(first_ty)))
        }
        Expr::Index { base, .. } => {
            infer_index_type(&infer_codegen_expr_type(base, function, info)?)
        }
        Expr::Path(path) => match path.as_slice() {
            [name] => info
                .locals
                .get(&function.name)
                .and_then(|locals| locals.get(name))
                .cloned()
                .or_else(|| {
                    function
                        .params
                        .iter()
                        .find(|param| param.name == *name)
                        .map(|param| param.ty.clone())
                })
                .or_else(|| info.functions.get(name).map(|sig| sig.return_type.clone()))
                .ok_or_else(|| {
                    CompileError::new(format!("unknown expression `{name}` in code generation"))
                }),
            _ => Err(CompileError::new(format!(
                "unsupported qualified expression `{}` in code generation",
                path.join(".")
            ))),
        },
        Expr::FieldAccess { base, field } => {
            let base_ty = infer_codegen_expr_type(base, function, info)?;
            match deref_refs(&base_ty) {
                Type::Named(name) => info
                    .types
                    .get(name)
                    .and_then(|type_info| type_info.field_map.get(field))
                    .cloned()
                    .ok_or_else(|| {
                        CompileError::new(format!("type `{name}` has no field `{field}`"))
                    }),
                other => Err(CompileError::new(format!(
                    "field access requires a named type during code generation, got {}",
                    describe_type(other)
                ))),
            }
        }
        Expr::StructInit { name, .. } => Ok(Type::Named(name.clone())),
        Expr::BuiltinCall { name, args } => infer_builtin_type(name, args, function, info),
        Expr::MethodCall {
            receiver,
            method,
            args: _,
        } => {
            let receiver_ty = infer_codegen_expr_type(receiver, function, info)?;
            let element_ty = list_element_type(&receiver_ty)?;
            match method.as_str() {
                "capacity" => Ok(Type::I32),
                "remove" => Ok(element_ty.clone()),
                "append" | "reserve" | "set" | "insert" | "clear" => Ok(Type::Void),
                _ => Err(CompileError::new(format!(
                    "unsupported list accessor `@{method}` during code generation"
                ))),
            }
        }
        Expr::Call { callee, .. } => {
            let Some(path) = callee.as_path() else {
                return Err(CompileError::new(
                    "only direct calls are supported during code generation",
                ));
            };
            match path {
                [name] => info
                    .functions
                    .get(name)
                    .map(|sig| sig.return_type.clone())
                    .ok_or_else(|| CompileError::new(format!("unknown function `{name}`"))),
                _ => Err(CompileError::new(format!(
                    "unsupported call target `{}` in code generation",
                    path.join(".")
                ))),
            }
        }
        Expr::Cast { ty, .. } => Ok(ty.clone()),
        Expr::Unary { op, expr } => match op {
            UnaryOp::Neg | UnaryOp::LogicalNot => {
                infer_codegen_integer_unary_type(*op, expr, function, info)
            }
        },
        Expr::Pack(_) => Err(CompileError::new(
            "packed `{...}` expressions are only valid inside @print",
        )),
        Expr::Binary { lhs, op, rhs } => {
            let lhs_ty =
                resolve_codegen_aliases(&infer_codegen_expr_type(lhs, function, info)?, info)?;
            let rhs_ty =
                resolve_codegen_aliases(&infer_codegen_expr_type(rhs, function, info)?, info)?;
            match op {
                BinaryOp::Add | BinaryOp::Subtract | BinaryOp::Divide | BinaryOp::Multiply
                    if common_codegen_numeric_type(&lhs_ty, &rhs_ty).is_some() =>
                {
                    Ok(common_codegen_numeric_type(&lhs_ty, &rhs_ty).unwrap())
                }
                BinaryOp::Modulo if common_codegen_integer_type(&lhs_ty, &rhs_ty).is_some() => {
                    Ok(common_codegen_integer_type(&lhs_ty, &rhs_ty).unwrap())
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
                    if is_codegen_condition_type(&lhs_ty) && is_codegen_condition_type(&rhs_ty) =>
                {
                    Ok(Type::Bool)
                }
                BinaryOp::LogicalAnd | BinaryOp::LogicalOr => Err(CompileError::new(
                    "logical operators currently require bool or integer operands",
                )),
                BinaryOp::BitAnd | BinaryOp::BitOr | BinaryOp::BitXor
                    if common_codegen_integer_type(&lhs_ty, &rhs_ty).is_some() =>
                {
                    Ok(common_codegen_integer_type(&lhs_ty, &rhs_ty).unwrap())
                }
                BinaryOp::BitAnd | BinaryOp::BitOr | BinaryOp::BitXor => Err(CompileError::new(
                    "bitwise operators currently require compatible integer operands",
                )),
                BinaryOp::ShiftLeft | BinaryOp::ShiftRight
                    if is_codegen_integer_type(&lhs_ty) && is_codegen_integer_type(&rhs_ty) =>
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
                    if common_codegen_numeric_type(&lhs_ty, &rhs_ty).is_some()
                        || (lhs_ty == Type::Bool && rhs_ty == Type::Bool) =>
                {
                    Ok(Type::Bool)
                }
                BinaryOp::LessThan
                | BinaryOp::LessEqual
                | BinaryOp::GreaterThan
                | BinaryOp::GreaterEqual
                | BinaryOp::Equal => Err(CompileError::new(
                    "comparison operators currently require compatible numeric or bool operands",
                )),
            }
        }
    }
}

fn render_logical_shift_right(
    lhs: &Expr,
    rhs: &Expr,
    function: &Function,
    info: &ProgramInfo,
) -> Result<String, CompileError> {
    let lhs_ty = resolve_codegen_aliases(&infer_codegen_expr_type(lhs, function, info)?, info)?;
    let rendered_lhs = render_expr(lhs, function, info)?;
    let rendered_rhs = render_expr(rhs, function, info)?;
    match lhs_ty {
        Type::I8 | Type::I16 | Type::I32 | Type::I64 | Type::Isize => {
            let signed = c_type(&lhs_ty);
            let unsigned = unsigned_c_type(&lhs_ty).ok_or_else(|| {
                CompileError::new(format!(
                    "logical shift-right requires an integer operand during code generation, got {}",
                    describe_type(&lhs_ty)
                ))
            })?;
            Ok(format!(
                "(({})(({})({rendered_lhs}) >> ({rendered_rhs})))",
                signed, unsigned
            ))
        }
        Type::U16 | Type::U32 | Type::U64 | Type::Usize => {
            Ok(format!("(({})({rendered_lhs}) >> ({rendered_rhs}))", c_type(&lhs_ty)))
        }
        other => Err(CompileError::new(format!(
            "logical shift-right requires an integer operand during code generation, got {}",
            describe_type(&other)
        ))),
    }
}

fn infer_codegen_integer_unary_type(
    op: UnaryOp,
    expr: &Expr,
    function: &Function,
    info: &ProgramInfo,
) -> Result<Type, CompileError> {
    let inner_ty = resolve_codegen_aliases(&infer_codegen_expr_type(expr, function, info)?, info)?;
    match op {
        UnaryOp::Neg if is_codegen_signed_numeric_type(&inner_ty) => Ok(inner_ty),
        UnaryOp::Neg => Err(CompileError::new(
            "unary `-` currently requires a signed numeric operand",
        )),
        UnaryOp::LogicalNot if is_codegen_condition_type(&inner_ty) => Ok(Type::Bool),
        UnaryOp::LogicalNot => Err(CompileError::new(
            "unary `!` currently requires a bool or integer operand",
        )),
    }
}

fn infer_builtin_type(
    name: &str,
    args: &[Expr],
    function: &Function,
    info: &ProgramInfo,
) -> Result<Type, CompileError> {
    match name {
        "append" | "capacity" | "reserve" | "set" | "insert" | "remove" | "clear" => {
            if args.is_empty() {
                return Err(CompileError::new(format!(
                    "@{name} expects a list receiver as its first argument",
                )));
            }
            let receiver_ty = infer_codegen_expr_type(&args[0], function, info)?;
            let element_ty = list_element_type(&receiver_ty)?;
            match name {
                "capacity" => Ok(Type::I32),
                "remove" => Ok(element_ty.clone()),
                _ => Ok(Type::Void),
            }
        }
        "puts" | "print" | "free" => Ok(Type::Void),
        "alloc" | "realloc" => Ok(Type::Mut(Box::new(Type::Ref(Box::new(Type::U8))))),
        "addr" => Ok(Type::Ref(Box::new(infer_codegen_expr_type(
            &args[0], function, info,
        )?))),
        "deref" => match infer_codegen_expr_type(&args[0], function, info)? {
            Type::Ref(inner) => Ok(*inner),
            Type::Mut(inner) => match *inner {
                Type::Ref(inner) => Ok(*inner),
                other => Err(CompileError::new(format!(
                    "@deref requires a ref(...) argument during code generation, got {}",
                    describe_type(&other)
                ))),
            },
            other => Err(CompileError::new(format!(
                "@deref requires a ref(...) argument during code generation, got {}",
                describe_type(&other)
            ))),
        },
        _ => Err(CompileError::new(format!(
            "unsupported builtin intrinsic `@{name}` during code generation"
        ))),
    }
}

fn flatten_print_args(args: &[Expr]) -> Vec<&Expr> {
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
    Bool,
    Pointer,
    String,
    Float,
    Double,
}

fn convert_format_string(input: &str, arg_types: &[Type]) -> Result<(String, Vec<PrintMarker>), CompileError> {
    let mut output = String::new();
    let mut markers = Vec::new();
    let chars: Vec<char> = input.chars().collect();
    let mut index = 0;
    let mut arg_index = 0;
    while index < chars.len() {
        if chars[index] == '{' {
            let start = index + 1;
            let mut end = start;
            while end < chars.len() && chars[end] != '}' {
                end += 1;
            }
            if end >= chars.len() {
                return Err(CompileError::new("unterminated @print format marker"));
            }
            let marker: String = chars[start..end].iter().collect();
            let parsed = match marker.as_str() {
                "d" => PrintMarker::Int,
                "b" => PrintMarker::Bool,
                "p" => PrintMarker::Pointer,
                "s" => PrintMarker::String,
                "f" => PrintMarker::Float,
                "lf" => PrintMarker::Double,
                _ => {
                    return Err(CompileError::new(format!(
                        "unsupported @print marker `{{{marker}}}`"
                    )));
                }
            };
            let arg_ty = arg_types.get(arg_index).ok_or_else(|| {
                CompileError::new("@print format expects more values than were provided")
            })?;
            let specifier = print_format_specifier(parsed, arg_ty)?;
            output.push_str(specifier);
            markers.push(parsed);
            arg_index += 1;
            index = end + 1;
            continue;
        }
        output.push(chars[index]);
        index += 1;
    }
    Ok((output, markers))
}

fn collect_list_types(program: &Program, info: &ProgramInfo) -> Vec<Type> {
    let mut set = HashSet::new();
    for type_def in &program.type_defs {
        if let Some(alias) = &type_def.alias {
            collect_list_types_from_type(alias, &mut set);
        }
        for field in &type_def.fields {
            collect_list_types_from_type(&field.ty, &mut set);
        }
    }
    for function in &program.functions {
        collect_list_types_from_type(&function.return_type, &mut set);
        for param in &function.params {
            collect_list_types_from_type(&param.ty, &mut set);
        }
    }
    for locals in info.locals.values() {
        for ty in locals.values() {
            collect_list_types_from_type(ty, &mut set);
        }
    }

    let mut list_types: Vec<_> = set.into_iter().collect();
    list_types.sort_by_key(describe_type);
    list_types
}

fn collect_list_types_from_type(ty: &Type, set: &mut HashSet<Type>) {
    match ty {
        Type::List(inner) => {
            collect_list_types_from_type(inner, set);
            set.insert(ty.clone());
        }
        Type::Mut(inner) => collect_list_types_from_type(inner, set),
        Type::Ref(inner) => collect_list_types_from_type(inner, set),
        _ => {}
    }
}

fn render_list_support(list_ty: &Type) -> Result<String, CompileError> {
    let element_ty = list_element_type(list_ty)?;
    let list_name = c_type(list_ty);
    let element_c_ty = c_type(element_ty);
    let helper_prefix = list_helper_prefix(element_ty);
    let mut output = String::new();

    output.push_str("typedef struct {\n");
    output.push_str("    ");
    output.push_str(&element_c_ty);
    output.push_str(" *data;\n");
    output.push_str("    int32_t len;\n");
    output.push_str("    int32_t cap;\n");
    output.push_str("} ");
    output.push_str(&list_name);
    output.push_str(";\n\n");

    output.push_str("static ");
    output.push_str(&list_name);
    output.push(' ');
    output.push_str(&helper_prefix);
    output.push_str("_new(void) {\n");
    output.push_str("    return (");
    output.push_str(&list_name);
    output.push_str("){ .data = NULL, .len = 0, .cap = 0 };\n");
    output.push_str("}\n\n");

    output.push_str("static void ");
    output.push_str(&helper_prefix);
    output.push_str("_ensure_capacity(");
    output.push_str(&list_name);
    output.push_str(" *list, int32_t requested) {\n");
    output.push_str("    if (requested < 0) {\n");
    output.push_str("        scar_runtime_panic(\"list capacity cannot be negative\");\n");
    output.push_str("    }\n");
    output.push_str("    if (requested <= list->cap) {\n");
    output.push_str("        return;\n");
    output.push_str("    }\n");
    output.push_str("    int32_t new_cap = list->cap > 0 ? list->cap : 4;\n");
    output.push_str("    while (new_cap < requested) {\n");
    output.push_str("        if (new_cap > INT32_MAX / 2) {\n");
    output.push_str("            new_cap = requested;\n");
    output.push_str("            break;\n");
    output.push_str("        }\n");
    output.push_str("        new_cap *= 2;\n");
    output.push_str("    }\n");
    output.push_str("    void *resized = realloc(list->data, sizeof(");
    output.push_str(&element_c_ty);
    output.push_str(") * (size_t)new_cap);\n");
    output.push_str("    if (resized == NULL) {\n");
    output.push_str("        scar_runtime_panic(\"failed to allocate list storage\");\n");
    output.push_str("    }\n");
    output.push_str("    list->data = (");
    output.push_str(&element_c_ty);
    output.push_str(" *)resized;\n");
    output.push_str("    list->cap = new_cap;\n");
    output.push_str("}\n\n");

    output.push_str("static ");
    output.push_str(&list_name);
    output.push(' ');
    output.push_str(&helper_prefix);
    output.push_str("_from_array(");
    output.push_str(&element_c_ty);
    output.push_str(" const *items, int32_t len) {\n");
    output.push_str("    ");
    output.push_str(&list_name);
    output.push_str(" list = ");
    output.push_str(&helper_prefix);
    output.push_str("_new();\n");
    output.push_str("    if (len == 0) {\n");
    output.push_str("        return list;\n");
    output.push_str("    }\n");
    output.push_str("    ");
    output.push_str(&helper_prefix);
    output.push_str("_ensure_capacity(&list, len);\n");
    output.push_str("    memcpy(list.data, items, sizeof(");
    output.push_str(&element_c_ty);
    output.push_str(") * (size_t)len);\n");
    output.push_str("    list.len = len;\n");
    output.push_str("    return list;\n");
    output.push_str("}\n\n");

    output.push_str("static ");
    output.push_str(&element_c_ty);
    output.push_str(" *");
    output.push_str(&helper_prefix);
    output.push_str("_at(");
    output.push_str(&list_name);
    output.push_str(" *list, int32_t index) {\n");
    output.push_str("    if (index < 0 || index >= list->len) {\n");
    output.push_str("        scar_runtime_panic(\"list index out of bounds\");\n");
    output.push_str("    }\n");
    output.push_str("    return &list->data[index];\n");
    output.push_str("}\n\n");

    output.push_str("static int32_t ");
    output.push_str(&helper_prefix);
    output.push_str("_capacity(");
    output.push_str(&list_name);
    output.push_str(" *list) {\n");
    output.push_str("    return list->cap;\n");
    output.push_str("}\n\n");

    output.push_str("static void ");
    output.push_str(&helper_prefix);
    output.push_str("_reserve(");
    output.push_str(&list_name);
    output.push_str(" *list, int32_t capacity) {\n");
    output.push_str("    ");
    output.push_str(&helper_prefix);
    output.push_str("_ensure_capacity(list, capacity);\n");
    output.push_str("}\n\n");

    output.push_str("static void ");
    output.push_str(&helper_prefix);
    output.push_str("_append(");
    output.push_str(&list_name);
    output.push_str(" *list, ");
    output.push_str(&element_c_ty);
    output.push_str(" value) {\n");
    output.push_str("    ");
    output.push_str(&helper_prefix);
    output.push_str("_ensure_capacity(list, list->len + 1);\n");
    output.push_str("    list->data[list->len++] = value;\n");
    output.push_str("}\n\n");

    output.push_str("static void ");
    output.push_str(&helper_prefix);
    output.push_str("_set(");
    output.push_str(&list_name);
    output.push_str(" *list, int32_t index, ");
    output.push_str(&element_c_ty);
    output.push_str(" value) {\n");
    output.push_str("    *");
    output.push_str(&helper_prefix);
    output.push_str("_at(list, index) = value;\n");
    output.push_str("}\n\n");

    output.push_str("static void ");
    output.push_str(&helper_prefix);
    output.push_str("_insert(");
    output.push_str(&list_name);
    output.push_str(" *list, int32_t index, ");
    output.push_str(&element_c_ty);
    output.push_str(" value) {\n");
    output.push_str("    if (index < 0 || index > list->len) {\n");
    output.push_str("        scar_runtime_panic(\"list insert index out of bounds\");\n");
    output.push_str("    }\n");
    output.push_str("    ");
    output.push_str(&helper_prefix);
    output.push_str("_ensure_capacity(list, list->len + 1);\n");
    output.push_str("    if (index < list->len) {\n");
    output.push_str("        memmove(&list->data[index + 1], &list->data[index], sizeof(");
    output.push_str(&element_c_ty);
    output.push_str(") * (size_t)(list->len - index));\n");
    output.push_str("    }\n");
    output.push_str("    list->data[index] = value;\n");
    output.push_str("    list->len += 1;\n");
    output.push_str("}\n\n");

    output.push_str("static ");
    output.push_str(&element_c_ty);
    output.push(' ');
    output.push_str(&helper_prefix);
    output.push_str("_remove(");
    output.push_str(&list_name);
    output.push_str(" *list, int32_t index) {\n");
    output.push_str("    ");
    output.push_str(&element_c_ty);
    output.push_str(" removed = *");
    output.push_str(&helper_prefix);
    output.push_str("_at(list, index);\n");
    output.push_str("    if (index + 1 < list->len) {\n");
    output.push_str("        memmove(&list->data[index], &list->data[index + 1], sizeof(");
    output.push_str(&element_c_ty);
    output.push_str(") * (size_t)(list->len - index - 1));\n");
    output.push_str("    }\n");
    output.push_str("    list->len -= 1;\n");
    output.push_str("    return removed;\n");
    output.push_str("}\n\n");

    output.push_str("static void ");
    output.push_str(&helper_prefix);
    output.push_str("_clear(");
    output.push_str(&list_name);
    output.push_str(" *list) {\n");
    output.push_str("    list->len = 0;\n");
    output.push_str("}\n");

    Ok(output)
}

fn list_helper_prefix(element_ty: &Type) -> String {
    format!("scar_list__{}", type_suffix(element_ty))
}

fn type_suffix(ty: &Type) -> String {
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
        Type::Named(name) => sanitize_identifier(name),
        Type::Mut(inner) => format!("mut__{}", type_suffix(inner)),
        Type::Ref(inner) => format!("ref__{}", type_suffix(inner)),
        Type::List(inner) => format!("list__{}", type_suffix(inner)),
    }
}

fn sanitize_identifier(name: &str) -> String {
    name.chars()
        .map(|ch| if ch.is_ascii_alphanumeric() { ch } else { '_' })
        .collect()
}

fn c_type(ty: &Type) -> String {
    match ty {
        Type::Void => "void".to_string(),
        Type::Bool => "int32_t".to_string(),
        Type::I8 => "int8_t".to_string(),
        Type::I16 => "int16_t".to_string(),
        Type::I32 => "int32_t".to_string(),
        Type::I64 => "int64_t".to_string(),
        Type::Isize => "intptr_t".to_string(),
        Type::U16 => "uint16_t".to_string(),
        Type::U32 => "uint32_t".to_string(),
        Type::U64 => "uint64_t".to_string(),
        Type::Usize => "uintptr_t".to_string(),
        Type::U8 => "const char *".to_string(),
        Type::F32 => "float".to_string(),
        Type::F64 => "double".to_string(),
        Type::Named(name) => name.clone(),
        Type::Mut(inner) => c_type_mut(inner),
        Type::Ref(inner) => {
            if inner.as_ref() == &Type::U8 {
                return "const char *".to_string();
            }
            let inner = c_type(inner);
            if inner.ends_with('*') {
                format!("{inner}*")
            } else {
                format!("{inner} *")
            }
        }
        Type::List(inner) => format!("scar_list__{}", type_suffix(inner)),
    }
}

fn list_element_type(ty: &Type) -> Result<&Type, CompileError> {
    match deref_refs(ty) {
        Type::List(inner) => Ok(inner),
        other => Err(CompileError::new(format!(
            "expected list type, got {}",
            describe_type(other)
        ))),
    }
}

fn infer_index_type(ty: &Type) -> Result<Type, CompileError> {
    Ok(list_element_type(ty)?.clone())
}

fn is_codegen_integer_type(ty: &Type) -> bool {
    codegen_integer_rank(ty).is_some()
}

fn is_codegen_condition_type(ty: &Type) -> bool {
    matches!(ty, Type::Bool) || is_codegen_integer_type(ty)
}

fn is_codegen_string_compatible(ty: &Type) -> bool {
    matches!(ty, Type::U8)
        || matches!(ty, Type::Ref(inner) if inner.as_ref() == &Type::U8)
        || matches!(ty, Type::Mut(inner) if matches!(inner.as_ref(), Type::Ref(inner) if inner.as_ref() == &Type::U8))
}

fn is_codegen_signed_numeric_type(ty: &Type) -> bool {
    is_codegen_signed_integer_type(ty) || matches!(ty, Type::F32 | Type::F64)
}

fn is_codegen_signed_integer_type(ty: &Type) -> bool {
    matches!(ty, Type::I8 | Type::I16 | Type::I32 | Type::I64 | Type::Isize)
}

fn is_codegen_unsigned_integer_type(ty: &Type) -> bool {
    matches!(ty, Type::U16 | Type::U32 | Type::U64 | Type::Usize)
}

fn codegen_integer_rank(ty: &Type) -> Option<u8> {
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

fn codegen_float_rank(ty: &Type) -> Option<u8> {
    match ty {
        Type::F32 => Some(1),
        Type::F64 => Some(2),
        _ => None,
    }
}

fn common_codegen_integer_type(lhs: &Type, rhs: &Type) -> Option<Type> {
    if lhs == rhs && is_codegen_integer_type(lhs) {
        return Some(lhs.clone());
    }
    if is_codegen_signed_integer_type(lhs) && is_codegen_signed_integer_type(rhs) {
        return Some(if codegen_integer_rank(lhs)? >= codegen_integer_rank(rhs)? {
            lhs.clone()
        } else {
            rhs.clone()
        });
    }
    if is_codegen_unsigned_integer_type(lhs) && is_codegen_unsigned_integer_type(rhs) {
        return Some(if codegen_integer_rank(lhs)? >= codegen_integer_rank(rhs)? {
            lhs.clone()
        } else {
            rhs.clone()
        });
    }
    None
}

fn common_codegen_numeric_type(lhs: &Type, rhs: &Type) -> Option<Type> {
    if let Some(common) = common_codegen_integer_type(lhs, rhs) {
        return Some(common);
    }
    if lhs == rhs && matches!(lhs, Type::F32 | Type::F64) {
        return Some(lhs.clone());
    }
    if matches!(lhs, Type::F32 | Type::F64) && matches!(rhs, Type::F32 | Type::F64) {
        return Some(if codegen_float_rank(lhs)? >= codegen_float_rank(rhs)? {
            lhs.clone()
        } else {
            rhs.clone()
        });
    }
    None
}

fn unsigned_c_type(ty: &Type) -> Option<&'static str> {
    match ty {
        Type::I8 => Some("uint8_t"),
        Type::I16 => Some("uint16_t"),
        Type::I32 => Some("uint32_t"),
        Type::I64 => Some("uint64_t"),
        Type::Isize => Some("uintptr_t"),
        _ => None,
    }
}

fn print_format_specifier(marker: PrintMarker, ty: &Type) -> Result<&'static str, CompileError> {
    match marker {
        PrintMarker::Int if is_codegen_signed_integer_type(ty) => Ok("%jd"),
        PrintMarker::Int if is_codegen_unsigned_integer_type(ty) => Ok("%ju"),
        PrintMarker::Bool if matches!(ty, Type::Bool) => Ok("%s"),
        PrintMarker::Pointer if matches!(ty, Type::Ref(_) | Type::Mut(_) | Type::Named(_))
            || is_codegen_string_compatible(ty) =>
        {
            Ok("%p")
        }
        PrintMarker::String if is_codegen_string_compatible(ty) => {
            Ok("%s")
        }
        PrintMarker::Float if matches!(ty, Type::F32) => Ok("%f"),
        PrintMarker::Double if matches!(ty, Type::F64) => Ok("%lf"),
        PrintMarker::Int => Err(CompileError::new(format!(
            "format marker `{{d}}` does not accept value of type {}",
            describe_type(ty)
        ))),
        PrintMarker::Bool => Err(CompileError::new(format!(
            "format marker `{{b}}` does not accept value of type {}",
            describe_type(ty)
        ))),
        PrintMarker::Float => Err(CompileError::new(format!(
            "format marker `{{f}}` does not accept value of type {}",
            describe_type(ty)
        ))),
        PrintMarker::Double => Err(CompileError::new(format!(
            "format marker `{{lf}}` does not accept value of type {}",
            describe_type(ty)
        ))),
        PrintMarker::Pointer => Err(CompileError::new(format!(
            "format marker `{{p}}` does not accept value of type {}",
            describe_type(ty)
        ))),
        PrintMarker::String => Err(CompileError::new(format!(
            "format marker `{{s}}` does not accept value of type {}",
            describe_type(ty)
        ))),
    }
}

fn render_print_value(marker: PrintMarker, ty: &Type, rendered: &str) -> Result<String, CompileError> {
    match marker {
        PrintMarker::Int if is_codegen_signed_integer_type(ty) => {
            Ok(format!("((intmax_t)({rendered}))"))
        }
        PrintMarker::Int if is_codegen_unsigned_integer_type(ty) => {
            Ok(format!("((uintmax_t)({rendered}))"))
        }
        PrintMarker::Bool if matches!(ty, Type::Bool) => {
            Ok(format!("(({}) ? \"true\" : \"false\")", rendered))
        }
        PrintMarker::Pointer => Ok(format!("(void *)({rendered})")),
        PrintMarker::String | PrintMarker::Float | PrintMarker::Double => Ok(rendered.to_string()),
        _ => Err(CompileError::new(format!(
            "unsupported @print argument type {}",
            describe_type(ty)
        ))),
    }
}

fn resolve_codegen_aliases(ty: &Type, info: &ProgramInfo) -> Result<Type, CompileError> {
    match ty {
        Type::Named(name) => {
            let Some(type_info) = info.types.get(name) else {
                return Ok(Type::Named(name.clone()));
            };
            if let Some(alias) = &type_info.alias {
                resolve_codegen_aliases(alias, info)
            } else {
                Ok(Type::Named(name.clone()))
            }
        }
        Type::Mut(inner) => Ok(Type::Mut(Box::new(resolve_codegen_aliases(inner, info)?))),
        Type::Ref(inner) => Ok(Type::Ref(Box::new(resolve_codegen_aliases(inner, info)?))),
        Type::List(inner) => Ok(Type::List(Box::new(resolve_codegen_aliases(inner, info)?))),
        other => Ok(other.clone()),
    }
}

fn deref_refs(mut ty: &Type) -> &Type {
    loop {
        match ty {
            Type::Ref(inner) | Type::Mut(inner) => ty = inner,
            _ => return ty,
        }
    }
}

fn escape_c_string(value: &str) -> String {
    let mut escaped = String::new();
    for ch in value.chars() {
        match ch {
            '\\' => escaped.push_str("\\\\"),
            '"' => escaped.push_str("\\\""),
            '\n' => escaped.push_str("\\n"),
            '\r' => escaped.push_str("\\r"),
            '\t' => escaped.push_str("\\t"),
            other => escaped.push(other),
        }
    }
    escaped
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
        Type::Mut(inner) => format!("mut({})", describe_type(inner)),
        Type::Ref(inner) => format!("ref({})", describe_type(inner)),
        Type::List(inner) => format!("list[{}]", describe_type(inner)),
    }
}

fn c_type_mut(ty: &Type) -> String {
    match ty {
        Type::U8 => "char".to_string(),
        Type::Ref(inner) if inner.as_ref() == &Type::U8 => "char *".to_string(),
        Type::Ref(inner) => {
            let inner = c_type_mut(inner);
            if inner.ends_with('*') {
                format!("{inner}*")
            } else {
                format!("{inner} *")
            }
        }
        Type::Mut(inner) => c_type_mut(inner),
        other => c_type(other),
    }
}

fn is_reference_like(ty: &Type) -> bool {
    matches!(ty, Type::Ref(_))
        || matches!(ty, Type::Mut(inner) if matches!(inner.as_ref(), Type::Ref(_)))
}

fn indent(output: &mut String, level: usize) {
    for _ in 0..level {
        output.push_str("    ");
    }
}
