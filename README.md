## Scar

In-development general purpose systems programming language with abstracted concurrency constructs builtin.

No macros or convoluted compile-time metaprogramming.

Prioritizes:

-  immutability
-  minimalism
-  readability

#### Example

```scar
pub fn do_thing() -> void:
    parallel for i = 1 to 5:
        print "i = %d" | i
        sleep 0.1
    print "Parallel for loop completed."

do_thing()
```

Reassignments are made explicit with `reassign` keyword.
All values are constant by default.

---

<font color="grey">(Under construction)</font>

<img src="assets/wip.png" style="border-radius: 5px;" width=45%>
