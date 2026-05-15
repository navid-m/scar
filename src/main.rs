mod ast;
mod codegen;
mod lexer;
mod parser;
mod sema;

use std::{env, fs, path::PathBuf, process::ExitCode};

use codegen::generate_c;
use lexer::lex;
use parser::parse_program;
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
    let source = fs::read_to_string(&cli.input).map_err(|error| {
        CompileError::new(format!("failed to read {}: {error}", cli.input.display()))
    })?;

    let tokens = lex(&source)?;
    let program = parse_program(tokens)?;
    let info = analyze(&program)?;
    let generated = generate_c(&program, &info)?;

    fs::write(&cli.output, generated).map_err(|error| {
        CompileError::new(format!("failed to write {}: {error}", cli.output.display()))
    })?;

    Ok(())
}

struct Cli {
    input: PathBuf,
    output: PathBuf,
}

impl Cli {
    fn parse(args: impl Iterator<Item = String>) -> Result<Self, CompileError> {
        let mut input = None;
        let mut output = None;
        let mut pending_output = false;

        for arg in args {
            if pending_output {
                output = Some(PathBuf::from(arg));
                pending_output = false;
                continue;
            }

            match arg.as_str() {
                "-o" | "--output" => pending_output = true,
                "--emit-c" => {}
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

        let input =
            input.ok_or_else(|| CompileError::new("usage: scar <input.scar> [-o output.c]"))?;
        let output = output.unwrap_or_else(|| input.with_extension("c"));
        Ok(Self { input, output })
    }
}
