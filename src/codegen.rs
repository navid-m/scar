use crate::{
    CompileError,
    ast::{BinaryOp, Expr, Function, Program, Stmt, Type, TypeDef},
    sema::ProgramInfo,
};

pub fn generate_c(program: &Program, info: &ProgramInfo) -> Result<String, CompileError> {
    let mut output = String::new();
    output.push_str("#include <stdint.h>\n");
    output.push_str("#include <stdio.h>\n\n");

    for type_def in &program.type_defs {
        output.push_str(&render_type_def(type_def));
        output.push('\n');
    }

    for function in &program.functions {
        output.push_str(&render_signature(function));
        output.push_str(";\n");
    }
    output.push('\n');

    for function in &program.functions {
        render_function(&mut output, function, info)?;
        output.push('\n');
    }

    Ok(output)
}

fn render_function(
    output: &mut String,
    function: &Function,
    info: &ProgramInfo,
) -> Result<(), CompileError> {
    output.push_str(&render_signature(function));
    output.push_str(" {\n");
    for stmt in &function.body {
        render_stmt(output, stmt, function, info, 1)?;
    }
    if function.name == "main" && function.return_type == Type::Void {
        indent(output, 1);
        output.push_str("return 0;\n");
    }
    output.push_str("}\n");
    Ok(())
}

