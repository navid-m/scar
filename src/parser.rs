use crate::{
    CompileError,
    ast::{BinaryOp, Expr, FieldDef, FieldInit, Function, ModuleUse, Param, Program, Stmt, Type, TypeDef},
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
        let mut module_uses = Vec::new();
        let mut type_defs = Vec::new();
        let mut functions = Vec::new();
        self.consume_newlines();
        while !self.is_eof() {
            if self.check_simple(&TokenKind::Val) {
                module_uses.push(self.parse_module_use()?);
            } else if self.check_simple(&TokenKind::Type) {
                type_defs.push(self.parse_type_def()?);
            } else {
                functions.push(self.parse_function()?);
            }
            self.consume_newlines();
        }
        Ok(Program {
            module_uses,
            type_defs,
            functions,
        })
    }

    fn parse_module_use(&mut self) -> Result<ModuleUse, CompileError> {
        self.expect_simple(TokenKind::Val)?;
        let name = self.expect_ident()?;
        self.expect_simple(TokenKind::Assign)?;
        match self.current().kind.clone() {
            TokenKind::Ident(keyword) if keyword == "use" => self.advance(),
            _ => {
                return Err(self.error_at_current(
                    "expected `use(\"path\")` in top-level module binding",
                ));
            }
        }
        self.expect_simple(TokenKind::LParen)?;
        let TokenKind::Str(path) = self.current().kind.clone() else {
            return Err(self.error_at_current("expected a string literal module path"));
        };
        self.advance();
        self.expect_simple(TokenKind::RParen)?;
        self.expect_stmt_terminator()?;
        Ok(ModuleUse { name, path })
    }

    fn parse_type_def(&mut self) -> Result<TypeDef, CompileError> {
        self.expect_simple(TokenKind::Type)?;
        let name = self.expect_ident()?;
        self.expect_newline("expected a newline after type name")?;

        let mut fields = Vec::new();
        self.consume_newlines();
        while !self.check_simple(&TokenKind::End) && !self.is_eof() {
            let field_name = self.expect_ident()?;
            let ty = self.parse_type()?;
            self.expect_stmt_terminator()?;
            fields.push(FieldDef {
                name: field_name,
                ty,
            });
        }

        self.expect_simple(TokenKind::End)?;
        self.consume_newlines();
        Ok(TypeDef { name, fields })
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
            let declared_type = if self.check_simple(&TokenKind::Assign) {
                None
            } else {
                Some(self.parse_type()?)
            };
            self.expect_simple(TokenKind::Assign)?;
            let init = self.parse_expr()?;
            self.expect_stmt_terminator()?;
            return Ok(Stmt::VarDecl {
                mutable,
                name,
                declared_type,
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

        let expr = if self.at_stmt_end() {
            self.maybe_promote_bracketless_call(expr)
        } else {
            expr
        };
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
            Stmt::ForRange {
                pragma: None,
                var_name,
                start,
                end,
                body,
            } => Ok(Stmt::ForRange {
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
        if self.check_simple(&TokenKind::Assign) {
            self.advance();
            let start = self.parse_expr()?;
            self.expect_simple(TokenKind::To)?;
            let end = self.parse_expr()?;
            self.expect_newline("expected a newline after for header")?;
            let body = self.parse_block()?;
            self.expect_simple(TokenKind::End)?;
            self.consume_newlines();
            return Ok(Stmt::ForRange {
                pragma,
                var_name,
                start,
                end,
                body,
            });
        }

        if pragma.is_some() {
            return Err(self.error_at_current(
                "pragma directives currently apply only to range-based `for` loops",
            ));
        }

        self.expect_simple(TokenKind::In)?;
        let iterable = self.parse_expr()?;
        self.expect_newline("expected a newline after for header")?;
        let body = self.parse_block()?;
        self.expect_simple(TokenKind::End)?;
        self.consume_newlines();
        Ok(Stmt::ForEach {
            var_name,
            iterable,
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
        loop {
            if self.check_simple(&TokenKind::Dot) {
                self.advance();
                let field = self.expect_ident()?;
                expr = Expr::FieldAccess {
                    base: Box::new(expr),
                    field,
                };
                continue;
            }

            if self.check_simple(&TokenKind::At) {
                self.advance();
                let method = self.expect_ident()?;
                let args = self.parse_call_args()?;
                expr = Expr::MethodCall {
                    receiver: Box::new(expr),
                    method,
                    args,
                };
                continue;
            }

            if self.check_simple(&TokenKind::LParen) {
                if let Expr::Path(path) = &expr {
                    if path.len() == 1 && self.looks_like_struct_init() {
                        expr = self.parse_struct_init(path[0].clone())?;
                        continue;
                    }
                }

                let args = self.parse_call_args()?;
                expr = Expr::Call {
                    callee: Box::new(expr),
                    args,
                };
                continue;
            }

            break;
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
            TokenKind::LBracket => self.parse_list_literal(),
            TokenKind::At => self.parse_builtin_call(),
            TokenKind::Ident(_) => self.parse_name(),
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
        let args = self.parse_call_args()?;
        Ok(Expr::BuiltinCall { name, args })
    }

    fn parse_name(&mut self) -> Result<Expr, CompileError> {
        Ok(Expr::Path(vec![self.expect_ident()?]))
    }

    fn parse_struct_init(&mut self, name: String) -> Result<Expr, CompileError> {
        self.expect_simple(TokenKind::LParen)?;
        let mut fields = Vec::new();
        loop {
            let field_name = self.expect_ident()?;
            self.expect_simple(TokenKind::Colon)?;
            let value = self.parse_expr()?;
            fields.push(FieldInit {
                name: field_name,
                value,
            });
            if self.check_simple(&TokenKind::Comma) {
                self.advance();
            } else {
                break;
            }
        }
        self.expect_simple(TokenKind::RParen)?;
        Ok(Expr::StructInit { name, fields })
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

    fn parse_list_literal(&mut self) -> Result<Expr, CompileError> {
        self.expect_simple(TokenKind::LBracket)?;
        let mut values = Vec::new();
        if !self.check_simple(&TokenKind::RBracket) {
            loop {
                values.push(self.parse_expr()?);
                if self.check_simple(&TokenKind::Comma) {
                    self.advance();
                } else {
                    break;
                }
            }
        }
        self.expect_simple(TokenKind::RBracket)?;
        Ok(Expr::ListLiteral(values))
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
            TokenKind::Ident(name) => {
                self.advance();
                Ok(Type::Named(name))
            }
            TokenKind::Ref => {
                self.advance();
                self.expect_simple(TokenKind::LParen)?;
                let inner = self.parse_type()?;
                self.expect_simple(TokenKind::RParen)?;
                Ok(Type::Ref(Box::new(inner)))
            }
            TokenKind::List => {
                self.advance();
                self.expect_simple(TokenKind::LBracket)?;
                let inner = self.parse_type()?;
                self.expect_simple(TokenKind::RBracket)?;
                Ok(Type::List(Box::new(inner)))
            }
            _ => Err(self.error_at_current("expected a type")),
        }
    }

    fn starts_type(&self) -> bool {
        matches!(
            self.current().kind,
            TokenKind::Void
                | TokenKind::I32
                | TokenKind::U8
                | TokenKind::Ref
                | TokenKind::List
                | TokenKind::Ident(_)
        )
    }

    fn parse_call_args(&mut self) -> Result<Vec<Expr>, CompileError> {
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
        Ok(args)
    }

    fn maybe_promote_bracketless_call(&self, expr: Expr) -> Expr {
        match expr {
            Expr::Path(_) => Expr::Call {
                callee: Box::new(expr),
                args: Vec::new(),
            },
            other => other,
        }
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

    fn looks_like_struct_init(&self) -> bool {
        self.check_simple(&TokenKind::LParen)
            && self
                .tokens
                .get(self.pos + 1)
                .is_some_and(|token| matches!(token.kind, TokenKind::Ident(_)))
            && self
                .tokens
                .get(self.pos + 2)
                .is_some_and(|token| matches!(token.kind, TokenKind::Colon))
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
            TokenKind::Type => "`type`",
            TokenKind::End => "`end`",
            TokenKind::Var => "`var`",
            TokenKind::Val => "`val`",
            TokenKind::Return => "`return`",
            TokenKind::Parallel => "`parallel`",
            TokenKind::For => "`for`",
            TokenKind::To => "`to`",
            TokenKind::In => "`in`",
            TokenKind::Ref => "`ref`",
            TokenKind::List => "`list`",
            TokenKind::Void => "`void`",
            TokenKind::I32 => "`i32`",
            TokenKind::U8 => "`u8`",
            TokenKind::Newline => "a newline",
            TokenKind::At => "`@`",
            TokenKind::LParen => "`(`",
            TokenKind::RParen => "`)`",
            TokenKind::LBracket => "`[`",
            TokenKind::RBracket => "`]`",
            TokenKind::LBrace => "`{`",
            TokenKind::RBrace => "`}`",
            TokenKind::Comma => "`,`",
            TokenKind::Colon => "`:`",
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
        ast::{Expr, Stmt, Type},
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
    fn parses_top_level_module_use() {
        let source = "val some_module = use(\"some_file\")\npub def main() void\nend\n";
        let program = parse_program(lex(source).unwrap()).unwrap();

        assert_eq!(program.module_uses.len(), 1);
        assert_eq!(program.module_uses[0].name, "some_module");
        assert_eq!(program.module_uses[0].path, "some_file");
        assert!(program.type_defs.is_empty());
        assert_eq!(program.functions.len(), 1);
    }

    #[test]
    fn parses_type_defs_and_field_access() {
        let source = "type SomeType\n\tx i32\n\ty i32\nend\npub def main() void\n\tval st = SomeType(x: 10, y: 12)\n\t@print(\"{d}\", {st.x})\nend\n";
        let program = parse_program(lex(source).unwrap()).unwrap();

        assert_eq!(program.type_defs.len(), 1);
        assert_eq!(program.type_defs[0].name, "SomeType");
        assert_eq!(program.type_defs[0].fields.len(), 2);

        match &program.functions[0].body[0] {
            Stmt::VarDecl {
                init: Expr::StructInit { name, fields },
                ..
            } => {
                assert_eq!(name, "SomeType");
                assert_eq!(fields.len(), 2);
            }
            other => panic!("expected struct init, got {other:?}"),
        }
    }

    #[test]
    fn parses_pragma_for_loop() {
        let source = "pub def main() void\n@(\"omp parallel for\")\nfor var i = 0 to 10\nend\nend\n";
        let program = parse_program(lex(source).unwrap()).unwrap();

        match &program.functions[0].body[0] {
            Stmt::ForRange {
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
    fn parses_list_declaration_foreach_and_bracketless_call() {
        let source = "pub def helper() void\nend\npub def main() void\n\tval values list[i32] = [1, 2, 3]\n\tfor var value in values\n\t\t@print(\"{d}\", {value})\n\tend\n\thelper\nend\n";
        let program = parse_program(lex(source).unwrap()).unwrap();

        match &program.functions[1].body[0] {
            Stmt::VarDecl {
                declared_type: Some(Type::List(inner)),
                init: Expr::ListLiteral(values),
                ..
            } => {
                assert_eq!(**inner, Type::I32);
                assert_eq!(values.len(), 3);
            }
            other => panic!("expected typed list declaration, got {other:?}"),
        }

        match &program.functions[1].body[1] {
            Stmt::ForEach {
                var_name,
                iterable,
                body,
            } => {
                assert_eq!(var_name, "value");
                assert!(matches!(iterable, Expr::Path(path) if path == &vec!["values".to_string()]));
                assert_eq!(body.len(), 1);
            }
            other => panic!("expected foreach loop, got {other:?}"),
        }

        match &program.functions[1].body[2] {
            Stmt::Expr(Expr::Call { callee, args }) => {
                assert!(matches!(callee.as_ref(), Expr::Path(path) if path == &vec!["helper".to_string()]));
                assert!(args.is_empty());
            }
            other => panic!("expected bracketless call, got {other:?}"),
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
