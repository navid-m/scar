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
    pub params: Vec<Param>,
    pub return_type: Type,
    pub body: Vec<Stmt>,
}

#[derive(Debug, Clone)]
pub struct Param {
    pub name: String,
    pub ty: Type,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Type {
    Void,
    I32,
    U8,
    Named(String),
    Ref(Box<Type>),
}

#[derive(Debug, Clone)]
pub enum Stmt {
    VarDecl {
        mutable: bool,
        name: String,
        init: Expr,
    },
    Assign {
        target: Expr,
        value: Expr,
    },
    AddAssign {
        target: Expr,
        value: Expr,
    },
    Return(Option<Expr>),
    Expr(Expr),
    For {
        pragma: Option<String>,
        var_name: String,
        start: Expr,
        end: Expr,
        body: Vec<Stmt>,
    },
}

#[derive(Debug, Clone)]
pub enum Expr {
    Int(i64),
    String(String),
    Path(Vec<String>),
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
    Call {
        callee: Box<Expr>,
        args: Vec<Expr>,
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
}

impl Expr {
    pub fn as_path(&self) -> Option<&[String]> {
        match self {
            Self::Path(path) => Some(path),
            _ => None,
        }
    }
}
