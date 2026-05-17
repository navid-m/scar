use std::{
    collections::{HashMap, HashSet},
    env,
    fs,
    path::{Path, PathBuf},
};

use crate::{
    CompileError,
    ast::{
        Expr, FieldDef, FieldInit, Function, GenericParam, InterfaceDef, InterfaceMethod, ModuleUse,
        Param, Program, Stmt, TestBlock, Type, TypeDef,
    },
    lexer::lex,
    parser::parse_program,
};

pub fn resolve_entry_program(entry: &Path) -> Result<Program, CompileError> {
    let entry = canonicalize_path(entry)?;
    let root_dir = entry
        .parent()
        .ok_or_else(|| CompileError::new("input file must have a parent directory"))?
        .to_path_buf();
    let mut resolver = Resolver {
        root_dir,
        local_std_root: discover_local_std_root()?,
        home_std_root: discover_home_std_root()?,
        cache: HashMap::new(),
        emitted_modules: HashSet::new(),
        resolved_interfaces: Vec::new(),
        resolved_types: Vec::new(),
        resolved_functions: Vec::new(),
        visiting: Vec::new(),
    };

    let program = parse_program_file(&entry)?;
    let module_aliases = resolver.resolve_module_uses(&program.module_uses, &entry)?;
    let local_types = build_named_type_map(&program.type_defs, &program.interface_defs, None);
    let local_functions = build_function_map(&program.functions, None);
    let mut interface_defs = resolver.resolved_interfaces;
    for interface_def in program.interface_defs {
        interface_defs.push(rewrite_interface_def(
            interface_def,
            &local_types,
            &module_aliases,
        )?);
    }
    let mut type_defs = resolver.resolved_types;
    for type_def in program.type_defs {
        type_defs.push(rewrite_type_def(type_def, &local_types, &module_aliases)?);
    }
    let mut functions = resolver.resolved_functions;
    for function in program.functions {
        functions.push(rewrite_function(
            function,
            &local_functions,
            &local_types,
            &module_aliases,
        )?);
    }
    let tests = program
        .tests
        .into_iter()
        .map(|test| rewrite_test_block(test, &local_functions, &local_types, &module_aliases))
        .collect::<Result<Vec<_>, _>>()?;

    instantiate_generic_functions(Program {
        module_uses: Vec::new(),
        interface_defs,
        type_defs,
        functions,
        tests,
    })
}

#[derive(Clone)]
struct ModuleExports {
    functions: HashMap<String, String>,
    named_types: HashMap<String, String>,
    interfaces: HashMap<String, String>,
}

type ModuleAliases = HashMap<String, ModuleExports>;

struct Resolver {
    root_dir: PathBuf,
    local_std_root: Option<PathBuf>,
    home_std_root: Option<PathBuf>,
    cache: HashMap<PathBuf, ModuleExports>,
    emitted_modules: HashSet<PathBuf>,
    resolved_interfaces: Vec<InterfaceDef>,
    resolved_types: Vec<TypeDef>,
    resolved_functions: Vec<Function>,
    visiting: Vec<PathBuf>,
}

impl Resolver {
    fn resolve_module_uses(
        &mut self,
        module_uses: &[ModuleUse],
        current_file: &Path,
    ) -> Result<ModuleAliases, CompileError> {
        let current_dir = current_file.parent().ok_or_else(|| {
            CompileError::new(format!(
                "source file {} must have a parent directory",
                current_file.display()
            ))
        })?;

        let mut aliases = HashMap::new();
        for module_use in module_uses {
            let module_path = resolve_module_use_path(
                current_dir,
                &module_use.path,
                self.local_std_root.as_deref(),
                self.home_std_root.as_deref(),
            );
            let exports = self.resolve_module(&module_path)?;
            aliases.insert(module_use.name.clone(), exports);
        }
        Ok(aliases)
    }

    fn resolve_module(&mut self, module_path: &Path) -> Result<ModuleExports, CompileError> {
        let module_path = canonicalize_path(module_path)?;
        if let Some(exports) = self.cache.get(&module_path) {
            return Ok(exports.clone());
        }
        if let Some(index) = self.visiting.iter().position(|path| path == &module_path) {
            let cycle = self.visiting[index..]
                .iter()
                .chain(std::iter::once(&module_path))
                .map(|path| path.display().to_string())
                .collect::<Vec<_>>()
                .join(" -> ");
            return Err(CompileError::new(format!(
                "module import cycle detected: {cycle}"
            )));
        }

        self.visiting.push(module_path.clone());
        let parsed = parse_program_file(&module_path)?;
        let module_aliases = self.resolve_module_uses(&parsed.module_uses, &module_path)?;
        let prefix = module_prefix(&module_path, &self.root_dir);
        let local_types = build_named_type_map(&parsed.type_defs, &parsed.interface_defs, Some(&prefix));
        let local_functions = build_function_map(&parsed.functions, Some(&prefix));
        let public_functions = build_public_function_map(&parsed.functions, Some(&prefix));
        let public_types = build_public_named_type_map(&parsed.type_defs, Some(&prefix));
        let public_interfaces = build_public_interface_map(&parsed.interface_defs, Some(&prefix));

        let mut rewritten_interfaces = Vec::new();
        for interface_def in parsed.interface_defs {
            rewritten_interfaces.push(rewrite_interface_def(
                interface_def,
                &local_types,
                &module_aliases,
            )?);
        }
        let mut rewritten_types = Vec::new();
        for type_def in parsed.type_defs {
            rewritten_types.push(rewrite_type_def(type_def, &local_types, &module_aliases)?);
        }

        let mut rewritten_functions = Vec::new();
        for function in parsed.functions {
            rewritten_functions.push(rewrite_function(
                function,
                &local_functions,
                &local_types,
                &module_aliases,
            )?);
        }

        if self.emitted_modules.insert(module_path.clone()) {
            self.resolved_interfaces.extend(rewritten_interfaces);
            self.resolved_types.extend(rewritten_types);
            self.resolved_functions.extend(rewritten_functions);
        }

        self.visiting.pop();
        let exports = ModuleExports {
            functions: public_functions,
            named_types: public_types,
            interfaces: public_interfaces,
        };
        self.cache.insert(module_path, exports.clone());
        Ok(exports)
    }
}

fn parse_program_file(path: &Path) -> Result<Program, CompileError> {
    let source = fs::read_to_string(path).map_err(|error| {
        CompileError::new(format!("failed to read {}: {error}", path.display()))
    })?;
    let tokens = lex(&source)
        .map_err(|error| CompileError::new(format!("in {}: {error}", path.display())))?;
    parse_program(tokens)
        .map_err(|error| CompileError::new(format!("in {}: {error}", path.display())))
}

fn discover_local_std_root() -> Result<Option<PathBuf>, CompileError> {
    let candidate = env::current_dir()
        .map_err(|error| CompileError::new(format!("failed to get current directory: {error}")))?
        .join("lib")
        .join("std");
    if candidate.is_dir() {
        Ok(Some(canonicalize_path(&candidate)?))
    } else {
        Ok(None)
    }
}

fn discover_home_std_root() -> Result<Option<PathBuf>, CompileError> {
    let Some(home) = env::var_os("HOME") else {
        return Ok(None);
    };
    let candidate = PathBuf::from(home).join(".scar").join("lib").join("std");
    if candidate.is_dir() {
        Ok(Some(canonicalize_path(&candidate)?))
    } else {
        Ok(None)
    }
}

fn resolve_module_use_path(
    current_dir: &Path,
    module_path: &str,
    local_std_root: Option<&Path>,
    home_std_root: Option<&Path>,
) -> PathBuf {
    if let Some(rest) = module_path.strip_prefix("std/") {
        if let Some(local_std_root) = local_std_root {
            return local_std_root.join(rest).with_extension("scar");
        }
        if let Some(home_std_root) = home_std_root {
            return home_std_root.join(rest).with_extension("scar");
        }
    }
    current_dir.join(module_path).with_extension("scar")
}

fn canonicalize_path(path: &Path) -> Result<PathBuf, CompileError> {
    fs::canonicalize(path).map_err(|error| {
        CompileError::new(format!("failed to resolve {}: {error}", path.display()))
    })
}

