use crate::{
    CompileError,
    ast::{
        BinaryOp, Expr, FieldDef, FieldInit, Function, ModuleUse, Param, Program, Stmt, Type,
        TypeDef, UnaryOp,
    },
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
                type_defs.push(self.parse_type_def(false)?);
            } else if self.check_simple(&TokenKind::Extern) && self.check_next_simple(&TokenKind::Type) {
                type_defs.push(self.parse_type_def(true)?);
            } else if self.check_simple(&TokenKind::Extern) {
                functions.push(self.parse_extern_function()?);
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

    fn parse_type_def(&mut self, is_extern: bool) -> Result<TypeDef, CompileError> {
        if is_extern {
            self.expect_simple(TokenKind::Extern)?;
        }
        self.expect_simple(TokenKind::Type)?;
        let name = self.expect_ident()?;
        if self.check_simple(&TokenKind::Assign) {
            if is_extern {
                return Err(self.error_at_current(
                    "extern types must be explicitly defined with fields",
                ));
            }
            self.advance();
            let alias = Some(self.parse_type()?);
            self.expect_stmt_terminator()?;
            return Ok(TypeDef {
                name,
                is_extern,
                alias,
                fields: Vec::new(),
            });
        }
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
        Ok(TypeDef {
            name,
            is_extern,
            alias: None,
            fields,
        })
    }

    fn parse_function(&mut self) -> Result<Function, CompileError> {
        let is_pub = if self.check_simple(&TokenKind::Pub) {
            self.advance();
            true
        } else {
            false
        };
        let (name, params, return_type) = self.parse_function_signature()?;
        self.expect_newline("expected a newline after function signature")?;
        let body = self.parse_block()?;
        self.expect_simple(TokenKind::End)?;
        self.consume_newlines();

        Ok(Function {
            is_pub,
            name,
            extern_name: None,
            params,
            return_type,
            body,
        })
    }

    fn parse_extern_function(&mut self) -> Result<Function, CompileError> {
        self.expect_simple(TokenKind::Extern)?;
        let (name, params, return_type) = self.parse_function_signature()?;
        self.expect_simple(TokenKind::Assign)?;
        let TokenKind::Str(extern_name) = self.current().kind.clone() else {
            return Err(self.error_at_current("expected a string literal extern symbol name"));
        };
        self.advance();
        self.expect_stmt_terminator()?;

        Ok(Function {
            is_pub: false,
            name,
            extern_name: Some(extern_name),
            params,
            return_type,
            body: Vec::new(),
        })
    }

    fn parse_function_signature(&mut self) -> Result<(String, Vec<Param>, Type), CompileError> {
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
        Ok((name, params, return_type))
    }

    fn parse_block(&mut self) -> Result<Vec<Stmt>, CompileError> {
        self.parse_block_until(&[TokenKind::End])
    }

    fn parse_block_until(&mut self, terminators: &[TokenKind]) -> Result<Vec<Stmt>, CompileError> {
        let mut body = Vec::new();
        self.consume_newlines();
        while !self.check_any_simple(terminators) && !self.is_eof() {
            body.push(self.parse_stmt()?);
            self.consume_newlines();
        }
        Ok(body)
    }

    fn parse_stmt(&mut self) -> Result<Stmt, CompileError> {
        let line = self.current().line;
        let column = self.current().column;

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
                line,
                column,
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
            return Ok(Stmt::Return {
                line,
                column,
                value,
            });
        }

        if self.check_simple(&TokenKind::If) {
            return self.parse_if_stmt();
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

        if self.check_simple(&TokenKind::Continue) {
            self.advance();
            self.expect_stmt_terminator()?;
            return Ok(Stmt::Continue { line, column });
        }

        let expr = self.parse_expr()?;
        if self.check_simple(&TokenKind::Assign) {
            self.advance();
            let value = self.parse_expr()?;
            self.expect_stmt_terminator()?;
            return Ok(Stmt::Assign {
                line,
                column,
                target: expr,
                value,
            });
        }
        if self.check_simple(&TokenKind::PlusEqual) {
            self.advance();
            let value = self.parse_expr()?;
            self.expect_stmt_terminator()?;
            return Ok(Stmt::AddAssign {
                line,
                column,
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
        Ok(Stmt::Expr { line, column, expr })
    }

    fn parse_if_stmt(&mut self) -> Result<Stmt, CompileError> {
        let line = self.current().line;
        let column = self.current().column;
        self.expect_simple(TokenKind::If)?;
        let condition = self.parse_expr()?;
        self.expect_newline("expected a newline after if condition")?;
        let then_body = self.parse_block_until(&[TokenKind::Else, TokenKind::End])?;
        let else_body = if self.check_simple(&TokenKind::Else) {
            self.advance();
            self.expect_newline("expected a newline after else")?;
            self.parse_block()?
        } else {
            Vec::new()
        };
        self.expect_simple(TokenKind::End)?;
        self.consume_newlines();
        Ok(Stmt::If {
            line,
            column,
            condition,
            then_body,
            else_body,
        })
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
                line,
                column,
                pragma: None,
                var_name,
                start,
                end,
                body,
            } => Ok(Stmt::ForRange {
                line,
                column,
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
        let line = self.current().line;
        let column = self.current().column;
        self.expect_simple(TokenKind::For)?;

        if self.check_simple(&TokenKind::Newline) {
            if pragma.is_some() {
                return Err(self.error_at_current(
                    "pragma directives currently apply only to range-based `for` loops",
                ));
            }
            self.consume_newlines();
            let body = self.parse_block()?;
            self.expect_simple(TokenKind::End)?;
            self.consume_newlines();
            return Ok(Stmt::Loop { line, column, body });
        }

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
                line,
                column,
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
            line,
            column,
            var_name,
            iterable,
            body,
        })
    }

    fn parse_expr(&mut self) -> Result<Expr, CompileError> {
        self.parse_cast()
    }

    fn parse_cast(&mut self) -> Result<Expr, CompileError> {
        let mut expr = self.parse_or()?;
        while self.check_simple(&TokenKind::As) {
            self.advance();
            let ty = self.parse_type()?;
            expr = Expr::Cast {
                expr: Box::new(expr),
                ty,
            };
        }
        Ok(expr)
    }

    fn parse_or(&mut self) -> Result<Expr, CompileError> {
        let mut expr = self.parse_xor()?;
        while self.check_simple(&TokenKind::Or) {
            self.advance();
            let rhs = self.parse_xor()?;
            expr = Expr::Binary {
                lhs: Box::new(expr),
                op: BinaryOp::Or,
                rhs: Box::new(rhs),
            };
        }
        Ok(expr)
    }

    fn parse_xor(&mut self) -> Result<Expr, CompileError> {
        let mut expr = self.parse_and()?;
        while self.check_simple(&TokenKind::Xor) {
            self.advance();
            let rhs = self.parse_and()?;
            expr = Expr::Binary {
                lhs: Box::new(expr),
                op: BinaryOp::Xor,
                rhs: Box::new(rhs),
            };
        }
        Ok(expr)
    }

    fn parse_and(&mut self) -> Result<Expr, CompileError> {
        let mut expr = self.parse_equality()?;
        while self.check_simple(&TokenKind::And) {
            self.advance();
            let rhs = self.parse_equality()?;
            expr = Expr::Binary {
                lhs: Box::new(expr),
                op: BinaryOp::And,
                rhs: Box::new(rhs),
            };
        }
        Ok(expr)
    }

    fn parse_equality(&mut self) -> Result<Expr, CompileError> {
        let mut expr = self.parse_comparison()?;
        while self.check_simple(&TokenKind::EqualEqual) {
            self.advance();
            let rhs = self.parse_comparison()?;
            expr = Expr::Binary {
                lhs: Box::new(expr),
                op: BinaryOp::Equal,
                rhs: Box::new(rhs),
            };
        }
        Ok(expr)
    }

    fn parse_comparison(&mut self) -> Result<Expr, CompileError> {
        let mut expr = self.parse_shift()?;
        loop {
            let op = if self.check_simple(&TokenKind::Less) {
                Some(BinaryOp::LessThan)
            } else if self.check_simple(&TokenKind::GreaterEqual) {
                Some(BinaryOp::GreaterEqual)
            } else {
                None
            };
            let Some(op) = op else {
                break;
            };
            self.advance();
            let rhs = self.parse_shift()?;
            expr = Expr::Binary {
                lhs: Box::new(expr),
                op,
                rhs: Box::new(rhs),
            };
        }
        Ok(expr)
    }

    fn parse_shift(&mut self) -> Result<Expr, CompileError> {
        let mut expr = self.parse_additive()?;
        loop {
            let op = if self.check_simple(&TokenKind::Shl) {
                Some(BinaryOp::ShiftLeft)
            } else if self.check_simple(&TokenKind::Shr) {
                Some(BinaryOp::ShiftRight)
            } else {
                None
            };
            let Some(op) = op else {
                break;
            };
            self.advance();
            let rhs = self.parse_additive()?;
            expr = Expr::Binary {
                lhs: Box::new(expr),
                op,
                rhs: Box::new(rhs),
            };
        }
        Ok(expr)
    }

    fn parse_additive(&mut self) -> Result<Expr, CompileError> {
        let mut expr = self.parse_unary()?;
        while self.check_simple(&TokenKind::Plus) {
            self.advance();
            let rhs = self.parse_unary()?;
            expr = Expr::Binary {
                lhs: Box::new(expr),
                op: BinaryOp::Add,
                rhs: Box::new(rhs),
            };
        }
        Ok(expr)
    }

    fn parse_unary(&mut self) -> Result<Expr, CompileError> {
        if self.check_simple(&TokenKind::Minus) {
            self.advance();
            let expr = self.parse_unary()?;
            return Ok(Expr::Unary {
                op: UnaryOp::Neg,
                expr: Box::new(expr),
            });
        }
        if self.check_simple(&TokenKind::Not) {
            self.advance();
            let expr = self.parse_unary()?;
            return Ok(Expr::Unary {
                op: UnaryOp::Not,
                expr: Box::new(expr),
            });
        }
        self.parse_postfix()
    }

    fn parse_postfix(&mut self) -> Result<Expr, CompileError> {
        let mut expr = self.parse_primary()?;
        loop {
            if self.check_simple(&TokenKind::LBracket) {
                self.advance();
                let index = self.parse_expr()?;
                self.expect_simple(TokenKind::RBracket)?;
                expr = Expr::Index {
                    base: Box::new(expr),
                    index: Box::new(index),
                };
                continue;
            }

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
        self.consume_newlines();
        let mut fields = Vec::new();
        while !self.check_simple(&TokenKind::RParen) {
            let field_name = self.expect_ident()?;
            self.expect_simple(TokenKind::Colon)?;
            let value = self.parse_expr()?;
            fields.push(FieldInit {
                name: field_name,
                value,
            });
            if self.check_simple(&TokenKind::Comma) {
                self.advance();
                self.consume_newlines();
            } else {
                self.consume_newlines();
                break;
            }
        }
        self.expect_simple(TokenKind::RParen)?;
        Ok(Expr::StructInit { name, fields })
    }

    fn parse_pack(&mut self) -> Result<Expr, CompileError> {
        self.expect_simple(TokenKind::LBrace)?;
        self.consume_newlines();
        let mut values = Vec::new();
        if !self.check_simple(&TokenKind::RBrace) {
            loop {
                values.push(self.parse_expr()?);
                if self.check_simple(&TokenKind::Comma) {
                    self.advance();
                    self.consume_newlines();
                } else {
                    self.consume_newlines();
                    break;
                }
            }
        }
        self.expect_simple(TokenKind::RBrace)?;
        Ok(Expr::Pack(values))
    }

    fn parse_list_literal(&mut self) -> Result<Expr, CompileError> {
        self.expect_simple(TokenKind::LBracket)?;
        self.consume_newlines();
        let mut values = Vec::new();
        if !self.check_simple(&TokenKind::RBracket) {
            loop {
                values.push(self.parse_expr()?);
                if self.check_simple(&TokenKind::Comma) {
                    self.advance();
                    self.consume_newlines();
                } else {
                    self.consume_newlines();
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
            TokenKind::U32 => {
                self.advance();
                Ok(Type::U32)
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
                | TokenKind::U32
                | TokenKind::U8
                | TokenKind::Ref
                | TokenKind::List
                | TokenKind::Ident(_)
        )
    }

    fn parse_call_args(&mut self) -> Result<Vec<Expr>, CompileError> {
        self.expect_simple(TokenKind::LParen)?;
        self.consume_newlines();
        let mut args = Vec::new();
        if !self.check_simple(&TokenKind::RParen) {
            loop {
                args.push(self.parse_expr()?);
                if self.check_simple(&TokenKind::Comma) {
                    self.advance();
                    self.consume_newlines();
                } else {
                    self.consume_newlines();
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
            Err(self.error_at_current(format!(
                "expected {}",
                Self::describe(&expected)
            )))
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
            || self.check_simple(&TokenKind::Else)
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
        self.tokens.get(self.pos + 1).is_some_and(|token| {
            std::mem::discriminant(&token.kind) == std::mem::discriminant(expected)
        })
    }

    fn check_any_simple(&self, expected: &[TokenKind]) -> bool {
        expected.iter().any(|kind| self.check_simple(kind))
    }

    fn looks_like_struct_init(&self) -> bool {
        self.check_simple(&TokenKind::LParen)
            && self
                .next_non_newline_kind(self.pos + 1)
                .is_some_and(|kind| matches!(kind, TokenKind::Ident(_)))
            && self
                .next_non_newline_kind_after_ident(self.pos + 1)
                .is_some_and(|kind| matches!(kind, TokenKind::Colon))
    }

    fn next_non_newline_kind(&self, start: usize) -> Option<&TokenKind> {
        self.tokens
            .iter()
            .skip(start)
            .find(|token| !matches!(token.kind, TokenKind::Newline))
            .map(|token| &token.kind)
    }

    fn next_non_newline_kind_after_ident(&self, start: usize) -> Option<&TokenKind> {
        let mut saw_ident = false;
        for token in self.tokens.iter().skip(start) {
            if matches!(token.kind, TokenKind::Newline) {
                continue;
            }
            if !saw_ident {
                if matches!(token.kind, TokenKind::Ident(_)) {
                    saw_ident = true;
                    continue;
                }
                return None;
            }
            return Some(&token.kind);
        }
        None
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
            TokenKind::Extern => "`extern`",
            TokenKind::Type => "`type`",
            TokenKind::End => "`end`",
            TokenKind::Var => "`var`",
            TokenKind::Val => "`val`",
            TokenKind::Return => "`return`",
            TokenKind::If => "`if`",
            TokenKind::Else => "`else`",
            TokenKind::Continue => "`continue`",
            TokenKind::Parallel => "`parallel`",
            TokenKind::For => "`for`",
            TokenKind::To => "`to`",
            TokenKind::In => "`in`",
            TokenKind::And => "`and`",
            TokenKind::Or => "`or`",
            TokenKind::Xor => "`xor`",
            TokenKind::Not => "`not`",
            TokenKind::Shl => "`shl`",
            TokenKind::Shr => "`shr`",
            TokenKind::As => "`as`",
            TokenKind::Ref => "`ref`",
            TokenKind::List => "`list`",
            TokenKind::Void => "`void`",
            TokenKind::I32 => "`i32`",
            TokenKind::U32 => "`u32`",
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
            TokenKind::EqualEqual => "`==`",
            TokenKind::Less => "`<`",
            TokenKind::GreaterEqual => "`>=`",
            TokenKind::Minus => "`-`",
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
        ast::{BinaryOp, Expr, Stmt, Type, UnaryOp},
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
    fn parses_extern_function() {
        let source = "extern def sleep(t u32) void = \"sleep\"\n";
        let program = parse_program(lex(source).unwrap()).unwrap();

        assert_eq!(program.functions.len(), 1);
        assert_eq!(program.functions[0].extern_name.as_deref(), Some("sleep"));
        assert_eq!(program.functions[0].params[0].ty, Type::U32);
        assert!(program.functions[0].body.is_empty());
    }

    #[test]
    fn parses_extern_type_alias_and_packed_type() {
        let source = "extern type Point\n\tx i32\n\ty i32\nend\ntype Count = i32\n";
        let program = parse_program(lex(source).unwrap()).unwrap();

        assert_eq!(program.type_defs.len(), 2);
        assert_eq!(program.type_defs[0].name, "Point");
        assert!(program.type_defs[0].is_extern);
        assert!(program.type_defs[0].alias.is_none());
        assert_eq!(program.type_defs[0].fields.len(), 2);

        assert_eq!(program.type_defs[1].name, "Count");
        assert!(!program.type_defs[1].is_extern);
        assert_eq!(program.type_defs[1].alias, Some(Type::I32));
        assert!(program.type_defs[1].fields.is_empty());
    }

    #[test]
    fn rejects_extern_type_aliases() {
        let source = "extern type Useconds = i32\n";
        let error = parse_program(lex(source).unwrap()).unwrap_err();

        assert!(error
            .message
            .contains("extern types must be explicitly defined with fields"));
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
    fn parses_top_level_single_hash_comments() {
        let source = "# comment\npub def main() void\n\t# inside function\nend\n";
        let program = parse_program(lex(source).unwrap()).unwrap();

        assert_eq!(program.functions.len(), 1);
        assert_eq!(program.functions[0].name, "main");
    }

    #[test]
    fn parses_type_defs_and_field_access() {
        let source = "type SomeType\n\tx i32\n\ty i32\nend\npub def main() void\n\tval st = SomeType(x: 10, y: 12)\n\t@print(\"{d}\", {st.x})\nend\n";
        let program = parse_program(lex(source).unwrap()).unwrap();

        assert_eq!(program.type_defs.len(), 1);
        assert_eq!(program.type_defs[0].name, "SomeType");
        assert!(!program.type_defs[0].is_extern);
        assert!(program.type_defs[0].alias.is_none());
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
                body,
                ..
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
                ..
            } => {
                assert_eq!(var_name, "value");
                assert!(matches!(iterable, Expr::Path(path) if path == &vec!["values".to_string()]));
                assert_eq!(body.len(), 1);
            }
            other => panic!("expected foreach loop, got {other:?}"),
        }

        match &program.functions[1].body[2] {
            Stmt::Expr {
                expr: Expr::Call { callee, args },
                ..
            } => {
                assert!(matches!(callee.as_ref(), Expr::Path(path) if path == &vec!["helper".to_string()]));
                assert!(args.is_empty());
            }
            other => panic!("expected bracketless call, got {other:?}"),
        }
    }

    #[test]
    fn parses_if_loop_index_and_cast() {
        let source = "pub def main() void\n\tvar rows list[list[i32]] = []\n\tfor\n\t\tif rows[0][1] == -1\n\t\t\tcontinue\n\t\telse\n\t\t\t@append(rows, [1, 2] as list[i32])\n\t\tend\n\tend\nend\n";
        let program = parse_program(lex(source).unwrap()).unwrap();

        assert!(matches!(program.functions[0].body[1], Stmt::Loop { .. }));
    }

    #[test]
    fn parses_bitwise_and_boolean_style_operators() {
        let source = "pub def main() void\n\tval mask = not (1 shl 2) and 7 or 8 xor 3\n\tif 1 == 1 and 2 == 2\n\t\t@print(\"{d}\", {mask})\n\tend\nend\n";
        let program = parse_program(lex(source).unwrap()).unwrap();

        match &program.functions[0].body[0] {
            Stmt::VarDecl { init, .. } => match init {
                Expr::Binary {
                    op: BinaryOp::Or,
                    lhs,
                    rhs,
                } => {
                    assert!(matches!(
                        rhs.as_ref(),
                        Expr::Binary {
                            op: BinaryOp::Xor,
                            ..
                        }
                    ));
                    assert!(matches!(
                        lhs.as_ref(),
                        Expr::Binary {
                            op: BinaryOp::And,
                            lhs,
                            ..
                        } if matches!(
                            lhs.as_ref(),
                            Expr::Unary {
                                op: UnaryOp::Not,
                                expr,
                            } if matches!(
                                expr.as_ref(),
                                Expr::Binary {
                                    op: BinaryOp::ShiftLeft,
                                    ..
                                }
                            )
                        )
                    ));
                }
                other => panic!("expected operator tree, got {other:?}"),
            },
            other => panic!("expected variable declaration, got {other:?}"),
        }

        match &program.functions[0].body[1] {
            Stmt::If { condition, .. } => assert!(matches!(
                condition,
                Expr::Binary {
                    op: BinaryOp::And,
                    lhs,
                    rhs,
                } if matches!(
                    lhs.as_ref(),
                    Expr::Binary {
                        op: BinaryOp::Equal,
                        ..
                    }
                ) && matches!(
                    rhs.as_ref(),
                    Expr::Binary {
                        op: BinaryOp::Equal,
                        ..
                    }
                )
            )),
            other => panic!("expected if statement, got {other:?}"),
        }
    }

    #[test]
    fn parses_at_builtin_calls() {
        let source = "pub def main() void\n@print(\"{d}\", {1})\nend\n";
        let program = parse_program(lex(source).unwrap()).unwrap();

        match &program.functions[0].body[0] {
            Stmt::Expr {
                expr: Expr::BuiltinCall { name, args },
                ..
            } => {
                assert_eq!(name, "print");
                assert_eq!(args.len(), 2);
            }
            other => panic!("expected @builtin expression, got {other:?}"),
        }
    }
}
