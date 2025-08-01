## Scar

In-development general purpose systems programming language with abstracted concurrency constructs builtin.

No macros. Scar favors immutability, minimalism and readability over convoluted compile-time metaprogramming.

#### Example

```scar
pub fn do_thing() -> void:
    parallel for i = 1 to 5:
        print "i = %d" | i
        sleep 0.1
    print "Parallel for loop completed."

do_thing()
```
