string code = "++++++++[>++++[>++>+++>+++>+<<<<-]>+>+>->>+[<]<-]>>.>---.+++++++..+++.>>.<-.<.+++.------.--------.>>+.>++." 
list[int] tape = []
int ptr = 0
int pc = 0
int code_len = len(code)

for i = 0 to 299:
    set tape[i] = 0

print "Running the brainfuck code..."

while pc < code_len:
    char chara = code[pc]

    if chara == '>':
        set ptr = ptr + 1

    elif chara == '<':
        set ptr = ptr - 1

    elif chara == '+':
        set tape[ptr] = tape[ptr] + 1

    elif chara == '-':
        set tape[ptr] = tape[ptr] - 1

    elif chara == '.':
        print "%c" | tape[ptr]

    elif chara == ',':
        set tape[ptr] = 0

    elif chara == '[':
        if tape[ptr] == 0:
            int loop = 1
            while loop > 0:
                set pc = pc + 1
                if code[pc] == '[':
                    set loop = loop + 1
                elif code[pc] == ']':
                    set loop = loop - 1

    elif chara == ']':
        if tape[ptr] != 0:
            int loop = 1
            while loop > 0:
                set pc = pc - 1
                if code[pc] == ']':
                    set loop = loop + 1
                elif code[pc] == '[':
                    set loop = loop - 1
            set pc = pc - 1  

    set pc = pc + 1
