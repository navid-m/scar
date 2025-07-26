string code = "++++++++[>++++[>++>+++>+++>+<<<<-]>+>+>->>+[<]<-]>>.>---.+++++++..+++.>>.<-.<.+++.------.--------.>>+.>++." 
list[int] tape = []
int ptr = 0
int pc = 0
int code_len = len(code)

for i = 0 to 299:
    reassign tape[i] = 0

print "Running the brainfuck code..."

while pc < code_len:
    char chara = code[pc]

    if chara == '>':
        reassign ptr = ptr + 1

    elif chara == '<':
        reassign ptr = ptr - 1

    elif chara == '+':
        reassign tape[ptr] = tape[ptr] + 1

    elif chara == '-':
        reassign tape[ptr] = tape[ptr] - 1

    elif chara == '.':
        print "%c" | tape[ptr]

    elif chara == ',':
        reassign tape[ptr] = 0

    elif chara == '[':
        if tape[ptr] == 0:
            int loop = 1
            while loop > 0:
                reassign pc = pc + 1
                if code[pc] == '[':
                    reassign loop = loop + 1
                elif code[pc] == ']':
                    reassign loop = loop - 1

    elif chara == ']':
        if tape[ptr] != 0:
            int loop = 1
            while loop > 0:
                reassign pc = pc - 1
                if code[pc] == ']':
                    reassign loop = loop + 1
                elif code[pc] == '[':
                    reassign loop = loop - 1
            reassign pc = pc - 1  

    reassign pc = pc + 1
