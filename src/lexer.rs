use crate::CompileError;

#[derive(Debug, Clone, PartialEq)]
pub struct Token {
    pub kind: TokenKind,
    pub line: usize,
    pub column: usize,
}

#[derive(Debug, Clone, PartialEq)]
pub enum TokenKind {
    Pub,
    Def,
    Type,
    End,
    Var,
    Val,
    Return,
    Parallel,
    For,
    To,
    Ref,
    Void,
    I32,
    U8,
    Ident(String),
    Int(i64),
    Str(String),
    Newline,
    At,
    LParen,
    RParen,
    LBrace,
    RBrace,
    Comma,
    Colon,
    Dot,
    Assign,
    Plus,
    PlusEqual,
    Eof,
}

pub fn lex(source: &str) -> Result<Vec<Token>, CompileError> {
    Lexer::new(source).lex()
}

struct Lexer {
    chars: Vec<char>,
    pos: usize,
    line: usize,
    column: usize,
}

impl Lexer {
    fn new(source: &str) -> Self {
        Self {
            chars: source.chars().collect(),
            pos: 0,
            line: 1,
            column: 1,
        }
    }

    fn lex(mut self) -> Result<Vec<Token>, CompileError> {
        let mut tokens = Vec::new();

        while let Some(ch) = self.peek() {
            match ch {
                ' ' | '\t' | '\r' => {
                    self.bump();
                }
                '\n' => {
                    let (line, column) = (self.line, self.column);
                    self.bump();
                    tokens.push(Token {
                        kind: TokenKind::Newline,
                        line,
                        column,
                    });
                }
                '#' if self.peek_next() == Some('#') => {
                    self.bump();
                    self.bump();
                    while let Some(current) = self.peek() {
                        if current == '\n' {
                            break;
                        }
                        self.bump();
                    }
                }
                '@' => tokens.push(self.single(TokenKind::At)),
                '(' => tokens.push(self.single(TokenKind::LParen)),
                ')' => tokens.push(self.single(TokenKind::RParen)),
                '{' => tokens.push(self.single(TokenKind::LBrace)),
                '}' => tokens.push(self.single(TokenKind::RBrace)),
                ',' => tokens.push(self.single(TokenKind::Comma)),
                ':' => tokens.push(self.single(TokenKind::Colon)),
                '.' => tokens.push(self.single(TokenKind::Dot)),
                '=' => tokens.push(self.single(TokenKind::Assign)),
                '+' => {
                    let line = self.line;
                    let column = self.column;
                    self.bump();
                    if self.peek() == Some('=') {
                        self.bump();
                        tokens.push(Token {
                            kind: TokenKind::PlusEqual,
                            line,
                            column,
                        });
                    } else {
                        tokens.push(Token {
                            kind: TokenKind::Plus,
                            line,
                            column,
                        });
                    }
                }
                '"' => tokens.push(self.string()?),
                '0'..='9' => tokens.push(self.number()?),
                _ if is_ident_start(ch) => tokens.push(self.ident_or_keyword()),
                _ => {
                    return Err(self.error(format!("unexpected character `{ch}`")));
                }
            }
        }

        tokens.push(Token {
            kind: TokenKind::Eof,
            line: self.line,
            column: self.column,
        });
        Ok(tokens)
    }

    fn single(&mut self, kind: TokenKind) -> Token {
        let token = Token {
            kind,
            line: self.line,
            column: self.column,
        };
        self.bump();
        token
    }

    fn string(&mut self) -> Result<Token, CompileError> {
        let line = self.line;
        let column = self.column;
        self.bump();

        let mut value = String::new();
        loop {
            let Some(ch) = self.peek() else {
                return Err(self.error("unterminated string literal"));
            };
            match ch {
                '"' => {
                    self.bump();
                    break;
                }
                '\\' => {
                    self.bump();
                    let Some(escaped) = self.peek() else {
                        return Err(self.error("unterminated escape sequence"));
                    };
                    let decoded = match escaped {
                        'n' => '\n',
                        't' => '\t',
                        'r' => '\r',
                        '"' => '"',
                        '\\' => '\\',
                        other => {
                            return Err(
                                self.error(format!("unsupported escape sequence `\\{other}`"))
                            );
                        }
                    };
                    self.bump();
                    value.push(decoded);
                }
                '\n' => return Err(self.error("newline in string literal")),
                other => {
                    self.bump();
                    value.push(other);
                }
            }
        }

        Ok(Token {
            kind: TokenKind::Str(value),
            line,
            column,
        })
    }

    fn number(&mut self) -> Result<Token, CompileError> {
        let line = self.line;
        let column = self.column;
        let start = self.pos;
        while matches!(self.peek(), Some('0'..='9')) {
            self.bump();
        }
        let lexeme: String = self.chars[start..self.pos].iter().collect();
        let value = lexeme.parse::<i64>().map_err(|error| {
            CompileError::new(format!(
                "invalid integer literal `{lexeme}` at {line}:{column}: {error}"
            ))
        })?;
        Ok(Token {
            kind: TokenKind::Int(value),
            line,
            column,
        })
    }

    fn ident_or_keyword(&mut self) -> Token {
        let line = self.line;
        let column = self.column;
        let start = self.pos;
        while matches!(self.peek(), Some(ch) if is_ident_continue(ch)) {
            self.bump();
        }
        let lexeme: String = self.chars[start..self.pos].iter().collect();
        let kind = match lexeme.as_str() {
            "pub" => TokenKind::Pub,
            "def" => TokenKind::Def,
            "type" => TokenKind::Type,
            "end" => TokenKind::End,
            "var" => TokenKind::Var,
            "val" => TokenKind::Val,
            "return" => TokenKind::Return,
            "parallel" => TokenKind::Parallel,
            "for" => TokenKind::For,
            "to" => TokenKind::To,
            "ref" => TokenKind::Ref,
            "void" => TokenKind::Void,
            "i32" => TokenKind::I32,
            "u8" => TokenKind::U8,
            _ => TokenKind::Ident(lexeme),
        };
        Token { kind, line, column }
    }

    fn peek(&self) -> Option<char> {
        self.chars.get(self.pos).copied()
    }

    fn peek_next(&self) -> Option<char> {
        self.chars.get(self.pos + 1).copied()
    }

    fn bump(&mut self) -> Option<char> {
        let ch = self.peek()?;
        self.pos += 1;
        if ch == '\n' {
            self.line += 1;
            self.column = 1;
        } else {
            self.column += 1;
        }
        Some(ch)
    }

    fn error(&self, message: impl Into<String>) -> CompileError {
        CompileError::new(format!(
            "{} at {}:{}",
            message.into(),
            self.line,
            self.column
        ))
    }
}

fn is_ident_start(ch: char) -> bool {
    ch == '_' || ch.is_ascii_alphabetic()
}

fn is_ident_continue(ch: char) -> bool {
    is_ident_start(ch) || ch.is_ascii_digit()
}
