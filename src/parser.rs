use crate::{
    CompileError,
    ast::{BinaryOp, Expr, Function, Param, Program, Stmt, Type},
    lexer::{Token, TokenKind},
};

pub fn parse_program(tokens: Vec<Token>) -> Result<Program, CompileError> {
    Parser::new(tokens).parse_program()
}

struct Parser {
    tokens: Vec<Token>,
    pos: usize,
}

impl Parser {
    fn new(tokens: Vec<Token>) -> Self {
        Self { tokens, pos: 0 }
    }

    fn parse_program(&mut self) -> Result<Program, CompileError> {
        let mut functions = Vec::new();
        self.consume_newlines();
        while !self.is_eof() {
            functions.push(self.parse_function()?);
            self.consume_newlines();
        }
        Ok(Program { functions })
    }

    fn parse_function(&mut self) -> Result<Function, CompileError> {
        let is_pub = if self.check_simple(&TokenKind::Pub) {
            self.advance();
            true
        } else {
            false
        };
        self.expect_simple(TokenKind::Def)?;
        let name = self.expect_ident()?;
        self.expect_simple(TokenKind::LParen)?;

        let mut params = Vec::new();
        if !self.check_simple(&TokenKind::RParen) {
            loop {
                let param_name = self.expect_ident()?;
                let ty = self.parse_type()?;
                params.push(Param {
                    name: param_name,
                    ty,
                });
                if self.check_simple(&TokenKind::Comma) {
                    self.advance();
                } else {
                    break;
                }
            }
        }
        self.expect_simple(TokenKind::RParen)?;

        let return_type = if self.starts_type() {
            self.parse_type()?
        } else {
            Type::Void
        };

        self.expect_newline("expected a newline after function signature")?;
        let body = self.parse_block()?;
        self.expect_simple(TokenKind::End)?;
        self.consume_newlines();

        Ok(Function {
            is_pub,
            name,
            params,
            return_type,
            body,
        })
    }

    fn parse_block(&mut self) -> Result<Vec<Stmt>, CompileError> {
        let mut body = Vec::new();
        self.consume_newlines();
        while !self.check_simple(&TokenKind::End) && !self.is_eof() {
            body.push(self.parse_stmt()?);
            self.consume_newlines();
        }
        Ok(body)
    }

    fn parse_stmt(&mut self) -> Result<Stmt, CompileError> {
        if self.check_simple(&TokenKind::Var) || self.check_simple(&TokenKind::Val) {
            let mutable = self.check_simple(&TokenKind::Var);
            self.advance();
            let name = self.expect_ident()?;
            self.expect_simple(TokenKind::Assign)?;
            let init = self.parse_expr()?;
            self.expect_stmt_terminator()?;
            return Ok(Stmt::VarDecl {
                mutable,
                name,
                init,
            });
        }

        if self.check_simple(&TokenKind::Return) {
            self.advance();
            let value = if self.at_stmt_end() {
                None
            } else {
                Some(self.parse_expr()?)
            };
            self.expect_stmt_terminator()?;
            return Ok(Stmt::Return(value));
        }

        if self.check_simple(&TokenKind::At) && self.check_next_simple(&TokenKind::LParen) {
            return self.parse_pragma_stmt();
        }

        if self.check_simple(&TokenKind::For) {
            return self.parse_for_stmt(None);
        }

        if self.check_simple(&TokenKind::Parallel) {
            self.advance();
            return self.parse_for_stmt(Some("omp parallel for".to_string()));
        }

        let expr = self.parse_expr()?;
        if self.check_simple(&TokenKind::Assign) {
            self.advance();
            let value = self.parse_expr()?;
            self.expect_stmt_terminator()?;
            return Ok(Stmt::Assign {
                target: expr,
                value,
            });
        }
        if self.check_simple(&TokenKind::PlusEqual) {
            self.advance();
            let value = self.parse_expr()?;
            self.expect_stmt_terminator()?;
            return Ok(Stmt::AddAssign {
                target: expr,
                value,
            });
        }

        self.expect_stmt_terminator()?;
        Ok(Stmt::Expr(expr))
    }

    fn parse_pragma_stmt(&mut self) -> Result<Stmt, CompileError> {
        self.expect_simple(TokenKind::At)?;
        self.expect_simple(TokenKind::LParen)?;
        let TokenKind::Str(pragma) = self.current().kind.clone() else {
            return Err(self.error_at_current("expected a string literal pragma directive"));
        };
        self.advance();
        self.expect_simple(TokenKind::RParen)?;
        self.expect_newline("expected a newline after pragma directive")?;

        match self.parse_stmt()? {
            Stmt::For {
                pragma: None,
                var_name,
                start,
                end,
                body,
            } => Ok(Stmt::For {
                pragma: Some(pragma),
                var_name,
                start,
                end,
                body,
            }),
            _ => Err(CompileError::new(
                "pragma directives currently apply only to `for` loops",
            )),
        }
    }

