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
        return this.age

Cat myCat = new Cat()
myCat.setAge(10)
int age = myCat.getAge()
myCat.setInfo(8, "Whiskers")
reassign age = myCat.getAge()
print "The age was %d" | age
