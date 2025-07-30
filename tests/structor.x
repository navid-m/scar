class ArgConstructorClass:
    init(int a, int b):
        this.a = a #Note the right hand side type inference here, we dont have to say "int this.a = a"
        this.b = b
        print "a = %d", a
        print "b = %d", b

var acc = new ArgConstructorClass(1, 2)
