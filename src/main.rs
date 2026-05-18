mod ast;
mod codegen;
mod lexer;
mod parser;
mod resolver;
mod sema;

use std::{
    collections::hash_map::DefaultHasher,
    env, fs,
    hash::{Hash, Hasher},
    path::{Path, PathBuf},
    process::{Command, ExitCode},
};

use ast::{Expr, Function, Program, Stmt, Type};
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
        self.location
            .map(|location| (location.line, location.column))
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
            eprintln!("{}", format_compile_error(&error, cli.default_error_path()));
            ExitCode::FAILURE
        }
    }
}

fn run(cli: &Cli) -> Result<(), CompileError> {
    match cli {
        Cli::Build(cli) => run_build(cli),
        Cli::Run(cli) => run_run_command(cli),
        Cli::Test(cli) => run_test_command(cli),
    }
}

#[derive(Debug, Clone, Copy)]
struct SourceLocation {
    line: usize,
    column: usize,
}

enum Cli {
    Build(BuildCli),
    Run(RunCli),
    Test(TestCli),
}

impl Cli {
    fn parse(args: impl Iterator<Item = String>) -> Result<Self, CompileError> {
        let args: Vec<String> = args.collect();
        if args.first().is_some_and(|arg| arg == "run") {
            Ok(Self::Run(RunCli::parse(args.into_iter().skip(1))?))
        } else if args.first().is_some_and(|arg| arg == "test") {
            Ok(Self::Test(TestCli::parse(args.into_iter().skip(1))?))
        } else {
            Ok(Self::Build(BuildCli::parse(args.into_iter())?))
        }
    }

    fn default_error_path(&self) -> Option<&Path> {
        match self {
            Cli::Build(cli) => Some(&cli.input),
            Cli::Run(cli) => Some(&cli.input),
            Cli::Test(cli) => Some(&cli.target),
        }
    }
}

struct BuildCli {
    input: PathBuf,
    output: PathBuf,
    emit_c: bool,
    optimize: bool,
}

