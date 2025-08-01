        .file   "<stdin>"
        .text
        .globl  _exception
        .bss
        .align 4
_exception:
        .space 4
        .text
        .globl  length
        .def    length; .scl    2;      .type   32;     .endef
        .seh_proc       length
length:
        pushq   %rbp
        .seh_pushreg    %rbp
        movq    %rsp, %rbp
        .seh_setframe   %rbp, 0
        subq    $32, %rsp
        .seh_stackalloc 32
        .seh_endprologue
        movq    %rcx, 16(%rbp)
        movq    16(%rbp), %rax
        movq    %rax, %rcx
        call    strlen
        addq    $32, %rsp
        popq    %rbp
        ret
        .seh_endproc
        .globl  compare
        .def    compare;        .scl    2;      .type   32;     .endef
        .seh_proc       compare
compare:
        pushq   %rbp
        .seh_pushreg    %rbp
        movq    %rsp, %rbp
        .seh_setframe   %rbp, 0
        subq    $32, %rsp
        .seh_stackalloc 32
        .seh_endprologue
        movq    %rcx, 16(%rbp)
        movq    %rdx, 24(%rbp)
        movq    24(%rbp), %rdx
        movq    16(%rbp), %rax
        movq    %rax, %rcx
        call    strcmp
        addq    $32, %rsp
        popq    %rbp
        ret
        .seh_endproc
        .globl  concat
        .def    concat; .scl    2;      .type   32;     .endef
        .seh_proc       concat
concat:
        pushq   %rbp
        .seh_pushreg    %rbp
        pushq   %rbx
        .seh_pushreg    %rbx
        subq    $56, %rsp
        .seh_stackalloc 56
        leaq    48(%rsp), %rbp
        .seh_setframe   %rbp, 48
        .seh_endprologue
        movq    %rcx, 32(%rbp)
        movq    %rdx, 40(%rbp)
        movq    32(%rbp), %rax
        movq    %rax, %rcx
        call    strlen
        movq    %rax, %rbx
        movq    40(%rbp), %rax
        movq    %rax, %rcx
        call    strlen
        addq    %rbx, %rax
        addq    $1, %rax
        movq    %rax, %rcx
        call    malloc
        movq    %rax, -8(%rbp)
        movq    32(%rbp), %rdx
        movq    -8(%rbp), %rax
        movq    %rax, %rcx
        call    strcpy
        movq    40(%rbp), %rdx
        movq    -8(%rbp), %rax
        movq    %rax, %rcx
        call    strcat
        movq    -8(%rbp), %rax
        addq    $56, %rsp
        popq    %rbx
        popq    %rbp
        ret
        .seh_endproc
        .globl  main
        .def    main;   .scl    2;      .type   32;     .endef
        .seh_proc       main
main:
        pushq   %rbp
        .seh_pushreg    %rbp
        movq    %rsp, %rbp
        .seh_setframe   %rbp, 0
        subq    $32, %rsp
        .seh_stackalloc 32
        .seh_endprologue
        call    __main
        movl    $0, %eax
        addq    $32, %rsp
        popq    %rbp
        ret
        .seh_endproc
        .def    __main; .scl    2;      .type   32;     .endef
        .ident  "GCC: (MinGW-W64 x86_64-ucrt-posix-seh, built by Brecht Sanders, r3) 14.2.0"
        .def    strlen; .scl    2;      .type   32;     .endef
        .def    strcmp; .scl    2;      .type   32;     .endef
        .def    malloc; .scl    2;      .type   32;     .endef
        .def    strcpy; .scl    2;      .type   32;     .endef
        .def    strcat; .scl    2;      .type   32;     .endef