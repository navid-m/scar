use std::{
    collections::{HashMap, HashSet},
    fs,
    path::{Path, PathBuf},
};

use crate::{
    CompileError,
    ast::{Expr, Function, ModuleUse, Program, Stmt},
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
        cache: HashMap::new(),
        emitted_modules: HashSet::new(),
        resolved_functions: Vec::new(),
        visiting: Vec::new(),
    };

    let program = parse_program_file(&entry)?;
    let module_aliases = resolver.resolve_module_uses(&program.module_uses, &entry)?;
    let local_functions = build_function_map(&program.functions, None);
    let mut functions = resolver.resolved_functions;
    for function in program.functions {
        functions.push(rewrite_function(function, &local_functions, &module_aliases)?);
    }

    Ok(Program {
        module_uses: Vec::new(),
        functions,
    })
}

#[derive(Clone)]
struct ModuleExports {
    functions: HashMap<String, String>,
}

type ModuleAliases = HashMap<String, HashMap<String, String>>;

struct Resolver {
    root_dir: PathBuf,
    cache: HashMap<PathBuf, ModuleExports>,
    emitted_modules: HashSet<PathBuf>,
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
            let module_path = current_dir.join(&module_use.path).with_extension("scar");
            let exports = self.resolve_module(&module_path)?;
            aliases.insert(module_use.name.clone(), exports.functions);
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
            return Err(CompileError::new(format!("module import cycle detected: {cycle}")));
        }

        self.visiting.push(module_path.clone());
        let parsed = parse_program_file(&module_path)?;
        let module_aliases = self.resolve_module_uses(&parsed.module_uses, &module_path)?;
        let prefix = module_prefix(&module_path, &self.root_dir);
        let local_functions = build_function_map(&parsed.functions, Some(&prefix));

        let mut rewritten_functions = Vec::new();
        for function in parsed.functions {
            rewritten_functions.push(rewrite_function(function, &local_functions, &module_aliases)?);
        }

        if self.emitted_modules.insert(module_path.clone()) {
            self.resolved_functions.extend(rewritten_functions);
        }

        self.visiting.pop();
        let exports = ModuleExports {
            functions: local_functions,
        };
        self.cache.insert(module_path, exports.clone());
        Ok(exports)
    }
}

fn parse_program_file(path: &Path) -> Result<Program, CompileError> {
    let source = fs::read_to_string(path)
        .map_err(|error| CompileError::new(format!("failed to read {}: {error}", path.display())))?;
    let tokens = lex(&source)
        .map_err(|error| CompileError::new(format!("in {}: {error}", path.display())))?;
    parse_program(tokens).map_err(|error| CompileError::new(format!("in {}: {error}", path.display())))
}

fn canonicalize_path(path: &Path) -> Result<PathBuf, CompileError> {
    fs::canonicalize(path)
        .map_err(|error| CompileError::new(format!("failed to resolve {}: {error}", path.display())))
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
    module_aliases: &ModuleAliases,
) -> Result<Function, CompileError> {
    if let Some(mapped) = local_functions.get(&function.name) {
        function.name = mapped.clone();
    }
    function.body = function
        .body
        .into_iter()
        .map(|stmt| rewrite_stmt(stmt, local_functions, module_aliases))
        .collect::<Result<Vec<_>, _>>()?;
    Ok(function)
}