fn build_function_map(functions: &[Function], prefix: Option<&str>) -> HashMap<String, String> {
    functions
        .iter()
        .map(|function| {
            let mapped = match prefix {
                Some(prefix) => format!("{prefix}__{}", function.name),
                None => function.name.clone(),
            };
            (function.name.clone(), mapped)
        })
        .collect()
}

fn build_public_function_map(functions: &[Function], prefix: Option<&str>) -> HashMap<String, String> {
    functions
        .iter()
        .filter(|function| function.is_pub)
        .map(|function| {
            let mapped = match prefix {
                Some(prefix) => format!("{prefix}__{}", function.name),
                None => function.name.clone(),
            };
            (function.name.clone(), mapped)
        })
        .collect()
}

fn build_named_type_map(
    type_defs: &[TypeDef],
    interface_defs: &[InterfaceDef],
    prefix: Option<&str>,
) -> HashMap<String, String> {
    let mut named = type_defs
        .iter()
        .map(|type_def| {
            let mapped = match prefix {
                Some(prefix) => format!("{prefix}__{}", type_def.name),
                None => type_def.name.clone(),
            };
            (type_def.name.clone(), mapped)
        })
        .collect::<HashMap<_, _>>();
    named.extend(interface_defs.iter().map(|interface_def| {
        let mapped = match prefix {
            Some(prefix) => format!("{prefix}__{}", interface_def.name),
            None => interface_def.name.clone(),
        };
        (interface_def.name.clone(), mapped)
    }));
    named
}

fn build_public_named_type_map(type_defs: &[TypeDef], prefix: Option<&str>) -> HashMap<String, String> {
    type_defs
        .iter()
        .filter(|type_def| type_def.is_pub)
        .map(|type_def| {
            let mapped = match prefix {
                Some(prefix) => format!("{prefix}__{}", type_def.name),
                None => type_def.name.clone(),
            };
            (type_def.name.clone(), mapped)
        })
        .collect()
}

fn build_public_interface_map(
    interface_defs: &[InterfaceDef],
    prefix: Option<&str>,
) -> HashMap<String, String> {
    interface_defs
        .iter()
        .filter(|interface_def| interface_def.is_pub)
        .map(|interface_def| {
            let mapped = match prefix {
                Some(prefix) => format!("{prefix}__{}", interface_def.name),
                None => interface_def.name.clone(),
            };
            (interface_def.name.clone(), mapped)
        })
        .collect()
}

fn module_prefix(module_path: &Path, root_dir: &Path) -> String {
    let relative = module_path
        .strip_prefix(root_dir)
        .unwrap_or(module_path)
        .with_extension("");
    let mut segments = Vec::new();
    for component in relative.components() {
        let piece = component.as_os_str().to_string_lossy();
        let sanitized: String = piece
            .chars()
            .map(|ch| {
                if ch.is_ascii_alphanumeric() || ch == '_' {
                    ch
                } else {
                    '_'
                }
            })
            .collect();
        if !sanitized.is_empty() {
            segments.push(sanitized);
        }
    }
    if segments.is_empty() {
        "module".to_string()
    } else {
        segments.join("__")
    }
}

fn rewrite_function(
    mut function: Function,
    local_functions: &HashMap<String, String>,
    local_types: &HashMap<String, String>,
    module_aliases: &ModuleAliases,
) -> Result<Function, CompileError> {
    if let Some(mapped) = local_functions.get(&function.name) {
        function.name = mapped.clone();
    }
    function.generic_params = function
        .generic_params
        .into_iter()
        .map(|GenericParam { name, constraints }| GenericParam {
            name,
            constraints: constraints
                .into_iter()
                .map(|constraint| rewrite_type(constraint, local_types, module_aliases))
                .collect(),
        })
        .collect();
    function.params = function
        .params
        .into_iter()
        .map(|mut param| {
            param.ty = rewrite_type(param.ty, local_types, module_aliases);
            param
        })
        .collect();
    function.return_type = rewrite_type(function.return_type, local_types, module_aliases);
    function.body = function
        .body
        .into_iter()
        .map(|stmt| rewrite_stmt(stmt, local_functions, local_types, module_aliases))
        .collect::<Result<Vec<_>, _>>()?;
    Ok(function)
}

fn rewrite_interface_def(
    interface_def: InterfaceDef,
    local_types: &HashMap<String, String>,
    module_aliases: &ModuleAliases,
) -> Result<InterfaceDef, CompileError> {
    let name = local_types
        .get(&interface_def.name)
        .cloned()
        .unwrap_or(interface_def.name);
    let methods = interface_def
        .methods
        .into_iter()
        .map(|method| {
            Ok(InterfaceMethod {
                is_pub: method.is_pub,
                name: method.name,
                params: method
                    .params
                    .into_iter()
                    .map(|param| Param {
                        name: param.name,
                        ty: rewrite_type(param.ty, local_types, module_aliases),
                    })
                    .collect(),
                return_type: rewrite_type(method.return_type, local_types, module_aliases),
            })
        })
        .collect::<Result<Vec<_>, CompileError>>()?;
    Ok(InterfaceDef {
        is_pub: interface_def.is_pub,
        name,
        methods,
    })
}

fn rewrite_type_def(
    type_def: TypeDef,
    local_types: &HashMap<String, String>,
    module_aliases: &ModuleAliases,
) -> Result<TypeDef, CompileError> {
    let name = local_types
        .get(&type_def.name)
        .cloned()
        .unwrap_or(type_def.name);
    let is_extern = type_def.is_extern;
    let is_pub = type_def.is_pub;
    let alias = type_def
        .alias
        .map(|ty| rewrite_type(ty, local_types, module_aliases));
    let derives = type_def
        .derives
        .into_iter()
        .map(|ty| rewrite_type(ty, local_types, module_aliases))
        .collect();
    let fields = type_def
        .fields
        .into_iter()
        .map(|field| FieldDef {
            name: field.name,
            ty: rewrite_type(field.ty, local_types, module_aliases),
        })
        .collect();
    Ok(TypeDef {
        is_pub,
        name,
        is_extern,
        alias,
        derives,
        fields,
    })
}

fn rewrite_test_block(
    test: TestBlock,
    local_functions: &HashMap<String, String>,
    local_types: &HashMap<String, String>,
    module_aliases: &ModuleAliases,
) -> Result<TestBlock, CompileError> {
    Ok(TestBlock {
        name: test.name,
        body: test
            .body
            .into_iter()
            .map(|stmt| rewrite_stmt(stmt, local_functions, local_types, module_aliases))
            .collect::<Result<Vec<_>, _>>()?,
    })
}

