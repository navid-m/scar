#[derive(Debug, Clone)]
pub struct Program {
    pub module_uses: Vec<ModuleUse>,
    pub extern_headers: Vec<String>,
    pub interface_defs: Vec<InterfaceDef>,
    pub type_defs: Vec<TypeDef>,
    pub functions: Vec<Function>,
    pub tests: Vec<TestBlock>,
}

#[derive(Debug, Clone)]
pub struct ModuleUse {
    pub name: String,
    pub path: String,
}

#[derive(Debug, Clone)]
pub struct TypeDef {
    pub is_pub: bool,
    pub name: String,
    pub kind: TypeDefKind,
    pub is_extern: bool,
    pub alias: Option<Type>,
    pub derives: Vec<Type>,
    pub fields: Vec<FieldDef>,
    pub variants: Vec<UnionVariantDef>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum TypeDefKind {
    Struct,
    Union,
}

#[derive(Debug, Clone)]
pub struct InterfaceDef {
    pub is_pub: bool,
    pub name: String,
    pub methods: Vec<InterfaceMethod>,
}

#[derive(Debug, Clone)]
pub struct InterfaceMethod {
    pub is_pub: bool,
    pub name: String,
    pub params: Vec<Param>,
    pub return_type: Type,
}

#[derive(Debug, Clone)]
pub struct FieldDef {
    pub name: String,
    pub ty: Type,
}

#[derive(Debug, Clone)]
pub struct UnionVariantDef {
    pub name: String,
    pub payload_types: Vec<Type>,
}

#[derive(Debug, Clone)]
pub struct Function {
    pub is_pub: bool,
    pub name: String,
    pub extern_name: Option<String>,
    pub generic_params: Vec<GenericParam>,
    pub params: Vec<Param>,
    pub return_type: Type,
    pub body: Vec<Stmt>,
}

#[derive(Debug, Clone)]
pub struct GenericParam {
    pub name: String,
    pub constraints: Vec<Type>,
}

#[derive(Debug, Clone)]
pub struct Param {
    pub name: String,
    pub ty: Type,
}

#[derive(Debug, Clone)]
pub struct TestBlock {
    pub name: String,
    pub body: Vec<Stmt>,
}

#[derive(Debug, Clone)]
pub struct MatchArm {
    pub kind: MatchArmKind,
    pub bindings: Vec<Option<String>>,
    pub body: Vec<Stmt>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum MatchArmKind {
    Ok,
    Error,
    Variant(String),
}

#[derive(Debug, Clone, PartialEq, Eq, Hash)]
pub enum Type {
    Void,
    Bool,
    I8,
    I16,
    I32,
    I64,
    Isize,
    U16,
    U32,
    U64,
    Usize,
    U8,
    F32,
    F64,
    Infer,
    Named(String),
    Mut(Box<Type>),
    Ref(Box<Type>),
    List(Box<Type>),
    Result(Box<Type>),
    Error,
    None,
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
    MulAssign {
        line: usize,
        column: usize,
        target: Expr,
        value: Expr,
    },
    SubAssign {
        line: usize,
        column: usize,
        target: Expr,
        value: Expr,
    },
    DivAssign {
        line: usize,
        column: usize,
        target: Expr,
        value: Expr,
    },
    BitAndAssign {
        line: usize,
        column: usize,
        target: Expr,
        value: Expr,
    },
    BitOrAssign {
        line: usize,
        column: usize,
        target: Expr,
        value: Expr,
    },
    BitXorAssign {
        line: usize,
        column: usize,
        target: Expr,
        value: Expr,
    },
    Increment {
        line: usize,
        column: usize,
        target: Expr,
    },
    Decrement {
        line: usize,
        column: usize,
        target: Expr,
    },
    Assert {
        line: usize,
        column: usize,
        condition: Expr,
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
    Match {
        line: usize,
        column: usize,
        expr: Expr,
        arms: Vec<MatchArm>,
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
    While {
        line: usize,
        column: usize,
        condition: Expr,
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
    Break {
        line: usize,
        column: usize,
    },
}

#[derive(Debug, Clone)]
pub enum Expr {
    Int(i64),
    Char(u8),
    Bool(bool),
    Float(f64),
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
    Specialize {
        callee: Box<Expr>,
        type_args: Vec<Type>,
    },
    Cast {
        expr: Box<Expr>,
        ty: Type,
    },
    None,
    Error {
        message: Box<Expr>,
    },
    Try(Box<Expr>),
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
    Subtract,
    Divide,
    Multiply,
    Modulo,
    LogicalAnd,
    LogicalOr,
    BitAnd,
    BitOr,
    BitXor,
    ShiftLeft,
    ShiftRight,
    LessThan,
    LessEqual,
    GreaterThan,
    GreaterEqual,
    Equal,
    NotEqual,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum UnaryOp {
    Neg,
    LogicalNot,
    BitNot,
}

impl Expr {
    pub fn as_path(&self) -> Option<&[String]> {
        match self {
            Self::Path(path) => Some(path),
            _ => None,
        }
    }

    pub fn callee_path(&self) -> Option<Vec<String>> {
        match self {
            Self::Path(path) => Some(path.clone()),
            Self::FieldAccess { base, field } => {
                let mut path = base.callee_path()?;
                path.push(field.clone());
                Some(path)
            }
            _ => None,
        }
    }
}
