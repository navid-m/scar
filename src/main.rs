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
    location: Option<SourceLocation>,
}

impl CompileError {
    pub fn new(message: impl Into<String>) -> Self {
        Self {
            message: message.into(),
            location: None,
        }
    }

    pub fn with_location(mut self, line: usize, column: usize) -> Self {
        if self.location.is_none() {
            self.location = Some(SourceLocation { line, column });
        }
        self
    }

    pub fn message(&self) -> &str {
        &self.message
    }

    pub fn location(&self) -> Option<(usize, usize)> {
        self.location.map(|location| (location.line, location.column))
    }
}

impl std::fmt::Display for CompileError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(&self.message)
    }
}

impl std::error::Error for CompileError {}

fn main() -> ExitCode {
    let cli = match Cli::parse(env::args().skip(1)) {
        Ok(cli) => cli,
        Err(error) => {
            eprintln!("{}", format_compile_error(&error, None));
            return ExitCode::FAILURE;
        }
    };

    match run(&cli) {
        Ok(()) => ExitCode::SUCCESS,
        Err(error) => {
            eprintln!("{}", format_compile_error(&error, Some(&cli.input)));
            ExitCode::FAILURE
        }
    }
}

fn run(cli: &Cli) -> Result<(), CompileError> {
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

#[derive(Debug, Clone, Copy)]
struct SourceLocation {
    line: usize,
    column: usize,
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
        let output = match output {
            Some(output) => output,
            None => default_output_path(&input, emit_c)?,
        };
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
    if let Some(parent) = path.parent().filter(|parent| !parent.as_os_str().is_empty()) {
        fs::create_dir_all(parent).map_err(|error| {
            CompileError::new(format!(
                "failed to create output directory {}: {error}",
                parent.display()
            ))
        })?;
    }
    fs::write(path, contents)
        .map_err(|error| CompileError::new(format!("failed to write {}: {error}", path.display())))
}

fn compile_c_to_binary(
    c_path: &Path,
    output_path: &Path,
    optimize: bool,
) -> Result<(), CompileError> {
    if let Some(parent) = output_path.parent().filter(|parent| !parent.as_os_str().is_empty()) {
        fs::create_dir_all(parent).map_err(|error| {
            CompileError::new(format!(
                "failed to create output directory {}: {error}",
                parent.display()
            ))
        })?;
    }

    let compiler = select_compiler(optimize)?;
    let compiler_name = match compiler.kind {
        CompilerKind::Tcc => "tcc",
        CompilerKind::Clang => "clang",
    };

    let mut use_flag: &str = "-w";

    if compiler_name == "clang" {
        use_flag = "-Wno-everything";
    }

    let status = Command::new(compiler_name)
        .arg("-std=c99")
        .arg(use_flag)
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

fn default_output_path(input: &Path, emit_c: bool) -> Result<PathBuf, CompileError> {
    if emit_c {
        return Ok(input.with_extension("c"));
    }

    let stem = input
        .file_stem()
        .and_then(|value| value.to_str())
        .unwrap_or("scar-output");
    let mut output = env::current_dir()
        .map_err(|error| CompileError::new(format!("failed to get current directory: {error}")))?;
    output.push("scar-out");
    output.push(stem);
    if cfg!(windows) {
        output.set_extension("exe");
    } else {
        output.set_extension("");
    }
    Ok(output)
}

fn temporary_c_path(input: &Path) -> PathBuf {
    let stem = input
        .file_stem()
        .and_then(|value| value.to_str())
        .unwrap_or("scar-output");
    env::temp_dir().join(format!("scar-{stem}-{}.c", std::process::id()))
}

fn format_compile_error(error: &CompileError, default_path: Option<&Path>) -> String {
    let (path, line, column, message) = resolve_error_site(error, default_path);
    let Some(line) = line else {
        return format!("scar: {message}");
    };
    let Some(column) = column else {
        return format!("scar: {message}");
    };
    let Some(path) = path else {
        return format!("scar: {message} at {line}:{column}");
    };

    let Ok(source) = fs::read_to_string(&path) else {
        return format!("scar: {message}\n --> {}:{line}:{column}", path.display());
    };
    let Some(snippet) = source.lines().nth(line.saturating_sub(1)) else {
        return format!("scar: {message}\n --> {}:{line}:{column}", path.display());
    };

    let line_number = line.to_string();
    let gutter_width = line_number.len();
    let caret_padding = " ".repeat(column.saturating_sub(1));

    format!(
        "scar: {message}\n --> {}:{line}:{column}\n{} |\n{} | {}\n{} | {}^",
        path.display(),
        " ".repeat(gutter_width),
        line_number,
        snippet,
        " ".repeat(gutter_width),
        caret_padding
    )
}

fn resolve_error_site(
    error: &CompileError,
    default_path: Option<&Path>,
) -> (Option<PathBuf>, Option<usize>, Option<usize>, String) {
    if let Some((line, column)) = error.location() {
        return (
            default_path.map(Path::to_path_buf),
            Some(line),
            Some(column),
            error.message().to_string(),
        );
    }

    let message = error.message();
    let (path, message) = if let Some(rest) = message.strip_prefix("in ") {
        if let Some((path, tail)) = rest.split_once(": ") {
            (Some(PathBuf::from(path)), tail.to_string())
        } else {
            (default_path.map(Path::to_path_buf), message.to_string())
        }
    } else {
        (default_path.map(Path::to_path_buf), message.to_string())
    };

    if let Some((tail, line, column)) = parse_location_suffix(&message) {
        (path, Some(line), Some(column), tail.to_string())
    } else {
        (path, None, None, message)
    }
}

fn parse_location_suffix(message: &str) -> Option<(&str, usize, usize)> {
    let (tail, location) = message.rsplit_once(" at ")?;
    let (line, column) = location.split_once(':')?;
    let line = line.parse().ok()?;
    let column = column.parse().ok()?;
    Some((tail, line, column))
}

#[cfg(test)]
mod tests {
    use super::{Cli, CompilerKind, choose_compiler, default_output_path};
    use std::{env, path::Path};

    #[test]
    fn cli_defaults_to_binary_output() {
        let cli = Cli::parse(["main.scar".to_string()].into_iter()).unwrap();

        assert!(!cli.emit_c);
        assert_eq!(
            cli.output,
            default_output_path(Path::new("main.scar"), false).unwrap()
        );
    }

    #[test]
    fn cli_emit_uses_c_output() {
        let cli = Cli::parse(["--emit".to_string(), "main.scar".to_string()].into_iter()).unwrap();

        assert!(cli.emit_c);
        assert_eq!(cli.output, Path::new("main.c"));
    }

    #[test]
    fn default_binary_output_uses_invocation_directory() {
        let output = default_output_path(Path::new("nested/main.scar"), false).unwrap();
        let mut expected = env::current_dir().unwrap();
        expected.push("scar-out");
        expected.push(if cfg!(windows) { "main.exe" } else { "main" });

        assert_eq!(output, expected);
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