fn rewrite_stmt(
    stmt: Stmt,
    local_functions: &HashMap<String, String>,
    local_types: &HashMap<String, String>,
    module_aliases: &ModuleAliases,
) -> Result<Stmt, CompileError> {
    match stmt {
        Stmt::VarDecl {
            line,
            column,
            mutable,
            name,
            declared_type,
            init,
        } => Ok(Stmt::VarDecl {
            line,
            column,
            mutable,
            name,
            declared_type: declared_type.map(|ty| rewrite_type(ty, local_types, module_aliases)),
            init: rewrite_expr(init, local_functions, local_types, module_aliases)?,
        }),
        Stmt::Assign {
            line,
            column,
            target,
            value,
        } => Ok(Stmt::Assign {
            line,
            column,
            target: rewrite_expr(target, local_functions, local_types, module_aliases)?,
            value: rewrite_expr(value, local_functions, local_types, module_aliases)?,
        }),
        Stmt::AddAssign {
            line,
            column,
            target,
            value,
        } => Ok(Stmt::AddAssign {
            line,
            column,
            target: rewrite_expr(target, local_functions, local_types, module_aliases)?,
            value: rewrite_expr(value, local_functions, local_types, module_aliases)?,
        }),
        Stmt::MulAssign {
            line,
            column,
            target,
            value,
        } => Ok(Stmt::MulAssign {
            line,
            column,
            target: rewrite_expr(target, local_functions, local_types, module_aliases)?,
            value: rewrite_expr(value, local_functions, local_types, module_aliases)?,
        }),
        Stmt::SubAssign {
            line,
            column,
            target,
            value,
        } => Ok(Stmt::SubAssign {
            line,
            column,
            target: rewrite_expr(target, local_functions, local_types, module_aliases)?,
            value: rewrite_expr(value, local_functions, local_types, module_aliases)?,
        }),
        Stmt::DivAssign {
            line,
            column,
            target,
            value,
        } => Ok(Stmt::DivAssign {
            line,
            column,
            target: rewrite_expr(target, local_functions, local_types, module_aliases)?,
            value: rewrite_expr(value, local_functions, local_types, module_aliases)?,
        }),
        Stmt::BitAndAssign {
            line,
            column,
            target,
            value,
        } => Ok(Stmt::BitAndAssign {
            line,
            column,
            target: rewrite_expr(target, local_functions, local_types, module_aliases)?,
            value: rewrite_expr(value, local_functions, local_types, module_aliases)?,
        }),
        Stmt::BitOrAssign {
            line,
            column,
            target,
            value,
        } => Ok(Stmt::BitOrAssign {
            line,
            column,
            target: rewrite_expr(target, local_functions, local_types, module_aliases)?,
            value: rewrite_expr(value, local_functions, local_types, module_aliases)?,
        }),
        Stmt::BitXorAssign {
            line,
            column,
            target,
            value,
        } => Ok(Stmt::BitXorAssign {
            line,
            column,
            target: rewrite_expr(target, local_functions, local_types, module_aliases)?,
            value: rewrite_expr(value, local_functions, local_types, module_aliases)?,
        }),
        Stmt::Increment {
            line,
            column,
            target,
        } => Ok(Stmt::Increment {
            line,
            column,
            target: rewrite_expr(target, local_functions, local_types, module_aliases)?,
        }),
        Stmt::Decrement {
            line,
            column,
            target,
        } => Ok(Stmt::Decrement {
            line,
            column,
            target: rewrite_expr(target, local_functions, local_types, module_aliases)?,
        }),
        Stmt::Assert {
            line,
            column,
            condition,
        } => Ok(Stmt::Assert {
            line,
            column,
            condition: rewrite_expr(condition, local_functions, local_types, module_aliases)?,
        }),
        Stmt::Return {
            line,
            column,
            value,
        } => Ok(Stmt::Return {
            line,
            column,
            value: value
                .map(|expr| rewrite_expr(expr, local_functions, local_types, module_aliases))
                .transpose()?,
        }),
        Stmt::Expr { line, column, expr } => Ok(Stmt::Expr {
            line,
            column,
            expr: rewrite_expr(expr, local_functions, local_types, module_aliases)?,
        }),
        Stmt::If {
            line,
            column,
            condition,
            then_body,
            else_body,
        } => Ok(Stmt::If {
            line,
            column,
            condition: rewrite_expr(condition, local_functions, local_types, module_aliases)?,
            then_body: then_body
                .into_iter()
                .map(|stmt| rewrite_stmt(stmt, local_functions, local_types, module_aliases))
                .collect::<Result<Vec<_>, _>>()?,
            else_body: else_body
                .into_iter()
                .map(|stmt| rewrite_stmt(stmt, local_functions, local_types, module_aliases))
                .collect::<Result<Vec<_>, _>>()?,
        }),
        Stmt::Match {
            line,
            column,
            expr,
            arms,
        } => Ok(Stmt::Match {
            line,
            column,
            expr: rewrite_expr(expr, local_functions, local_types, module_aliases)?,
            arms: arms
                .into_iter()
                .map(|arm| {
                    Ok(crate::ast::MatchArm {
                        kind: arm.kind,
                        binding: arm.binding,
                        body: arm
                            .body
                            .into_iter()
                            .map(|stmt| {
                                rewrite_stmt(stmt, local_functions, local_types, module_aliases)
                            })
                            .collect::<Result<Vec<_>, _>>()?,
                    })
                })
                .collect::<Result<Vec<_>, CompileError>>()?,
        }),
        Stmt::ForRange {
            line,
            column,
            pragma,
            var_name,
            start,
            end,
            body,
        } => Ok(Stmt::ForRange {
            line,
            column,
            pragma,
            var_name,
            start: rewrite_expr(start, local_functions, local_types, module_aliases)?,
            end: rewrite_expr(end, local_functions, local_types, module_aliases)?,
            body: body
                .into_iter()
                .map(|stmt| rewrite_stmt(stmt, local_functions, local_types, module_aliases))
                .collect::<Result<Vec<_>, _>>()?,
        }),
        Stmt::ForEach {
            line,
            column,
            var_name,
            iterable,
            body,
        } => Ok(Stmt::ForEach {
            line,
            column,
            var_name,
            iterable: rewrite_expr(iterable, local_functions, local_types, module_aliases)?,
            body: body
                .into_iter()
                .map(|stmt| rewrite_stmt(stmt, local_functions, local_types, module_aliases))
                .collect::<Result<Vec<_>, _>>()?,
        }),
        Stmt::While {
            line,
            column,
            condition,
            body,
        } => Ok(Stmt::While {
            line,
            column,
            condition: rewrite_expr(condition, local_functions, local_types, module_aliases)?,
            body: body
                .into_iter()
                .map(|stmt| rewrite_stmt(stmt, local_functions, local_types, module_aliases))
                .collect::<Result<Vec<_>, _>>()?,
        }),
        Stmt::Loop { line, column, body } => Ok(Stmt::Loop {
            line,
            column,
            body: body
                .into_iter()
                .map(|stmt| rewrite_stmt(stmt, local_functions, local_types, module_aliases))
                .collect::<Result<Vec<_>, _>>()?,
        }),
        Stmt::Continue { line, column } => Ok(Stmt::Continue { line, column }),
    }
}

fn rewrite_expr(
    expr: Expr,
    local_functions: &HashMap<String, String>,
    local_types: &HashMap<String, String>,
    module_aliases: &ModuleAliases,
) -> Result<Expr, CompileError> {
    match expr {
        Expr::Int(_)
        | Expr::Char(_)
        | Expr::Bool(_)
        | Expr::Float(_)
        | Expr::String(_)
        | Expr::Path(_)
        | Expr::None => Ok(expr),
        Expr::ListLiteral(values) => Ok(Expr::ListLiteral(
            values
                .into_iter()
                .map(|value| rewrite_expr(value, local_functions, local_types, module_aliases))
                .collect::<Result<Vec<_>, _>>()?,
        )),
        Expr::Index { base, index } => Ok(Expr::Index {
            base: Box::new(rewrite_expr(
                *base,
                local_functions,
                local_types,
                module_aliases,
            )?),
            index: Box::new(rewrite_expr(
                *index,
                local_functions,
                local_types,
                module_aliases,
            )?),
        }),
        Expr::FieldAccess { base, field } => Ok(Expr::FieldAccess {
            base: Box::new(rewrite_expr(
                *base,
                local_functions,
                local_types,
                module_aliases,
            )?),
            field,
        }),
        Expr::StructInit { name, fields } => Ok(Expr::StructInit {
            name: local_types.get(&name).cloned().unwrap_or(name),
            fields: fields
                .into_iter()
                .map(|field| {
                    Ok(FieldInit {
                        name: field.name,
                        value: rewrite_expr(
                            field.value,
                            local_functions,
                            local_types,
                            module_aliases,
                        )?,
                    })
                })
                .collect::<Result<Vec<_>, CompileError>>()?,
        }),
        Expr::BuiltinCall { name, args } => Ok(Expr::BuiltinCall {
            name,
            args: args
                .into_iter()
                .map(|arg| rewrite_expr(arg, local_functions, local_types, module_aliases))
                .collect::<Result<Vec<_>, _>>()?,
        }),
        Expr::MethodCall {
            receiver,
            method,
            args,
        } => Ok(Expr::MethodCall {
            receiver: Box::new(rewrite_expr(
                *receiver,
                local_functions,
                local_types,
                module_aliases,
            )?),
            method,
            args: args
                .into_iter()
                .map(|arg| rewrite_expr(arg, local_functions, local_types, module_aliases))
                .collect::<Result<Vec<_>, _>>()?,
        }),
        Expr::Call { callee, args } => Ok(Expr::Call {
            callee: Box::new(rewrite_callee(
                *callee,
                local_functions,
                local_types,
                module_aliases,
            )?),
            args: args
                .into_iter()
                .map(|arg| rewrite_expr(arg, local_functions, local_types, module_aliases))
                .collect::<Result<Vec<_>, _>>()?,
        }),
        Expr::Specialize { callee, type_args } => Ok(Expr::Specialize {
            callee: Box::new(rewrite_callee(
                *callee,
                local_functions,
                local_types,
                module_aliases,
            )?),
            type_args: type_args
                .into_iter()
                .map(|ty| rewrite_type(ty, local_types, module_aliases))
                .collect(),
        }),
        Expr::Cast { expr, ty } => Ok(Expr::Cast {
            expr: Box::new(rewrite_expr(
                *expr,
                local_functions,
                local_types,
                module_aliases,
            )?),
            ty: rewrite_type(ty, local_types, module_aliases),
        }),
        Expr::Error { message } => Ok(Expr::Error {
            message: Box::new(rewrite_expr(
                *message,
                local_functions,
                local_types,
                module_aliases,
            )?),
        }),
        Expr::Try(expr) => Ok(Expr::Try(Box::new(rewrite_expr(
            *expr,
            local_functions,
            local_types,
            module_aliases,
        )?))),
        Expr::Unary { op, expr } => Ok(Expr::Unary {
            op,
            expr: Box::new(rewrite_expr(
                *expr,
                local_functions,
                local_types,
                module_aliases,
            )?),
        }),
        Expr::Pack(values) => Ok(Expr::Pack(
            values
                .into_iter()
                .map(|value| rewrite_expr(value, local_functions, local_types, module_aliases))
                .collect::<Result<Vec<_>, _>>()?,
        )),
        Expr::Binary { lhs, op, rhs } => Ok(Expr::Binary {
            lhs: Box::new(rewrite_expr(
                *lhs,
                local_functions,
                local_types,
                module_aliases,
            )?),
            op,
            rhs: Box::new(rewrite_expr(
                *rhs,
                local_functions,
                local_types,
                module_aliases,
            )?),
        }),
    }
}

