string code = "++++++++[>++++[>++>+++>+++>+<<<<-]>+>+>->>+[<]<-]>>.>---.+++++++..+++.>>.<-.<.+++.------.--------.>>+.>++." 
list[int] tape = []
int ptr = 0
int pc = 0
int code_len = len(code)

for i = 0 to 299:
    tape[i] = 0

print "Running the brainfuck code..."

while pc < code_len:
    char chara = code[pc]

    if chara == '>':
        ptr = ptr + 1

    elif chara == '<':
        ptr = ptr - 1

    elif chara == '+':
        tape[ptr] = tape[ptr] + 1

    elif chara == '-':
        tape[ptr] = tape[ptr] - 1

    elif chara == '.':
        print "%c" | tape[ptr]

    elif chara == ',':
        tape[ptr] = 0

    elif chara == '[':
        if tape[ptr] == 0:
            int loop = 1
            while loop > 0:
                pc = pc + 1
                if code[pc] == '[':
                    loop = loop + 1
                elif code[pc] == ']':
                    loop = loop - 1

    elif chara == ']':
        if tape[ptr] != 0:
            int loop = 1
            while loop > 0:
                pc = pc - 1
                if code[pc] == ']':
                    loop = loop + 1
                elif code[pc] == '[':
                    loop = loop - 1
            pc = pc - 1  

    pc = pc + 1