impl BuildCli {
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

struct RunCli {
    input: PathBuf,
}

impl RunCli {
    fn parse(args: impl Iterator<Item = String>) -> Result<Self, CompileError> {
        let mut input = None;

        for arg in args {
            if arg.starts_with('-') {
                return Err(CompileError::new(format!("unknown flag: {arg}")));
            }

            if input.is_some() {
                return Err(CompileError::new(
                    "expected a single input file after `scar run`",
                ));
            }

            input = Some(PathBuf::from(arg));
        }

        Ok(Self {
            input: input
                .ok_or_else(|| CompileError::new("usage: scar run <input.scar>"))?,
        })
    }
}

struct TestCli {
    target: PathBuf,
    output: Option<PathBuf>,
    emit_c: bool,
    optimize: bool,
}

impl TestCli {
    fn parse(args: impl Iterator<Item = String>) -> Result<Self, CompileError> {
        let mut target = None;
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
                    if target.is_some() {
                        return Err(CompileError::new(
                            "expected a single file or directory after `scar test`",
                        ));
                    }
                    target = Some(PathBuf::from(arg));
                }
            }
        }

        if pending_output {
            return Err(CompileError::new("expected a path after -o/--output"));
        }

        Ok(Self {
            target: target.ok_or_else(|| {
                CompileError::new(
                    "usage: scar test <file.scar|directory> [--emit] [-opt] [-o output]",
                )
            })?,
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
enum CompileMode {
    Standard,
    FastRun,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
struct CompilerSpec {
    kind: CompilerKind,
    opt_flag: &'static str,
}

fn write_output(path: &Path, contents: &str) -> Result<(), CompileError> {
    if let Some(parent) = path
        .parent()
        .filter(|parent| !parent.as_os_str().is_empty())
    {
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

fn run_build(cli: &BuildCli) -> Result<(), CompileError> {
    let program = resolve_entry_program(&cli.input)?;
    let info = analyze(&program)?;
    emit_program(
        &program,
        &info,
        &cli.input,
        &cli.output,
        cli.emit_c,
        cli.optimize,
    )
}

fn run_run_command(cli: &RunCli) -> Result<(), CompileError> {
    let program = resolve_entry_program(&cli.input)?;
    let info = analyze(&program)?;
    let generated = generate_c(&program, &info, true)?;
    let c_path = temporary_c_path(&cli.input);
    let binary_path = temporary_run_binary_path(&cli.input);

    let result = (|| {
        write_output(&c_path, &generated)?;
        compile_c_to_binary(&c_path, &binary_path, false, CompileMode::FastRun)?;
        run_program_binary(&binary_path)
    })();

    let _ = fs::remove_file(&c_path);
    let _ = fs::remove_file(&binary_path);
    result
}

fn emit_program(
    program: &Program,
    info: &sema::ProgramInfo,
    input: &Path,
    output: &Path,
    emit_c: bool,
    optimize: bool,
) -> Result<(), CompileError> {
    let generated = generate_c(program, info, !optimize)?;
    if emit_c {
        write_output(output, &generated)?;
    } else {
        let c_path = temporary_c_path(input);
        let result = (|| {
            write_output(&c_path, &generated)?;
            compile_c_to_binary(&c_path, output, optimize, CompileMode::Standard)
        })();
        let _ = fs::remove_file(&c_path);
        result?;
    }
    Ok(())
}

fn run_test_command(cli: &TestCli) -> Result<(), CompileError> {
    let files = collect_test_files(&cli.target)?;
    if files.is_empty() {
        return Err(CompileError::new(format!(
            "no .scar files found at {}",
            cli.target.display()
        )));
    }
    if cli.output.is_some() && files.len() > 1 {
        return Err(CompileError::new(
            "`scar test -o/--output` only supports a single input file",
        ));
    }

    let mut total_tests = 0usize;
    let mut files_with_tests = 0usize;
    for file in files {
        let count = run_tests_in_file(&file, cli)?;
        if count > 0 {
            files_with_tests += 1;
        }
        total_tests += count;
    }

    if cli.emit_c {
        println!(
            "scar: emitted test C for {} file(s) covering {} tests",
            files_with_tests, total_tests
        );
    } else {
        println!(
            "scar: {} tests passed across {} file(s)",
            total_tests, files_with_tests
        );
    }
    Ok(())
}

fn run_tests_in_file(path: &Path, cli: &TestCli) -> Result<usize, CompileError> {
    let program = resolve_entry_program(path)?;
    let test_count = program.tests.len();
    if test_count == 0 {
        println!("scar: {} (0 tests)", path.display());
        return Ok(0);
    }

    let runner = build_test_program(&program);
    let info = analyze(&runner)?;
    let generated = generate_c(&runner, &info, !cli.optimize)?;

    if cli.emit_c {
        let output = cli
            .output
            .clone()
            .unwrap_or_else(|| default_test_output_path(path));
        write_output(&output, &generated)?;
        println!("scar: emitted {} ({} tests)", output.display(), test_count);
        return Ok(test_count);
    }

    let c_path = temporary_test_c_path(path);
    let binary_path = temporary_test_binary_path(path);

    let result = (|| {
        write_output(&c_path, &generated)?;
        compile_c_to_binary(&c_path, &binary_path, cli.optimize, CompileMode::Standard)?;
        run_test_binary(&binary_path)?;
        Ok(())
    })();

    let _ = fs::remove_file(&c_path);
    let _ = fs::remove_file(&binary_path);
    result?;

    println!("scar: {} ({} tests)", path.display(), test_count);
    Ok(test_count)
}

fn build_test_program(program: &Program) -> Program {
    let mut functions: Vec<Function> = program
        .functions
        .iter()
        .filter(|function| function.name != "main")
        .cloned()
        .collect();
    let mut main_body = Vec::new();

    for (index, test) in program.tests.iter().enumerate() {
        let fn_name = format!("__scar_test_case_{index}");
        functions.push(Function {
            is_pub: false,
            name: fn_name.clone(),
            extern_name: None,
            generic_params: Vec::new(),
            params: Vec::new(),
            return_type: Type::Void,
            body: test.body.clone(),
        });
        main_body.push(Stmt::Expr {
            line: 0,
            column: 0,
            expr: Expr::Call {
                callee: Box::new(Expr::Path(vec![fn_name])),
                args: Vec::new(),
            },
        });
        main_body.push(Stmt::Expr {
            line: 0,
            column: 0,
            expr: Expr::BuiltinCall {
                name: "puts".to_string(),
                args: vec![Expr::String(format!("[pass] {}", test.name))],
            },
        });
    }

    functions.push(Function {
        is_pub: true,
        name: "main".to_string(),
        extern_name: None,
        generic_params: Vec::new(),
        params: Vec::new(),
        return_type: Type::Void,
        body: main_body,
    });

    Program {
        module_uses: Vec::new(),
        interface_defs: program.interface_defs.clone(),
        type_defs: program.type_defs.clone(),
        functions,
        tests: Vec::new(),
    }
}

fn run_test_binary(path: &Path) -> Result<(), CompileError> {
    let status = Command::new(path).status().map_err(|error| {
        CompileError::new(format!(
            "failed to run test binary {}: {error}",
            path.display()
        ))
    })?;
    if status.success() {
        Ok(())
    } else {
        Err(CompileError::new(format!(
            "tests failed in {}",
            path.display()
        )))
    }
}

fn run_program_binary(path: &Path) -> Result<(), CompileError> {
    let status = Command::new(path).status().map_err(|error| {
        CompileError::new(format!(
            "failed to run program binary {}: {error}",
            path.display()
        ))
    })?;
    if status.success() {
        Ok(())
    } else {
        Err(CompileError::new(format!(
            "program exited unsuccessfully: {}",
            path.display()
        )))
    }
}

fn collect_test_files(target: &Path) -> Result<Vec<PathBuf>, CompileError> {
    if target.is_file() {
        return Ok(vec![target.to_path_buf()]);
    }
    if !target.is_dir() {
        return Err(CompileError::new(format!(
            "{} is neither a file nor a directory",
            target.display()
        )));
    }

    let mut files = Vec::new();
    collect_test_files_recursive(target, &mut files)?;
    files.sort();
    Ok(files)
}

fn collect_test_files_recursive(dir: &Path, files: &mut Vec<PathBuf>) -> Result<(), CompileError> {
    for entry in fs::read_dir(dir)
        .map_err(|error| CompileError::new(format!("failed to read {}: {error}", dir.display())))?
    {
        let entry = entry.map_err(|error| {
            CompileError::new(format!("failed to read {}: {error}", dir.display()))
        })?;
        let path = entry.path();
        if path.is_dir() {
            collect_test_files_recursive(&path, files)?;
        } else if path.extension().and_then(|ext| ext.to_str()) == Some("scar") {
            files.push(path);
        }
    }
    Ok(())
}

fn compile_c_to_binary(
    c_path: &Path,
    output_path: &Path,
    optimize: bool,
    mode: CompileMode,
) -> Result<(), CompileError> {
    if let Some(parent) = output_path
        .parent()
        .filter(|parent| !parent.as_os_str().is_empty())
    {
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
        .args(extra_compiler_flags(compiler.kind, mode))
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
            "{compiler_name} failed to link {}",
            c_path.display()
        )))
    }
}

fn extra_compiler_flags(kind: CompilerKind, mode: CompileMode) -> &'static [&'static str] {
    match (kind, mode) {
        (CompilerKind::Clang, CompileMode::FastRun) => &["-pipe"],
        _ => &[],
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

fn temporary_test_c_path(input: &Path) -> PathBuf {
    env::temp_dir().join(format!("scar-test-{}.c", stable_path_hash(input)))
}

fn temporary_test_binary_path(input: &Path) -> PathBuf {
    let mut path = env::temp_dir().join(format!("scar-test-{}", stable_path_hash(input)));
    if cfg!(windows) {
        path.set_extension("exe");
    }
    path
}

fn temporary_run_binary_path(input: &Path) -> PathBuf {
    let stem = input
        .file_stem()
        .and_then(|value| value.to_str())
        .unwrap_or("scar-run");
    let mut path = env::temp_dir().join(format!("scar-run-{stem}-{}", std::process::id()));
    if cfg!(windows) {
        path.set_extension("exe");
    }
    path
}

fn default_test_output_path(input: &Path) -> PathBuf {
    let parent = input.parent().unwrap_or_else(|| Path::new(""));
    let stem = input
        .file_stem()
        .and_then(|value| value.to_str())
        .unwrap_or("scar-test");
    parent.join(format!("{stem}.test.c"))
}

fn stable_path_hash(path: &Path) -> u64 {
    let mut hasher = DefaultHasher::new();
    path.hash(&mut hasher);
    hasher.finish()
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
    use super::{
        Cli, CompileMode, CompilerKind, RunCli, TestCli, choose_compiler, default_output_path,
        extra_compiler_flags,
    };
    use std::{env, path::Path};

    #[test]
    fn cli_defaults_to_binary_output() {
        let cli = Cli::parse(["main.scar".to_string()].into_iter()).unwrap();
        let Cli::Build(cli) = cli else {
            panic!("expected build cli");
        };

        assert!(!cli.emit_c);
        assert_eq!(
            cli.output,
            default_output_path(Path::new("main.scar"), false).unwrap()
        );
    }

    #[test]
    fn cli_emit_uses_c_output() {
        let cli = Cli::parse(["--emit".to_string(), "main.scar".to_string()].into_iter()).unwrap();
        let Cli::Build(cli) = cli else {
            panic!("expected build cli");
        };

        assert!(cli.emit_c);
        assert_eq!(cli.output, Path::new("main.c"));
    }

    #[test]
    fn cli_parses_test_subcommand() {
        let cli = Cli::parse(["test".to_string(), ".".to_string()].into_iter()).unwrap();
        let Cli::Test(TestCli {
            target,
            output,
            emit_c,
            optimize,
        }) = cli
        else {
            panic!("expected test cli");
        };

        assert_eq!(target, Path::new("."));
        assert!(output.is_none());
        assert!(!emit_c);
        assert!(!optimize);
    }

    #[test]
    fn cli_parses_run_subcommand() {
        let cli = Cli::parse(["run".to_string(), "main.scar".to_string()].into_iter()).unwrap();
        let Cli::Run(RunCli { input }) = cli else {
            panic!("expected run cli");
        };

        assert_eq!(input, Path::new("main.scar"));
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

    #[test]
    fn fast_run_adds_pipe_for_clang() {
        assert_eq!(
            extra_compiler_flags(CompilerKind::Clang, CompileMode::FastRun),
            ["-pipe"]
        );
        assert!(extra_compiler_flags(CompilerKind::Tcc, CompileMode::FastRun).is_empty());
        assert!(extra_compiler_flags(CompilerKind::Clang, CompileMode::Standard).is_empty());
    }
}