fn rewrite_callee(
    callee: Expr,
    local_functions: &HashMap<String, String>,
    local_types: &HashMap<String, String>,
    module_aliases: &ModuleAliases,
) -> Result<Expr, CompileError> {
    if let Some(path) = extract_callee_path(&callee) {
        if path.len() == 1 {
            let name = &path[0];
            return Ok(Expr::Path(vec![
                local_functions
                    .get(name)
                    .cloned()
                    .unwrap_or_else(|| name.clone()),
            ]));
        }
        if let Some(module) = module_aliases.get(&path[0]) {
            let member = path[1..].join(".");
            let function = module.functions.get(&member).ok_or_else(|| {
                CompileError::new(format!(
                    "module `{}` has no public function `{}`",
                    path[0], member
                ))
            })?;
            return Ok(Expr::Path(vec![function.clone()]));
        }
        let qualified = path.join(".");
        if let Some(function) = local_functions.get(&qualified) {
            return Ok(Expr::Path(vec![function.clone()]));
        }
    }
    match callee {
        Expr::FieldAccess { base, field } => rewrite_expr(
            Expr::FieldAccess { base, field },
            local_functions,
            local_types,
            module_aliases,
        ),
        other => rewrite_expr(other, local_functions, local_types, module_aliases),
    }
}

fn extract_callee_path(expr: &Expr) -> Option<Vec<String>> {
    match expr {
        Expr::Path(path) => Some(path.clone()),
        Expr::FieldAccess { base, field } => {
            let mut path = extract_callee_path(base)?;
            path.push(field.clone());
            Some(path)
        }
        _ => None,
    }
}

fn rewrite_type(
    ty: Type,
    local_types: &HashMap<String, String>,
    module_aliases: &ModuleAliases,
) -> Type {
    match ty {
        Type::Named(name) => Type::Named(rewrite_named_type(name, local_types, module_aliases)),
        Type::Result(inner) => {
            Type::Result(Box::new(rewrite_type(*inner, local_types, module_aliases)))
        }
        Type::Mut(inner) => Type::Mut(Box::new(rewrite_type(*inner, local_types, module_aliases))),
        Type::Ref(inner) => Type::Ref(Box::new(rewrite_type(*inner, local_types, module_aliases))),
        Type::List(inner) => {
            Type::List(Box::new(rewrite_type(*inner, local_types, module_aliases)))
        }
        Type::U32 => Type::U32,
        other => other,
    }
}

fn rewrite_named_type(
    name: String,
    local_types: &HashMap<String, String>,
    module_aliases: &ModuleAliases,
) -> String {
    if let Some(mapped) = local_types.get(&name) {
        return mapped.clone();
    }
    if let Some((alias, member)) = name.split_once('.') {
        if let Some(module) = module_aliases.get(alias) {
            if let Some(mapped) = module.named_types.get(member) {
                return mapped.clone();
            }
            if let Some(mapped) = module.interfaces.get(member) {
                return mapped.clone();
            }
        }
    }
    name
}

fn instantiate_generic_functions(program: Program) -> Result<Program, CompileError> {
    let interface_names = program
        .interface_defs
        .iter()
        .map(|interface_def| interface_def.name.clone())
        .collect::<HashSet<_>>();
    let type_interfaces = program
        .type_defs
        .iter()
        .map(|type_def| {
            (
                type_def.name.clone(),
                type_def
                    .derives
                    .iter()
                    .filter_map(|derive| match derive {
                        Type::Named(name) => Some(name.clone()),
                        _ => None,
                    })
                    .collect::<HashSet<_>>(),
            )
        })
        .collect::<HashMap<_, _>>();
    let mut templates = HashMap::new();
    let mut concrete_functions = Vec::new();
    for function in program.functions {
        if function.generic_params.is_empty() {
            concrete_functions.push(function);
        } else {
            if function.extern_name.is_some() {
                return Err(CompileError::new(format!(
                    "generic extern functions are not supported: `{}`",
                    function.name
                )));
            }
            templates.insert(function.name.clone(), function);
        }
    }

    let mut instantiator = GenericInstantiator {
        templates,
        instantiated_names: HashMap::new(),
        generated_functions: Vec::new(),
        interface_names,
        type_interfaces,
    };

    let functions = concrete_functions
        .into_iter()
        .map(|function| instantiator.rewrite_function_body(function))
        .collect::<Result<Vec<_>, _>>()?;
    let tests = program
        .tests
        .into_iter()
        .map(|test| instantiator.rewrite_test_body(test))
        .collect::<Result<Vec<_>, _>>()?;

    let mut all_functions = functions;
    all_functions.extend(instantiator.generated_functions);

    Ok(Program {
        module_uses: program.module_uses,
        interface_defs: program.interface_defs,
        type_defs: program.type_defs,
        functions: all_functions,
        tests,
    })
}

struct GenericInstantiator {
    templates: HashMap<String, Function>,
    instantiated_names: HashMap<String, String>,
    generated_functions: Vec<Function>,
    interface_names: HashSet<String>,
    type_interfaces: HashMap<String, HashSet<String>>,
}

impl GenericInstantiator {
    fn rewrite_function_body(&mut self, mut function: Function) -> Result<Function, CompileError> {
        function.body = function
            .body
            .into_iter()
            .map(|stmt| self.rewrite_stmt_generics(stmt))
            .collect::<Result<Vec<_>, _>>()?;
        Ok(function)
    }

    fn rewrite_test_body(&mut self, mut test: TestBlock) -> Result<TestBlock, CompileError> {
        test.body = test
            .body
            .into_iter()
            .map(|stmt| self.rewrite_stmt_generics(stmt))
            .collect::<Result<Vec<_>, _>>()?;
        Ok(test)
    }