    fn parse_for_stmt(&mut self, pragma: Option<String>) -> Result<Stmt, CompileError> {
        self.expect_simple(TokenKind::For)?;
        self.expect_simple(TokenKind::Var)?;
        let var_name = self.expect_ident()?;
        self.expect_simple(TokenKind::Assign)?;
        let start = self.parse_expr()?;
        self.expect_simple(TokenKind::To)?;
        let end = self.parse_expr()?;
        self.expect_newline("expected a newline after for header")?;
        let body = self.parse_block()?;
        self.expect_simple(TokenKind::End)?;
        self.consume_newlines();
        Ok(Stmt::For {
            pragma,
            var_name,
            start,
            end,
            body,
        })
    }

    fn parse_expr(&mut self) -> Result<Expr, CompileError> {
        self.parse_additive()
    }

    fn parse_additive(&mut self) -> Result<Expr, CompileError> {
        let mut expr = self.parse_postfix()?;
        while self.check_simple(&TokenKind::Plus) {
            self.advance();
            let rhs = self.parse_postfix()?;
            expr = Expr::Binary {
                lhs: Box::new(expr),
                op: BinaryOp::Add,
                rhs: Box::new(rhs),
            };
        }
        Ok(expr)
    }

    fn parse_postfix(&mut self) -> Result<Expr, CompileError> {
        let mut expr = self.parse_primary()?;
        while self.check_simple(&TokenKind::LParen) {
            self.advance();
            let mut args = Vec::new();
            if !self.check_simple(&TokenKind::RParen) {
                loop {
                    args.push(self.parse_expr()?);
                    if self.check_simple(&TokenKind::Comma) {
                        self.advance();
                    } else {
                        break;
                    }
                }
            }
            self.expect_simple(TokenKind::RParen)?;
            expr = Expr::Call {
                callee: Box::new(expr),
                args,
            };
        }
        Ok(expr)
    }

    fn parse_primary(&mut self) -> Result<Expr, CompileError> {
        match self.current().kind.clone() {
            TokenKind::Int(value) => {
                self.advance();
                Ok(Expr::Int(value))
            }
            TokenKind::Str(value) => {
                self.advance();
                Ok(Expr::String(value))
            }
            TokenKind::At => self.parse_builtin_call(),
            TokenKind::Ident(_) => self.parse_path(),
            TokenKind::LParen => {
                self.advance();
                let expr = self.parse_expr()?;
                self.expect_simple(TokenKind::RParen)?;
                Ok(expr)
            }
            TokenKind::LBrace => self.parse_pack(),
            _ => Err(self.error_at_current("expected an expression")),
        }
    }

    fn parse_builtin_call(&mut self) -> Result<Expr, CompileError> {
        self.expect_simple(TokenKind::At)?;
        let name = self.expect_ident()?;
        self.expect_simple(TokenKind::LParen)?;

        let mut args = Vec::new();
        if !self.check_simple(&TokenKind::RParen) {
            loop {
                args.push(self.parse_expr()?);
                if self.check_simple(&TokenKind::Comma) {
                    self.advance();
                } else {
                    break;
                }
            }
        }

        self.expect_simple(TokenKind::RParen)?;
        Ok(Expr::BuiltinCall { name, args })
    }

    fn parse_path(&mut self) -> Result<Expr, CompileError> {
        let mut segments = vec![self.expect_ident()?];
        while self.check_simple(&TokenKind::Dot) {
            self.advance();
            segments.push(self.expect_ident()?);
        }
        Ok(Expr::Path(segments))
    }

    fn parse_pack(&mut self) -> Result<Expr, CompileError> {
        self.expect_simple(TokenKind::LBrace)?;
        let mut values = Vec::new();
        if !self.check_simple(&TokenKind::RBrace) {
            loop {
                values.push(self.parse_expr()?);
                if self.check_simple(&TokenKind::Comma) {
                    self.advance();
                } else {
                    break;
                }
            }
        }
        self.expect_simple(TokenKind::RBrace)?;
        Ok(Expr::Pack(values))
    }

    fn parse_type(&mut self) -> Result<Type, CompileError> {
        match self.current().kind.clone() {
            TokenKind::Void => {
                self.advance();
                Ok(Type::Void)
            }
            TokenKind::I32 => {
                self.advance();
                Ok(Type::I32)
            }
            TokenKind::U8 => {
                self.advance();
                Ok(Type::U8)
            }
            TokenKind::Ref => {
                self.advance();
                self.expect_simple(TokenKind::LParen)?;
                let inner = self.parse_type()?;
                self.expect_simple(TokenKind::RParen)?;
                Ok(Type::Ref(Box::new(inner)))
            }
            _ => Err(self.error_at_current("expected a type")),
        }
    }

    fn starts_type(&self) -> bool {
        matches!(
            self.current().kind,
            TokenKind::Void | TokenKind::I32 | TokenKind::U8 | TokenKind::Ref
        )
    }