fn rewrite_stmt(
    stmt: Stmt,
    local_functions: &HashMap<String, String>,
    module_aliases: &ModuleAliases,
) -> Result<Stmt, CompileError> {
    match stmt {
        Stmt::VarDecl {
            mutable,
            name,
            init,
        } => Ok(Stmt::VarDecl {
            mutable,
            name,
            init: rewrite_expr(init, local_functions, module_aliases)?,
        }),
        Stmt::Assign { target, value } => Ok(Stmt::Assign {
            target: rewrite_expr(target, local_functions, module_aliases)?,
            value: rewrite_expr(value, local_functions, module_aliases)?,
        }),
        Stmt::AddAssign { target, value } => Ok(Stmt::AddAssign {
            target: rewrite_expr(target, local_functions, module_aliases)?,
            value: rewrite_expr(value, local_functions, module_aliases)?,
        }),
        Stmt::Return(value) => Ok(Stmt::Return(
            value
                .map(|expr| rewrite_expr(expr, local_functions, module_aliases))
                .transpose()?,
        )),
        Stmt::Expr(expr) => Ok(Stmt::Expr(rewrite_expr(expr, local_functions, module_aliases)?)),
        Stmt::For {
            pragma,
            var_name,
            start,
            end,
            body,
        } => Ok(Stmt::For {
            pragma,
            var_name,
            start: rewrite_expr(start, local_functions, module_aliases)?,
            end: rewrite_expr(end, local_functions, module_aliases)?,
            body: body
                .into_iter()
                .map(|stmt| rewrite_stmt(stmt, local_functions, module_aliases))
                .collect::<Result<Vec<_>, _>>()?,
        }),
    }
}

fn rewrite_expr(
    expr: Expr,
    local_functions: &HashMap<String, String>,
    module_aliases: &ModuleAliases,
) -> Result<Expr, CompileError> {
    match expr {
        Expr::Int(_) | Expr::String(_) | Expr::Path(_) => Ok(expr),
        Expr::BuiltinCall { name, args } => Ok(Expr::BuiltinCall {
            name,
            args: args
                .into_iter()
                .map(|arg| rewrite_expr(arg, local_functions, module_aliases))
                .collect::<Result<Vec<_>, _>>()?,
        }),
        Expr::Call { callee, args } => Ok(Expr::Call {
            callee: Box::new(rewrite_callee(*callee, local_functions, module_aliases)?),
            args: args
                .into_iter()
                .map(|arg| rewrite_expr(arg, local_functions, module_aliases))
                .collect::<Result<Vec<_>, _>>()?,
        }),
        Expr::Pack(values) => Ok(Expr::Pack(
            values
                .into_iter()
                .map(|value| rewrite_expr(value, local_functions, module_aliases))
                .collect::<Result<Vec<_>, _>>()?,
        )),
        Expr::Binary { lhs, op, rhs } => Ok(Expr::Binary {
            lhs: Box::new(rewrite_expr(*lhs, local_functions, module_aliases)?),
            op,
            rhs: Box::new(rewrite_expr(*rhs, local_functions, module_aliases)?),
        }),
    }
}

fn rewrite_callee(
    callee: Expr,
    local_functions: &HashMap<String, String>,
    module_aliases: &ModuleAliases,
) -> Result<Expr, CompileError> {
    match callee {
        Expr::Path(path) if path.len() == 1 => {
            let name = &path[0];
            Ok(Expr::Path(vec![
                local_functions.get(name).cloned().unwrap_or_else(|| name.clone()),
            ]))
        }
        Expr::Path(path) if path.len() == 2 => {
            let module = module_aliases
                .get(&path[0])
                .ok_or_else(|| CompileError::new(format!("unknown module `{}`", path[0])))?;
            let function = module.get(&path[1]).ok_or_else(|| {
                CompileError::new(format!(
                    "module `{}` has no function `{}`",
                    path[0], path[1]
                ))
            })?;
            Ok(Expr::Path(vec![function.clone()]))
        }
        other => rewrite_expr(other, local_functions, module_aliases),
    }
}

#[cfg(test)]
mod tests {
    use std::{
        fs,
        path::{Path, PathBuf},
        time::{SystemTime, UNIX_EPOCH},
    };

    use super::resolve_entry_program;

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
        fs::write(&module, "def some_function() void\n\t@puts(\"hello\")\nend\n").unwrap();

        let program = resolve_entry_program(&entry).unwrap();

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
}
