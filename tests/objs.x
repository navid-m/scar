class Cat:
    init:
        int this.age = 5
        string this.name = "Fluffy"

    fn setAge(int newAge) -> void:
        reassign this.age = newAge

    fn setInfo(int newAge, string newName) -> void:
        reassign this.age = newAge
        reassign this.name = newName

    fn getAge() -> int:
        print "Age is %d" | this.age

Cat myCat = new Cat()
myCat.setAge(10)
myCat.setInfo(8, "Whiskers")
int age = myCat.getAge()

print "The age was %d" | age