    fn rewrite_stmt_generics(&mut self, stmt: Stmt) -> Result<Stmt, CompileError> {
        Ok(match stmt {
            Stmt::VarDecl {
                line,
                column,
                mutable,
                name,
                declared_type,
                init,
            } => Stmt::VarDecl {
                line,
                column,
                mutable,
                name,
                declared_type,
                init: self.rewrite_expr_generics(init)?,
            },
            Stmt::Assign {
                line,
                column,
                target,
                value,
            } => Stmt::Assign {
                line,
                column,
                target: self.rewrite_expr_generics(target)?,
                value: self.rewrite_expr_generics(value)?,
            },
            Stmt::AddAssign {
                line,
                column,
                target,
                value,
            } => Stmt::AddAssign {
                line,
                column,
                target: self.rewrite_expr_generics(target)?,
                value: self.rewrite_expr_generics(value)?,
            },
            Stmt::MulAssign {
                line,
                column,
                target,
                value,
            } => Stmt::MulAssign {
                line,
                column,
                target: self.rewrite_expr_generics(target)?,
                value: self.rewrite_expr_generics(value)?,
            },
            Stmt::SubAssign {
                line,
                column,
                target,
                value,
            } => Stmt::SubAssign {
                line,
                column,
                target: self.rewrite_expr_generics(target)?,
                value: self.rewrite_expr_generics(value)?,
            },
            Stmt::DivAssign {
                line,
                column,
                target,
                value,
            } => Stmt::DivAssign {
                line,
                column,
                target: self.rewrite_expr_generics(target)?,
                value: self.rewrite_expr_generics(value)?,
            },
            Stmt::BitAndAssign {
                line,
                column,
                target,
                value,
            } => Stmt::BitAndAssign {
                line,
                column,
                target: self.rewrite_expr_generics(target)?,
                value: self.rewrite_expr_generics(value)?,
            },
            Stmt::BitOrAssign {
                line,
                column,
                target,
                value,
            } => Stmt::BitOrAssign {
                line,
                column,
                target: self.rewrite_expr_generics(target)?,
                value: self.rewrite_expr_generics(value)?,
            },
            Stmt::BitXorAssign {
                line,
                column,
                target,
                value,
            } => Stmt::BitXorAssign {
                line,
                column,
                target: self.rewrite_expr_generics(target)?,
                value: self.rewrite_expr_generics(value)?,
            },
            Stmt::Increment {
                line,
                column,
                target,
            } => Stmt::Increment {
                line,
                column,
                target: self.rewrite_expr_generics(target)?,
            },
            Stmt::Decrement {
                line,
                column,
                target,
            } => Stmt::Decrement {
                line,
                column,
                target: self.rewrite_expr_generics(target)?,
            },
            Stmt::Assert {
                line,
                column,
                condition,
            } => Stmt::Assert {
                line,
                column,
                condition: self.rewrite_expr_generics(condition)?,
            },
            Stmt::Return { line, column, value } => Stmt::Return {
                line,
                column,
                value: value
                    .map(|expr| self.rewrite_expr_generics(expr))
                    .transpose()?,
            },
            Stmt::If {
                line,
                column,
                condition,
                then_body,
                else_body,
            } => Stmt::If {
                line,
                column,
                condition: self.rewrite_expr_generics(condition)?,
                then_body: then_body
                    .into_iter()
                    .map(|stmt| self.rewrite_stmt_generics(stmt))
                    .collect::<Result<Vec<_>, _>>()?,
                else_body: else_body
                    .into_iter()
                    .map(|stmt| self.rewrite_stmt_generics(stmt))
                    .collect::<Result<Vec<_>, _>>()?,
            },
            Stmt::Match {
                line,
                column,
                expr,
                arms,
            } => Stmt::Match {
                line,
                column,
                expr: self.rewrite_expr_generics(expr)?,
                arms: arms
                    .into_iter()
                    .map(|arm| {
                        Ok(crate::ast::MatchArm {
                            kind: arm.kind,
                            binding: arm.binding,
                            body: arm
                                .body
                                .into_iter()
                                .map(|stmt| self.rewrite_stmt_generics(stmt))
                                .collect::<Result<Vec<_>, _>>()?,
                        })
                    })
                    .collect::<Result<Vec<_>, CompileError>>()?,
            },
            Stmt::Expr { line, column, expr } => Stmt::Expr {
                line,
                column,
                expr: self.rewrite_expr_generics(expr)?,
            },
            Stmt::ForRange {
                line,
                column,
                pragma,
                var_name,
                start,
                end,
                body,
            } => Stmt::ForRange {
                line,
                column,
                pragma,
                var_name,
                start: self.rewrite_expr_generics(start)?,
                end: self.rewrite_expr_generics(end)?,
                body: body
                    .into_iter()
                    .map(|stmt| self.rewrite_stmt_generics(stmt))
                    .collect::<Result<Vec<_>, _>>()?,
            },
            Stmt::ForEach {
                line,
                column,
                var_name,
                iterable,
                body,
            } => Stmt::ForEach {
                line,
                column,
                var_name,
                iterable: self.rewrite_expr_generics(iterable)?,
                body: body
                    .into_iter()
                    .map(|stmt| self.rewrite_stmt_generics(stmt))
                    .collect::<Result<Vec<_>, _>>()?,
            },
            Stmt::While {
                line,
                column,
                condition,
                body,
            } => Stmt::While {
                line,
                column,
                condition: self.rewrite_expr_generics(condition)?,
                body: body
                    .into_iter()
                    .map(|stmt| self.rewrite_stmt_generics(stmt))
                    .collect::<Result<Vec<_>, _>>()?,
            },
            Stmt::Loop { line, column, body } => Stmt::Loop {
                line,
                column,
                body: body
                    .into_iter()
                    .map(|stmt| self.rewrite_stmt_generics(stmt))
                    .collect::<Result<Vec<_>, _>>()?,
            },
            Stmt::Continue { line, column } => Stmt::Continue { line, column },
        })
    }

    fn rewrite_expr_generics(&mut self, expr: Expr) -> Result<Expr, CompileError> {
        Ok(match expr {
            Expr::ListLiteral(values) => Expr::ListLiteral(
                values
                    .into_iter()
                    .map(|value| self.rewrite_expr_generics(value))
                    .collect::<Result<Vec<_>, _>>()?,
            ),
            Expr::Index { base, index } => Expr::Index {
                base: Box::new(self.rewrite_expr_generics(*base)?),
                index: Box::new(self.rewrite_expr_generics(*index)?),
            },
            Expr::FieldAccess { base, field } => Expr::FieldAccess {
                base: Box::new(self.rewrite_expr_generics(*base)?),
                field,
            },
            Expr::StructInit { name, fields } => Expr::StructInit {
                name,
                fields: fields
                    .into_iter()
                    .map(|field| {
                        Ok(FieldInit {
                            name: field.name,
                            value: self.rewrite_expr_generics(field.value)?,
                        })
                    })
                    .collect::<Result<Vec<_>, CompileError>>()?,
            },
            Expr::BuiltinCall { name, args } => Expr::BuiltinCall {
                name,
                args: args
                    .into_iter()
                    .map(|arg| self.rewrite_expr_generics(arg))
                    .collect::<Result<Vec<_>, _>>()?,
            },
            Expr::MethodCall {
                receiver,
                method,
                args,
            } => Expr::MethodCall {
                receiver: Box::new(self.rewrite_expr_generics(*receiver)?),
                method,
                args: args
                    .into_iter()
                    .map(|arg| self.rewrite_expr_generics(arg))
                    .collect::<Result<Vec<_>, _>>()?,
            },
            Expr::Call { callee, args } => {
                let callee = self.rewrite_expr_generics(*callee)?;
                let args = args
                    .into_iter()
                    .map(|arg| self.rewrite_expr_generics(arg))
                    .collect::<Result<Vec<_>, _>>()?;
                if let Expr::Specialize { callee, type_args } = callee {
                    let Some(path) = extract_callee_path(&callee) else {
                        return Err(CompileError::new(
                            "generic specialization currently requires a direct function name",
                        ));
                    };
                    let specialized =
                        self.instantiate_specialization(&path.join("."), &type_args)?;
                    Expr::Call {
                        callee: Box::new(Expr::Path(vec![specialized])),
                        args,
                    }
                } else {
                    Expr::Call {
                        callee: Box::new(callee),
                        args,
                    }
                }
            }
            Expr::Specialize { callee, type_args } => Expr::Specialize {
                callee: Box::new(self.rewrite_expr_generics(*callee)?),
                type_args,
            },
            Expr::Cast { expr, ty } => Expr::Cast {
                expr: Box::new(self.rewrite_expr_generics(*expr)?),
                ty,
            },
            Expr::Error { message } => Expr::Error {
                message: Box::new(self.rewrite_expr_generics(*message)?),
            },
            Expr::Try(expr) => Expr::Try(Box::new(self.rewrite_expr_generics(*expr)?)),
            Expr::Unary { op, expr } => Expr::Unary {
                op,
                expr: Box::new(self.rewrite_expr_generics(*expr)?),
            },
            Expr::Pack(values) => Expr::Pack(
                values
                    .into_iter()
                    .map(|value| self.rewrite_expr_generics(value))
                    .collect::<Result<Vec<_>, _>>()?,
            ),
            Expr::Binary { lhs, op, rhs } => Expr::Binary {
                lhs: Box::new(self.rewrite_expr_generics(*lhs)?),
                op,
                rhs: Box::new(self.rewrite_expr_generics(*rhs)?),
            },
            other => other,
        })
    }

