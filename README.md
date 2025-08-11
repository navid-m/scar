## Scar

In-development general purpose systems programming language with abstracted concurrency constructs builtin.

No macros or convoluted compile-time metaprogramming.

Optional garbage collection. The standard library will work with or without the GC.

Prioritizes:

-  immutability
-  minimalism
-  readability
-  easy parallelism

#### Resources

Complete documentation is available [here](scarlang-docs.pages.dev).

The VSCode extension is available [here](https://marketplace.visualstudio.com/items?itemName=NavidM.scar).

#### Example

```scar
pub fn do_thing() -> void:
    parallel for i = 1 to 5:
        print "i = %d" | i
        sleep 0.1
    print "Parallel for loop completed."
do_thing()
```

---

<font color="grey">(Under construction)</font>

<img src="assets/wip.png" style="border-radius: 5px;" width=45%>
