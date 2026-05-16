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
    Extern,
    Type,
    End,
    Var,
    Val,
    Return,
    If,
    Else,
    Continue,
    Parallel,
    For,
    To,
    In,
    And,
    Or,
    Xor,
    Not,
    Shl,
    Shr,
    As,
    Ref,
    List,
    Void,
    I32,
    U32,
    U8,
    Ident(String),
    Int(i64),
    Str(String),
    Newline,
    At,
    LParen,
    RParen,
    LBracket,
    RBracket,
    LBrace,
    RBrace,
    Comma,
    Colon,
    Dot,
    Assign,
    EqualEqual,
    Less,
    GreaterEqual,
    Star,
    Minus,
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
                '#' => {
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
                '[' => tokens.push(self.single(TokenKind::LBracket)),
                ']' => tokens.push(self.single(TokenKind::RBracket)),
                '{' => tokens.push(self.single(TokenKind::LBrace)),
                '}' => tokens.push(self.single(TokenKind::RBrace)),
                ',' => tokens.push(self.single(TokenKind::Comma)),
                ':' => tokens.push(self.single(TokenKind::Colon)),
                '.' => tokens.push(self.single(TokenKind::Dot)),
                '=' => {
                    let line = self.line;
                    let column = self.column;
                    self.bump();
                    if self.peek() == Some('=') {
                        self.bump();
                        tokens.push(Token {
                            kind: TokenKind::EqualEqual,
                            line,
                            column,
                        });
                    } else {
                        tokens.push(Token {
                            kind: TokenKind::Assign,
                            line,
                            column,
                        });
                    }
                }
                '<' => tokens.push(self.single(TokenKind::Less)),
                '>' => {
                    let line = self.line;
                    let column = self.column;
                    self.bump();
                    if self.peek() == Some('=') {
                        self.bump();
                        tokens.push(Token {
                            kind: TokenKind::GreaterEqual,
                            line,
                            column,
                        });
                    } else {
                        return Err(self.error("unexpected character `>`"));
                    }
                }
                '*' => tokens.push(self.single(TokenKind::Star)),
                '-' => tokens.push(self.single(TokenKind::Minus)),
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
                        'x' => {
                            self.bump();
                            let Some(high) = self.peek() else {
                                return Err(self.error("unterminated hex escape sequence"));
                            };
                            self.bump();
                            let Some(low) = self.peek() else {
                                return Err(self.error("unterminated hex escape sequence"));
                            };
                            let value = hex_value(high)
                                .zip(hex_value(low))
                                .map(|(high, low)| (high << 4) | low)
                                .ok_or_else(|| self.error("invalid hex escape sequence"))?;
                            self.bump();
                            value as char
                        }
                        other => {
                            return Err(
                                self.error(format!("unsupported escape sequence `\\{other}`"))
                            );
                        }
                    };
                    if escaped != 'x' {
                        self.bump();
                    }
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
            "extern" => TokenKind::Extern,
            "type" => TokenKind::Type,
            "end" => TokenKind::End,
            "var" => TokenKind::Var,
            "val" => TokenKind::Val,
            "return" => TokenKind::Return,
            "if" => TokenKind::If,
            "else" => TokenKind::Else,
            "continue" => TokenKind::Continue,
            "parallel" => TokenKind::Parallel,
            "for" => TokenKind::For,
            "to" => TokenKind::To,
            "in" => TokenKind::In,
            "and" => TokenKind::And,
            "or" => TokenKind::Or,
            "xor" => TokenKind::Xor,
            "not" => TokenKind::Not,
            "shl" => TokenKind::Shl,
            "shr" => TokenKind::Shr,
            "as" => TokenKind::As,
            "ref" => TokenKind::Ref,
            "list" => TokenKind::List,
            "void" => TokenKind::Void,
            "i32" => TokenKind::I32,
            "u32" => TokenKind::U32,
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

fn hex_value(ch: char) -> Option<u8> {
    match ch {
        '0'..='9' => Some((ch as u8) - b'0'),
        'a'..='f' => Some((ch as u8) - b'a' + 10),
        'A'..='F' => Some((ch as u8) - b'A' + 10),
        _ => None,
    }
}

#[cfg(test)]
mod tests {
    use super::{TokenKind, lex};

    #[test]
    fn lexes_single_hash_comments() {
        let tokens = lex("# headline\nval value = 1 # trailing\n").unwrap();

        assert!(matches!(tokens[0].kind, TokenKind::Newline));
        assert!(matches!(tokens[1].kind, TokenKind::Val));
        assert!(matches!(tokens[2].kind, TokenKind::Ident(ref name) if name == "value"));
        assert!(matches!(tokens[3].kind, TokenKind::Assign));
        assert!(matches!(tokens[4].kind, TokenKind::Int(1)));
        assert!(matches!(tokens[5].kind, TokenKind::Newline));
    }

    #[test]
    fn still_accepts_double_hash_comments() {
        let tokens = lex("## docs\npub def main() void\nend\n").unwrap();

        assert!(matches!(tokens[0].kind, TokenKind::Newline));
        assert!(matches!(tokens[1].kind, TokenKind::Pub));
    }
}