    fn instantiate_specialization(
        &mut self,
        name: &str,
        type_args: &[Type],
    ) -> Result<String, CompileError> {
        let key = specialization_key(name, type_args);
        if let Some(existing) = self.instantiated_names.get(&key) {
            return Ok(existing.clone());
        }
        let template = self
            .templates
            .get(name)
            .cloned()
            .ok_or_else(|| CompileError::new(format!("unknown generic function `{name}`")))?;
        if template.generic_params.len() != type_args.len() {
            return Err(CompileError::new(format!(
                "generic function `{}` expects {} type arguments but received {}",
                template.name,
                template.generic_params.len(),
                type_args.len()
            )));
        }

        let mut substitutions = HashMap::new();
        for (param, type_arg) in template.generic_params.iter().zip(type_args.iter()) {
            if !param.constraints.is_empty()
                && !param
                    .constraints
                    .iter()
                    .any(|allowed| self.constraint_matches(allowed, type_arg))
            {
                return Err(CompileError::new(format!(
                    "generic parameter `{}` on `{}` does not allow type {}",
                    param.name,
                    template.name,
                    describe_type(type_arg)
                )));
            }
            substitutions.insert(param.name.clone(), type_arg.clone());
        }

        let specialized_name = format!("{}__generic__{}", template.name, specialization_suffix(type_args));
        self.instantiated_names
            .insert(key, specialized_name.clone());

        let mut specialized = template.clone();
        specialized.name = specialized_name.clone();
        specialized.generic_params.clear();
        specialized.params = specialized
            .params
            .into_iter()
            .map(|param| crate::ast::Param {
                name: param.name,
                ty: substitute_type(param.ty, &substitutions),
            })
            .collect();
        specialized.return_type = substitute_type(specialized.return_type, &substitutions);
        if specialized.return_type == Type::Void {
            if let Some(inferred) = infer_implicit_return_type(&specialized, &substitutions) {
                specialized.return_type = inferred;
            }
        }
        specialized.body = specialized
            .body
            .into_iter()
            .map(|stmt| substitute_stmt(stmt, &substitutions))
            .collect();
        specialized = self.rewrite_function_body(specialized)?;
        self.generated_functions.push(specialized);

        Ok(specialized_name)
    }

    fn constraint_matches(&self, constraint: &Type, type_arg: &Type) -> bool {
        if constraint == type_arg {
            return true;
        }
        match (constraint, type_arg) {
            (Type::Named(interface_name), Type::Named(type_name))
                if self.interface_names.contains(interface_name) =>
            {
                self.type_interfaces
                    .get(type_name)
                    .is_some_and(|interfaces| interfaces.contains(interface_name))
            }
            _ => false,
        }
    }
}

fn substitute_stmt(stmt: Stmt, substitutions: &HashMap<String, Type>) -> Stmt {
    match stmt {
        Stmt::VarDecl {
            line,
            column,
            mutable,
            name,
            declared_type,
            init,
        } => Stmt::VarDecl {
            line,
            column,
            mutable,
            name,
            declared_type: declared_type.map(|ty| substitute_type(ty, substitutions)),
            init: substitute_expr(init, substitutions),
        },
        Stmt::Assign {
            line,
            column,
            target,
            value,
        } => Stmt::Assign {
            line,
            column,
            target: substitute_expr(target, substitutions),
            value: substitute_expr(value, substitutions),
        },
        Stmt::AddAssign {
            line,
            column,
            target,
            value,
        } => Stmt::AddAssign {
            line,
            column,
            target: substitute_expr(target, substitutions),
            value: substitute_expr(value, substitutions),
        },
        Stmt::MulAssign {
            line,
            column,
            target,
            value,
        } => Stmt::MulAssign {
            line,
            column,
            target: substitute_expr(target, substitutions),
            value: substitute_expr(value, substitutions),
        },
        Stmt::SubAssign {
            line,
            column,
            target,
            value,
        } => Stmt::SubAssign {
            line,
            column,
            target: substitute_expr(target, substitutions),
            value: substitute_expr(value, substitutions),
        },
        Stmt::DivAssign {
            line,
            column,
            target,
            value,
        } => Stmt::DivAssign {
            line,
            column,
            target: substitute_expr(target, substitutions),
            value: substitute_expr(value, substitutions),
        },
        Stmt::BitAndAssign {
            line,
            column,
            target,
            value,
        } => Stmt::BitAndAssign {
            line,
            column,
            target: substitute_expr(target, substitutions),
            value: substitute_expr(value, substitutions),
        },
        Stmt::BitOrAssign {
            line,
            column,
            target,
            value,
        } => Stmt::BitOrAssign {
            line,
            column,
            target: substitute_expr(target, substitutions),
            value: substitute_expr(value, substitutions),
        },
        Stmt::BitXorAssign {
            line,
            column,
            target,
            value,
        } => Stmt::BitXorAssign {
            line,
            column,
            target: substitute_expr(target, substitutions),
            value: substitute_expr(value, substitutions),
        },
        Stmt::Increment {
            line,
            column,
            target,
        } => Stmt::Increment {
            line,
            column,
            target: substitute_expr(target, substitutions),
        },
        Stmt::Decrement {
            line,
            column,
            target,
        } => Stmt::Decrement {
            line,
            column,
            target: substitute_expr(target, substitutions),
        },
        Stmt::Assert {
            line,
            column,
            condition,
        } => Stmt::Assert {
            line,
            column,
            condition: substitute_expr(condition, substitutions),
        },
        Stmt::Return { line, column, value } => Stmt::Return {
            line,
            column,
            value: value.map(|expr| substitute_expr(expr, substitutions)),
        },
        Stmt::If {
            line,
            column,
            condition,
            then_body,
            else_body,
        } => Stmt::If {
            line,
            column,
            condition: substitute_expr(condition, substitutions),
            then_body: then_body
                .into_iter()
                .map(|stmt| substitute_stmt(stmt, substitutions))
                .collect(),
            else_body: else_body
                .into_iter()
                .map(|stmt| substitute_stmt(stmt, substitutions))
                .collect(),
        },
        Stmt::Match {
            line,
            column,
            expr,
            arms,
        } => Stmt::Match {
            line,
            column,
            expr: substitute_expr(expr, substitutions),
            arms: arms
                .into_iter()
                .map(|arm| crate::ast::MatchArm {
                    kind: arm.kind,
                    binding: arm.binding,
                    body: arm
                        .body
                        .into_iter()
                        .map(|stmt| substitute_stmt(stmt, substitutions))
                        .collect(),
                })
                .collect(),
        },
        Stmt::Expr { line, column, expr } => Stmt::Expr {
            line,
            column,
            expr: substitute_expr(expr, substitutions),
        },
        Stmt::ForRange {
            line,
            column,
            pragma,
            var_name,
            start,
            end,
            body,
        } => Stmt::ForRange {
            line,
            column,
            pragma,
            var_name,
            start: substitute_expr(start, substitutions),
            end: substitute_expr(end, substitutions),
            body: body
                .into_iter()
                .map(|stmt| substitute_stmt(stmt, substitutions))
                .collect(),
        },
        Stmt::ForEach {
            line,
            column,
            var_name,
            iterable,
            body,
        } => Stmt::ForEach {
            line,
            column,
            var_name,
            iterable: substitute_expr(iterable, substitutions),
            body: body
                .into_iter()
                .map(|stmt| substitute_stmt(stmt, substitutions))
                .collect(),
        },
        Stmt::While {
            line,
            column,
            condition,
            body,
        } => Stmt::While {
            line,
            column,
            condition: substitute_expr(condition, substitutions),
            body: body
                .into_iter()
                .map(|stmt| substitute_stmt(stmt, substitutions))
                .collect(),
        },
        Stmt::Loop { line, column, body } => Stmt::Loop {
            line,
            column,
            body: body
                .into_iter()
                .map(|stmt| substitute_stmt(stmt, substitutions))
                .collect(),
        },
        Stmt::Continue { line, column } => Stmt::Continue { line, column },
    }
}

