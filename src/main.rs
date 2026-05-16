mod ast;
mod codegen;
mod lexer;
mod parser;
mod resolver;
mod sema;

use std::{
    env, fs,
    path::{Path, PathBuf},
    process::{Command, ExitCode},
};

use codegen::generate_c;
use resolver::resolve_entry_program;
use sema::analyze;

#[derive(Debug, Clone)]
pub struct CompileError {
    message: String,
}

impl CompileError {
    pub fn new(message: impl Into<String>) -> Self {
        Self {
            message: message.into(),
        }
    }
}

impl std::fmt::Display for CompileError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(&self.message)
    }
}

impl std::error::Error for CompileError {}

fn main() -> ExitCode {
    match run() {
        Ok(()) => ExitCode::SUCCESS,
        Err(error) => {
            eprintln!("scar: {error}");
            ExitCode::FAILURE
        }
    }
}

fn run() -> Result<(), CompileError> {
    let cli = Cli::parse(env::args().skip(1))?;
    let program = resolve_entry_program(&cli.input)?;
    let info = analyze(&program)?;
    let generated = generate_c(&program, &info)?;

    if cli.emit_c {
        write_output(&cli.output, &generated)?;
    } else {
        let c_path = temporary_c_path(&cli.input);
        let result = (|| {
            write_output(&c_path, &generated)?;
            compile_c_to_binary(&c_path, &cli.output, cli.optimize)
        })();
        let _ = fs::remove_file(&c_path);
        result?;
    }

    Ok(())
}

struct Cli {
    input: PathBuf,
    output: PathBuf,
    emit_c: bool,
    optimize: bool,
}

impl Cli {
    fn parse(args: impl Iterator<Item = String>) -> Result<Self, CompileError> {
        let mut input = None;
        let mut output = None;
        let mut pending_output = false;
        let mut emit_c = false;
        let mut optimize = false;

        for arg in args {
            if pending_output {
                output = Some(PathBuf::from(arg));
                pending_output = false;
                continue;
            }

            match arg.as_str() {
                "-o" | "--output" => pending_output = true,
                "--emit" => emit_c = true,
                "-opt" | "--opt" => optimize = true,
                _ if arg.starts_with('-') => {
                    return Err(CompileError::new(format!("unknown flag: {arg}")));
                }
                _ => {
                    if input.is_some() {
                        return Err(CompileError::new(
                            "expected a single input file and optional -o/--output path",
                        ));
                    }
                    input = Some(PathBuf::from(arg));
                }
            }
        }

        if pending_output {
            return Err(CompileError::new("expected a path after -o/--output"));
        }

        let input = input.ok_or_else(|| {
            CompileError::new("usage: scar <input.scar> [--emit] [-opt] [-o output]")
        })?;
        let output = output.unwrap_or_else(|| default_output_path(&input, emit_c));
        Ok(Self {
            input,
            output,
            emit_c,
            optimize,
        })
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum CompilerKind {
    Tcc,
    Clang,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
struct CompilerSpec {
    kind: CompilerKind,
    opt_flag: &'static str,
}

fn write_output(path: &Path, contents: &str) -> Result<(), CompileError> {
    fs::write(path, contents)
        .map_err(|error| CompileError::new(format!("failed to write {}: {error}", path.display())))
}

fn compile_c_to_binary(
    c_path: &Path,
    output_path: &Path,
    optimize: bool,
) -> Result<(), CompileError> {
    let compiler = select_compiler(optimize)?;
    let compiler_name = match compiler.kind {
        CompilerKind::Tcc => "tcc",
        CompilerKind::Clang => "clang",
    };

    let status = Command::new(compiler_name)
        .arg("-std=c99")
        .arg(compiler.opt_flag)
        .arg(c_path)
        .arg("-o")
        .arg(output_path)
        .status()
        .map_err(|error| {
            CompileError::new(format!(
                "failed to invoke {compiler_name} for {}: {error}",
                c_path.display()
            ))
        })?;

    if status.success() {
        Ok(())
    } else {
        Err(CompileError::new(format!(
            "{compiler_name} failed to compile {}",
            c_path.display()
        )))
    }
}

fn select_compiler(optimize: bool) -> Result<CompilerSpec, CompileError> {
    let has_tcc = command_exists("tcc");
    let has_clang = command_exists("clang");
    choose_compiler(has_tcc, has_clang, optimize)
}

fn choose_compiler(
    has_tcc: bool,
    has_clang: bool,
    optimize: bool,
) -> Result<CompilerSpec, CompileError> {
    if optimize {
        if has_clang {
            return Ok(CompilerSpec {
                kind: CompilerKind::Clang,
                opt_flag: "-O2",
            });
        }
        return Err(CompileError::new(
            "optimized builds require clang, but clang was not found on PATH",
        ));
    }

    if has_tcc {
        return Ok(CompilerSpec {
            kind: CompilerKind::Tcc,
            opt_flag: "-O0",
        });
    }

    if has_clang {
        return Ok(CompilerSpec {
            kind: CompilerKind::Clang,
            opt_flag: "-O0",
        });
    }

    Err(CompileError::new("tcc nor clang were found in PATH"))
}

fn command_exists(command: &str) -> bool {
    Command::new(command).arg("-v").output().is_ok()
}

fn default_output_path(input: &Path, emit_c: bool) -> PathBuf {
    if emit_c {
        return input.with_extension("c");
    }

    if cfg!(windows) {
        input.with_extension("exe")
    } else {
        input.with_extension("")
    }
}

fn temporary_c_path(input: &Path) -> PathBuf {
    let stem = input
        .file_stem()
        .and_then(|value| value.to_str())
        .unwrap_or("scar-output");
    env::temp_dir().join(format!("scar-{stem}-{}.c", std::process::id()))
}

#[cfg(test)]
mod tests {
    use super::{Cli, CompilerKind, choose_compiler, default_output_path};
    use std::path::Path;

    #[test]
    fn cli_defaults_to_binary_output() {
        let cli = Cli::parse(["main.scar".to_string()].into_iter()).unwrap();

        assert!(!cli.emit_c);
        assert_eq!(
            cli.output,
            default_output_path(Path::new("main.scar"), false)
        );
    }

    #[test]
    fn cli_emit_uses_c_output() {
        let cli = Cli::parse(["--emit".to_string(), "main.scar".to_string()].into_iter()).unwrap();

        assert!(cli.emit_c);
        assert_eq!(cli.output, Path::new("main.c"));
    }

    #[test]
    fn optimized_builds_require_clang() {
        let compiler = choose_compiler(true, true, true).unwrap();
        assert_eq!(compiler.kind, CompilerKind::Clang);
        assert_eq!(compiler.opt_flag, "-O2");
    }

    #[test]
    fn dev_builds_prefer_tcc() {
        let compiler = choose_compiler(true, true, false).unwrap();
        assert_eq!(compiler.kind, CompilerKind::Tcc);
        assert_eq!(compiler.opt_flag, "-O0");
    }

    #[test]
    fn dev_builds_fall_back_to_clang() {
        let compiler = choose_compiler(false, true, false).unwrap();
        assert_eq!(compiler.kind, CompilerKind::Clang);
        assert_eq!(compiler.opt_flag, "-O0");
    }
}