    fn expect_ident(&mut self) -> Result<String, CompileError> {
        match self.current().kind.clone() {
            TokenKind::Ident(name) => {
                self.advance();
                Ok(name)
            }
            _ => Err(self.error_at_current("expected an identifier")),
        }
    }

    fn expect_simple(&mut self, expected: TokenKind) -> Result<(), CompileError> {
        if self.check_simple(&expected) {
            self.advance();
            Ok(())
        } else {
            Err(self.error_at_current(format!("expected {}", Self::describe(&expected))))
        }
    }

    fn expect_newline(&mut self, message: &str) -> Result<(), CompileError> {
        if self.check_simple(&TokenKind::Newline) {
            self.consume_newlines();
            Ok(())
        } else {
            Err(self.error_at_current(message))
        }
    }

    fn expect_stmt_terminator(&mut self) -> Result<(), CompileError> {
        if self.at_stmt_end() {
            self.consume_newlines();
            Ok(())
        } else {
            Err(self.error_at_current("expected a newline after the statement"))
        }
    }

    fn at_stmt_end(&self) -> bool {
        self.check_simple(&TokenKind::Newline)
            || self.check_simple(&TokenKind::End)
            || self.check_simple(&TokenKind::Eof)
    }

    fn consume_newlines(&mut self) {
        while self.check_simple(&TokenKind::Newline) {
            self.advance();
        }
    }

    fn check_simple(&self, expected: &TokenKind) -> bool {
        std::mem::discriminant(&self.current().kind) == std::mem::discriminant(expected)
    }

    fn check_next_simple(&self, expected: &TokenKind) -> bool {
        self.tokens
            .get(self.pos + 1)
            .is_some_and(|token| std::mem::discriminant(&token.kind) == std::mem::discriminant(expected))
    }

    fn current(&self) -> &Token {
        &self.tokens[self.pos]
    }

    fn advance(&mut self) {
        if !self.is_eof() {
            self.pos += 1;
        }
    }

    fn is_eof(&self) -> bool {
        matches!(self.current().kind, TokenKind::Eof)
    }

    fn error_at_current(&self, message: impl Into<String>) -> CompileError {
        let token = self.current();
        CompileError::new(format!(
            "{} at {}:{}",
            message.into(),
            token.line,
            token.column
        ))
    }

    fn describe(kind: &TokenKind) -> &'static str {
        match kind {
            TokenKind::Pub => "`pub`",
            TokenKind::Def => "`def`",
            TokenKind::End => "`end`",
            TokenKind::Var => "`var`",
            TokenKind::Val => "`val`",
            TokenKind::Return => "`return`",
            TokenKind::Parallel => "`parallel`",
            TokenKind::For => "`for`",
            TokenKind::To => "`to`",
            TokenKind::Ref => "`ref`",
            TokenKind::Void => "`void`",
            TokenKind::I32 => "`i32`",
            TokenKind::U8 => "`u8`",
            TokenKind::Newline => "a newline",
            TokenKind::At => "`@`",
            TokenKind::LParen => "`(`",
            TokenKind::RParen => "`)`",
            TokenKind::LBrace => "`{`",
            TokenKind::RBrace => "`}`",
            TokenKind::Comma => "`,`",
            TokenKind::Dot => "`.`",
            TokenKind::Assign => "`=`",
            TokenKind::Plus => "`+`",
            TokenKind::PlusEqual => "`+=`",
            TokenKind::Eof => "end of file",
            TokenKind::Ident(_) => "an identifier",
            TokenKind::Int(_) => "an integer",
            TokenKind::Str(_) => "a string",
        }
    }
}

#[cfg(test)]
mod tests {
    use super::parse_program;
    use crate::{
        ast::{Expr, Stmt},
        lexer::lex,
    };

    #[test]
    fn parses_pub_function() {
        let source = "pub def main() void\nend\n";
        let program = parse_program(lex(source).unwrap()).unwrap();

        assert!(program.functions[0].is_pub);
        assert_eq!(program.functions[0].name, "main");
    }

    #[test]
    fn parses_pragma_for_loop() {
        let source = "pub def main() void\n@(\"omp parallel for\")\nfor var i = 0 to 10\nend\nend\n";
        let program = parse_program(lex(source).unwrap()).unwrap();

        match &program.functions[0].body[0] {
            Stmt::For {
                pragma,
                var_name,
                start: _,
                end: _,
                body,
            } => {
                assert_eq!(pragma.as_deref(), Some("omp parallel for"));
                assert_eq!(var_name, "i");
                assert!(body.is_empty());
            }
            other => panic!("expected for loop, got {other:?}"),
        }
    }

    #[test]
    fn parses_at_builtin_calls() {
        let source = "pub def main() void\n@print(\"{d}\", {1})\nend\n";
        let program = parse_program(lex(source).unwrap()).unwrap();

        match &program.functions[0].body[0] {
            Stmt::Expr(Expr::BuiltinCall { name, args }) => {
                assert_eq!(name, "print");
                assert_eq!(args.len(), 2);
            }
            other => panic!("expected @builtin expression, got {other:?}"),
        }
    }
}