fn substitute_expr(expr: Expr, substitutions: &HashMap<String, Type>) -> Expr {
    match expr {
        Expr::Bool(value) => Expr::Bool(value),
        Expr::Char(value) => Expr::Char(value),
        Expr::Path(path) => {
            if path.len() == 1 {
                if let Some(Type::Named(name)) = substitutions.get(&path[0]) {
                    return Expr::Path(vec![name.clone()]);
                }
            }
            Expr::Path(path)
        }
        Expr::ListLiteral(values) => Expr::ListLiteral(
            values
                .into_iter()
                .map(|value| substitute_expr(value, substitutions))
                .collect(),
        ),
        Expr::Index { base, index } => Expr::Index {
            base: Box::new(substitute_expr(*base, substitutions)),
            index: Box::new(substitute_expr(*index, substitutions)),
        },
        Expr::FieldAccess { base, field } => {
            if let Expr::Path(path) = base.as_ref() {
                if path.len() == 1 {
                    if let Some(Type::Named(type_name)) = substitutions.get(&path[0]) {
                        return Expr::Path(vec![format!("{type_name}.{field}")]);
                    }
                }
            }
            Expr::FieldAccess {
                base: Box::new(substitute_expr(*base, substitutions)),
                field,
            }
        }
        Expr::StructInit { name, fields } => Expr::StructInit {
            name,
            fields: fields
                .into_iter()
                .map(|field| FieldInit {
                    name: field.name,
                    value: substitute_expr(field.value, substitutions),
                })
                .collect(),
        },
        Expr::BuiltinCall { name, args } => Expr::BuiltinCall {
            name,
            args: args
                .into_iter()
                .map(|arg| substitute_expr(arg, substitutions))
                .collect(),
        },
        Expr::MethodCall {
            receiver,
            method,
            args,
        } => Expr::MethodCall {
            receiver: Box::new(substitute_expr(*receiver, substitutions)),
            method,
            args: args
                .into_iter()
                .map(|arg| substitute_expr(arg, substitutions))
                .collect(),
        },
        Expr::Call { callee, args } => Expr::Call {
            callee: Box::new(substitute_expr(*callee, substitutions)),
            args: args
                .into_iter()
                .map(|arg| substitute_expr(arg, substitutions))
                .collect(),
        },
        Expr::Specialize { callee, type_args } => Expr::Specialize {
            callee: Box::new(substitute_expr(*callee, substitutions)),
            type_args: type_args
                .into_iter()
                .map(|ty| substitute_type(ty, substitutions))
                .collect(),
        },
        Expr::Cast { expr, ty } => Expr::Cast {
            expr: Box::new(substitute_expr(*expr, substitutions)),
            ty: substitute_type(ty, substitutions),
        },
        Expr::Error { message } => Expr::Error {
            message: Box::new(substitute_expr(*message, substitutions)),
        },
        Expr::Try(expr) => Expr::Try(Box::new(substitute_expr(*expr, substitutions))),
        Expr::Unary { op, expr } => Expr::Unary {
            op,
            expr: Box::new(substitute_expr(*expr, substitutions)),
        },
        Expr::Pack(values) => Expr::Pack(
            values
                .into_iter()
                .map(|value| substitute_expr(value, substitutions))
                .collect(),
        ),
        Expr::Binary { lhs, op, rhs } => Expr::Binary {
            lhs: Box::new(substitute_expr(*lhs, substitutions)),
            op,
            rhs: Box::new(substitute_expr(*rhs, substitutions)),
        },
        other => other,
    }
}

fn substitute_type(ty: Type, substitutions: &HashMap<String, Type>) -> Type {
    match ty {
        Type::Named(name) => substitutions.get(&name).cloned().unwrap_or(Type::Named(name)),
        Type::Result(inner) => Type::Result(Box::new(substitute_type(*inner, substitutions))),
        Type::Mut(inner) => Type::Mut(Box::new(substitute_type(*inner, substitutions))),
        Type::Ref(inner) => Type::Ref(Box::new(substitute_type(*inner, substitutions))),
        Type::List(inner) => Type::List(Box::new(substitute_type(*inner, substitutions))),
        other => other,
    }
}

fn infer_implicit_return_type(
    function: &Function,
    substitutions: &HashMap<String, Type>,
) -> Option<Type> {
    let param_types = function
        .params
        .iter()
        .map(|param| (param.name.clone(), substitute_type(param.ty.clone(), substitutions)))
        .collect::<HashMap<_, _>>();
    infer_return_type_from_stmts(&function.body, &param_types)
}

fn infer_return_type_from_stmts(body: &[Stmt], param_types: &HashMap<String, Type>) -> Option<Type> {
    for stmt in body {
        match stmt {
            Stmt::Return { value: Some(expr), .. } => {
                if let Some(ty) = infer_expr_type_from_template(expr, param_types) {
                    return Some(ty);
                }
            }
            Stmt::If {
                then_body,
                else_body,
                ..
            } => {
                if let Some(ty) = infer_return_type_from_stmts(then_body, param_types) {
                    return Some(ty);
                }
                if let Some(ty) = infer_return_type_from_stmts(else_body, param_types) {
                    return Some(ty);
                }
            }
            Stmt::Match { arms, .. } => {
                for arm in arms {
                    if let Some(ty) = infer_return_type_from_stmts(&arm.body, param_types) {
                        return Some(ty);
                    }
                }
            }
            Stmt::ForRange { body, .. }
            | Stmt::ForEach { body, .. }
            | Stmt::While { body, .. }
            | Stmt::Loop { body, .. } => {
                if let Some(ty) = infer_return_type_from_stmts(body, param_types) {
                    return Some(ty);
                }
            }
            _ => {}
        }
    }
    None
}

fn infer_expr_type_from_template(expr: &Expr, param_types: &HashMap<String, Type>) -> Option<Type> {
    match expr {
        Expr::Int(_) => Some(Type::I32),
        Expr::Char(_) => Some(Type::U8),
        Expr::Bool(_) => Some(Type::Bool),
        Expr::Float(_) => Some(Type::F32),
        Expr::String(_) => Some(Type::Ref(Box::new(Type::U8))),
        Expr::Path(path) if path.len() == 1 => param_types.get(&path[0]).cloned(),
        Expr::Cast { ty, .. } => Some(ty.clone()),
        Expr::Unary { op, expr } => {
            let inner = infer_expr_type_from_template(expr, param_types)?;
            match op {
                crate::ast::UnaryOp::LogicalNot => Some(Type::Bool),
                _ => Some(inner),
            }
        }
        Expr::Binary { lhs, op, rhs } => {
            let lhs_ty = infer_expr_type_from_template(lhs, param_types)?;
            let rhs_ty = infer_expr_type_from_template(rhs, param_types)?;
            match op {
                crate::ast::BinaryOp::Add
                | crate::ast::BinaryOp::Subtract
                | crate::ast::BinaryOp::Divide
                | crate::ast::BinaryOp::Multiply
                | crate::ast::BinaryOp::Modulo
                | crate::ast::BinaryOp::BitAnd
                | crate::ast::BinaryOp::BitOr
                | crate::ast::BinaryOp::BitXor
                | crate::ast::BinaryOp::ShiftLeft
                | crate::ast::BinaryOp::ShiftRight
                    if lhs_ty == rhs_ty =>
                {
                    Some(lhs_ty)
                }
                crate::ast::BinaryOp::LessThan
                | crate::ast::BinaryOp::LessEqual
                | crate::ast::BinaryOp::GreaterThan
                | crate::ast::BinaryOp::GreaterEqual
                | crate::ast::BinaryOp::Equal
                | crate::ast::BinaryOp::NotEqual
                | crate::ast::BinaryOp::LogicalAnd
                | crate::ast::BinaryOp::LogicalOr => Some(Type::Bool),
                _ => None,
            }
        }
        _ => None,
    }
}

fn specialization_key(name: &str, type_args: &[Type]) -> String {
    format!("{name}[{}]", specialization_suffix(type_args))
}

fn specialization_suffix(type_args: &[Type]) -> String {
    type_args
        .iter()
        .map(type_suffix_for_specialization)
        .collect::<Vec<_>>()
        .join("__")
}