fn render_type_def(type_def: &TypeDef) -> String {
    let mut output = String::new();
    output.push_str("typedef struct {\n");
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

fn render_signature(function: &Function) -> String {
    let return_type = if function.name == "main" {
        "int".to_string()
    } else {
        c_type(&function.return_type)
    };

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

    format!("{return_type} {}({params})", function.name)
}

fn render_stmt(
    output: &mut String,
    stmt: &Stmt,
    function: &Function,
    info: &ProgramInfo,
    level: usize,
) -> Result<(), CompileError> {
    match stmt {
        Stmt::VarDecl {
            mutable,
            name,
            init,
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
            output.push_str(&render_expr(init)?);
            output.push_str(";\n");
        }
        Stmt::Assign { target, value } => {
            indent(output, level);
            output.push_str(&render_expr(target)?);
            output.push_str(" = ");
            output.push_str(&render_expr(value)?);
            output.push_str(";\n");
        }
        Stmt::AddAssign { target, value } => {
            indent(output, level);
            output.push_str(&render_expr(target)?);
            output.push_str(" += ");
            output.push_str(&render_expr(value)?);
            output.push_str(";\n");
        }
        Stmt::Return(None) => {
            indent(output, level);
            output.push_str("return;\n");
        }
        Stmt::Return(Some(value)) => {
            indent(output, level);
            output.push_str("return ");
            output.push_str(&render_expr(value)?);
            output.push_str(";\n");
        }
        Stmt::Expr(expr) => {
            indent(output, level);
            output.push_str(&render_expr(expr)?);
            output.push_str(";\n");
        }
        Stmt::For {
            pragma,
            var_name,
            start,
            end,
            body,
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
            output.push_str(&render_expr(start)?);
            output.push_str("; ");
            output.push_str(var_name);
            output.push_str(" <= ");
            output.push_str(&render_expr(end)?);
            output.push_str("; ++");
            output.push_str(var_name);
            output.push_str(") {\n");
            for stmt in body {
                render_stmt(output, stmt, function, info, level + 1)?;
            }
            indent(output, level);
            output.push_str("}\n");
        }
    }
    Ok(())
}

fn render_expr(expr: &Expr) -> Result<String, CompileError> {
    match expr {
        Expr::Int(value) => Ok(value.to_string()),
        Expr::String(value) => Ok(format!("\"{}\"", escape_c_string(value))),
        Expr::FieldAccess { base, field } => Ok(format!("({}).{}", render_expr(base)?, field)),
        Expr::StructInit { name, fields } => {
            let rendered_fields = fields
                .iter()
                .map(|field| Ok(format!(".{} = {}", field.name, render_expr(&field.value)?)))
                .collect::<Result<Vec<_>, CompileError>>()?;
            Ok(format!("({name}){{ {} }}", rendered_fields.join(", ")))
        }
        Expr::BuiltinCall { name, args } => render_builtin_call(name, args),
        Expr::Path(path) => match path.as_slice() {
            [name] => Ok(name.clone()),
            _ => Err(CompileError::new(format!(
                "unsupported qualified expression `{}` in code generation",
                path.join(".")
            ))),
        },
        Expr::Pack(_) => Err(CompileError::new(
            "packed `{...}` expressions are only valid inside @print",
        )),
        Expr::Binary { lhs, op, rhs } => match op {
            BinaryOp::Add => Ok(format!("({} + {})", render_expr(lhs)?, render_expr(rhs)?)),
        },
        Expr::Call { callee, args } => render_call(callee, args),
    }
}

fn render_call(callee: &Expr, args: &[Expr]) -> Result<String, CompileError> {
    if let Some(path) = callee.as_path() {
        if path.len() == 2 && path[0] == "builtin" {
            return render_builtin_call(&path[1], args);
        }
        if path.len() == 1 {
            let rendered_args = args
                .iter()
                .map(render_expr)
                .collect::<Result<Vec<_>, _>>()?;
            return Ok(format!("{}({})", path[0], rendered_args.join(", ")));
        }
    }

    Err(CompileError::new(
        "only direct function calls and @builtin calls are currently supported in codegen",
    ))
}

fn render_builtin_call(name: &str, args: &[Expr]) -> Result<String, CompileError> {
    match name {
        "puts" => Ok(format!("puts({})", render_expr(&args[0])?)),
        "print" => {
            let Expr::String(format) = &args[0] else {
                return Err(CompileError::new(
                    "@print requires a string literal as its first argument",
                ));
            };
            let (converted, markers) = convert_format_string(format)?;
            let flattened = flatten_print_args(&args[1..]);
            let mut rendered_args = vec![format!("\"{}\"", escape_c_string(&converted))];
            for (marker, expr) in markers.into_iter().zip(flattened.into_iter()) {
                let rendered = render_expr(expr)?;
                if marker == 'p' {
                    rendered_args.push(format!("(void *)({rendered})"));
                } else {
                    rendered_args.push(rendered);
                }
            }
            Ok(format!("printf({})", rendered_args.join(", ")))
        }
        "addr" => Ok(format!("(&{})", render_expr(&args[0])?)),
        "deref" => Ok(format!("(*({}))", render_expr(&args[0])?)),
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

fn convert_format_string(input: &str) -> Result<(String, Vec<char>), CompileError> {
    let mut output = String::new();
    let mut markers = Vec::new();
    let chars: Vec<char> = input.chars().collect();
    let mut index = 0;
    while index < chars.len() {
        if chars[index] == '{' {
            if index + 2 >= chars.len() || chars[index + 2] != '}' {
                return Err(CompileError::new(
                    "unterminated @print format marker",
                ));
            }
            let marker = chars[index + 1];
            let specifier = match marker {
                'd' => "%d",
                'p' => "%p",
                's' => "%s",
                _ => {
                    return Err(CompileError::new(format!(
                        "unsupported @print marker `{{{marker}}}`"
                    )));
                }
            };
            output.push_str(specifier);
            markers.push(marker);
            index += 3;
            continue;
        }
        output.push(chars[index]);
        index += 1;
    }
    Ok((output, markers))
}

fn c_type(ty: &Type) -> String {
    match ty {
        Type::Void => "void".to_string(),
        Type::I32 => "int32_t".to_string(),
        Type::U8 => "const char *".to_string(),
        Type::Named(name) => name.clone(),
        Type::Ref(inner) => {
            let inner = c_type(inner);
            if inner.ends_with('*') {
                format!("{inner}*")
            } else {
                format!("{inner} *")
            }
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

fn indent(output: &mut String, level: usize) {
    for _ in 0..level {
        output.push_str("    ");
    }
}
