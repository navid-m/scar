#[derive(Debug, Clone)]
pub struct Program {
    pub module_uses: Vec<ModuleUse>,
    pub type_defs: Vec<TypeDef>,
    pub functions: Vec<Function>,
}

#[derive(Debug, Clone)]
pub struct ModuleUse {
    pub name: String,
    pub path: String,
}

#[derive(Debug, Clone)]
pub struct TypeDef {
    pub name: String,
    pub is_extern: bool,
    pub extern_name: Option<String>,
    pub fields: Vec<FieldDef>,
}

#[derive(Debug, Clone)]
pub struct FieldDef {
    pub name: String,
    pub ty: Type,
}

#[derive(Debug, Clone)]
pub struct Function {
    pub is_pub: bool,
    pub name: String,
    pub extern_name: Option<String>,
    pub params: Vec<Param>,
    pub return_type: Type,
    pub body: Vec<Stmt>,
}

#[derive(Debug, Clone)]
pub struct Param {
    pub name: String,
    pub ty: Type,
}

#[derive(Debug, Clone, PartialEq, Eq, Hash)]
pub enum Type {
    Void,
    I32,
    U32,
    U8,
    Named(String),
    Ref(Box<Type>),
    List(Box<Type>),
}

#[derive(Debug, Clone)]
pub enum Stmt {
    VarDecl {
        line: usize,
        column: usize,
        mutable: bool,
        name: String,
        declared_type: Option<Type>,
        init: Expr,
    },
    Assign {
        line: usize,
        column: usize,
        target: Expr,
        value: Expr,
    },
    AddAssign {
        line: usize,
        column: usize,
        target: Expr,
        value: Expr,
    },
    Return {
        line: usize,
        column: usize,
        value: Option<Expr>,
    },
    If {
        line: usize,
        column: usize,
        condition: Expr,
        then_body: Vec<Stmt>,
        else_body: Vec<Stmt>,
    },
    Expr {
        line: usize,
        column: usize,
        expr: Expr,
    },
    ForRange {
        line: usize,
        column: usize,
        pragma: Option<String>,
        var_name: String,
        start: Expr,
        end: Expr,
        body: Vec<Stmt>,
    },
    ForEach {
        line: usize,
        column: usize,
        var_name: String,
        iterable: Expr,
        body: Vec<Stmt>,
    },
    Loop {
        line: usize,
        column: usize,
        body: Vec<Stmt>,
    },
    Continue {
        line: usize,
        column: usize,
    },
}

#[derive(Debug, Clone)]
pub enum Expr {
    Int(i64),
    String(String),
    Path(Vec<String>),
    ListLiteral(Vec<Expr>),
    Index {
        base: Box<Expr>,
        index: Box<Expr>,
    },
    FieldAccess {
        base: Box<Expr>,
        field: String,
    },
    StructInit {
        name: String,
        fields: Vec<FieldInit>,
    },
    BuiltinCall {
        name: String,
        args: Vec<Expr>,
    },
    MethodCall {
        receiver: Box<Expr>,
        method: String,
        args: Vec<Expr>,
    },
    Call {
        callee: Box<Expr>,
        args: Vec<Expr>,
    },
    Cast {
        expr: Box<Expr>,
        ty: Type,
    },
    Unary {
        op: UnaryOp,
        expr: Box<Expr>,
    },
    Pack(Vec<Expr>),
    Binary {
        lhs: Box<Expr>,
        op: BinaryOp,
        rhs: Box<Expr>,
    },
}

#[derive(Debug, Clone)]
pub struct FieldInit {
    pub name: String,
    pub value: Expr,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum BinaryOp {
    Add,
    LessThan,
    GreaterEqual,
    Equal,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum UnaryOp {
    Neg,
}

impl Expr {
    pub fn as_path(&self) -> Option<&[String]> {
        match self {
            Self::Path(path) => Some(path),
            _ => None,
        }
    }
}