fn type_suffix_for_specialization(ty: &Type) -> String {
    match ty {
        Type::Void => "void".to_string(),
        Type::Bool => "bool".to_string(),
        Type::I8 => "i8".to_string(),
        Type::I16 => "i16".to_string(),
        Type::I32 => "i32".to_string(),
        Type::I64 => "i64".to_string(),
        Type::Isize => "isize".to_string(),
        Type::U8 => "u8".to_string(),
        Type::U16 => "u16".to_string(),
        Type::U32 => "u32".to_string(),
        Type::U64 => "u64".to_string(),
        Type::Usize => "usize".to_string(),
        Type::F32 => "f32".to_string(),
        Type::F64 => "f64".to_string(),
        Type::Named(name) => sanitize_generic_name(name),
        Type::Mut(inner) => format!("mut__{}", type_suffix_for_specialization(inner)),
        Type::Ref(inner) => format!("ref__{}", type_suffix_for_specialization(inner)),
        Type::List(inner) => format!("list__{}", type_suffix_for_specialization(inner)),
        Type::Result(inner) => format!("result__{}", type_suffix_for_specialization(inner)),
        Type::Error => "error".to_string(),
        Type::None => "none".to_string(),
    }
}

fn sanitize_generic_name(name: &str) -> String {
    name.chars()
        .map(|ch| {
            if ch.is_ascii_alphanumeric() || ch == '_' {
                ch
            } else {
                '_'
            }
        })
        .collect()
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
        Type::U8 => "u8".to_string(),
        Type::U16 => "u16".to_string(),
        Type::U32 => "u32".to_string(),
        Type::U64 => "u64".to_string(),
        Type::Usize => "usize".to_string(),
        Type::F32 => "f32".to_string(),
        Type::F64 => "f64".to_string(),
        Type::Named(name) => name.clone(),
        Type::Mut(inner) => format!("mut({})", describe_type(inner)),
        Type::Ref(inner) => format!("ref({})", describe_type(inner)),
        Type::List(inner) => format!("list[{}]", describe_type(inner)),
        Type::Result(inner) => format!("{}|error", describe_type(inner)),
        Type::Error => "error".to_string(),
        Type::None => "none".to_string(),
    }
}

#[cfg(test)]
mod tests {
    use std::{
        fs,
        path::{Path, PathBuf},
        time::{SystemTime, UNIX_EPOCH},
    };

    use super::{resolve_entry_program, resolve_module_use_path};

    #[test]
    fn resolves_module_calls_to_namespaced_functions() {
        let temp_dir = create_temp_dir();
        let entry = temp_dir.join("main.scar");
        let module = temp_dir.join("some_file.scar");

        fs::write(
            &entry,
            "val some_module = use(\"some_file\")\npub def main() void\n\tsome_module.some_function()\nend\n",
        )
        .unwrap();
        fs::write(
            &module,
            "pub def some_function() void\n\t@puts(\"hello\")\nend\n",
        )
        .unwrap();

        let program = resolve_entry_program(&entry).unwrap();

        assert!(program.type_defs.is_empty());
        assert_eq!(program.functions.len(), 2);
        assert_eq!(program.functions[0].name, "some_file__some_function");
        assert_eq!(program.functions[1].name, "main");

        fs::remove_dir_all(temp_dir).unwrap();
    }

    fn create_temp_dir() -> PathBuf {
        let unique = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_nanos();
        let dir = std::env::temp_dir().join(format!("scar-module-test-{unique}"));
        fs::create_dir_all(&dir).unwrap();
        dir
    }

    #[allow(dead_code)]
    fn _assert_exists(path: &Path) {
        assert!(path.exists());
    }

    #[test]
    fn resolves_module_type_defs() {
        let temp_dir = create_temp_dir();
        let entry = temp_dir.join("main.scar");
        let module = temp_dir.join("some_file.scar");

        fs::write(
            &entry,
            "val some_module = use(\"some_file\")\npub def main() void\n\tsome_module.some_other_function()\nend\n",
        )
        .unwrap();
        fs::write(
            &module,
            "type SomeType\n\tx i32\nend\n\npub def some_other_function() void\n\tval st = SomeType(x: 10)\n\t@print(\"{d}\", {st.x})\nend\n",
        )
        .unwrap();

        let program = resolve_entry_program(&entry).unwrap();

        assert_eq!(program.type_defs.len(), 1);
        assert_eq!(program.type_defs[0].name, "some_file__SomeType");

        fs::remove_dir_all(temp_dir).unwrap();
    }

    #[test]
    fn resolves_namespaced_type_functions() {
        let temp_dir = create_temp_dir();
        let entry = temp_dir.join("main.scar");

        fs::write(
            &entry,
            "pub type Arena\n\tcap usize\nend\n\npub def Arena.new(cap usize) Arena\n\treturn Arena(cap: cap)\nend\n\npub def main() void\n\tval arena = Arena.new(4 as usize)\n\t@print(\"{d}\", {arena.cap})\nend\n",
        )
        .unwrap();

        let program = resolve_entry_program(&entry).unwrap();

        assert_eq!(program.type_defs[0].name, "Arena");
        assert_eq!(program.functions[0].name, "Arena.new");

        fs::remove_dir_all(temp_dir).unwrap();
    }

    #[test]
    fn resolves_specialized_same_module_type_functions() {
        let temp_dir = create_temp_dir();
        let entry = temp_dir.join("main.scar");

        fs::write(
            &entry,
            "pub type Arena\n\tcap usize\nend\n\npub def StringBuilder.new[A](ac A, cap usize) usize\n\treturn cap\nend\n\npub def main() void\n\tStringBuilder.new[Arena](Arena(cap: 1 as usize), 4 as usize)\nend\n",
        )
        .unwrap();

        let program = resolve_entry_program(&entry).unwrap();

        assert_eq!(program.functions.len(), 2);
        assert_eq!(program.functions[0].name, "main");
        assert!(program.functions[1]
            .name
            .starts_with("StringBuilder.new__generic__Arena"));

        fs::remove_dir_all(temp_dir).unwrap();
    }

    #[test]
    fn rejects_private_module_function_calls() {
        let temp_dir = create_temp_dir();
        let entry = temp_dir.join("main.scar");
        let module = temp_dir.join("some_file.scar");

        fs::write(
            &entry,
            "val some_module = use(\"some_file\")\npub def main() void\n\tsome_module.hidden()\nend\n",
        )
        .unwrap();
        fs::write(&module, "def hidden() void\nend\n").unwrap();

        let error = resolve_entry_program(&entry).unwrap_err();

        assert!(
            error
                .to_string()
                .contains("module `some_module` has no public function `hidden`")
        );

        fs::remove_dir_all(temp_dir).unwrap();
    }

    #[test]
    fn resolves_public_extern_functions_from_modules() {
        let temp_dir = create_temp_dir();
        let entry = temp_dir.join("main.scar");
        let module = temp_dir.join("some_file.scar");

        fs::write(
            &entry,
            "val some_module = use(\"some_file\")\npub def main() void\n\tsome_module.sleep(1 as u32)\nend\n",
        )
        .unwrap();
        fs::write(&module, "pub extern def sleep(t u32) void :: \"sleep\"\n").unwrap();

        let program = resolve_entry_program(&entry).unwrap();

        assert_eq!(program.functions.len(), 2);
        assert_eq!(program.functions[0].name, "some_file__sleep");
        assert!(program.functions[0].is_pub);
        assert_eq!(program.functions[0].extern_name.as_deref(), Some("sleep"));

        fs::remove_dir_all(temp_dir).unwrap();
    }

    #[test]
    fn std_imports_prefer_local_std_root() {
        let resolved = resolve_module_use_path(
            Path::new("/project/src"),
            "std/string",
            Some(Path::new("/project/lib/std")),
            Some(Path::new("/home/test/.scar/lib/std")),
        );

        assert_eq!(resolved, Path::new("/project/lib/std/string.scar"));
    }

    #[test]
    fn std_imports_fall_back_to_home_std_root() {
        let resolved = resolve_module_use_path(
            Path::new("/project/src"),
            "std/string",
            None,
            Some(Path::new("/home/test/.scar/lib/std")),
        );

        assert_eq!(resolved, Path::new("/home/test/.scar/lib/std/string.scar"));
    }

    #[test]
    fn non_std_imports_stay_relative_to_current_file() {
        let resolved = resolve_module_use_path(
            Path::new("/project/src"),
            "some_file",
            Some(Path::new("/project/lib/std")),
            Some(Path::new("/home/test/.scar/lib/std")),
        );

        assert_eq!(resolved, Path::new("/project/src/some_file.scar"));
    }
}
