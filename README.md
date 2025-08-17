## Scar

In-development general purpose systems programming language with abstracted concurrency constructs builtin.

No convoluted compile-time metaprogramming.

Optional garbage collection. The standard library will work with or without the GC.

Prioritizes:

-  immutability
-  minimalism
-  readability
-  easy parallelism

#### Example

```scar
pub fn do_thing() -> void:
    parallel for i = 1 to 5:
        print "i = %d" | i
        sleep 0.1
    print "Parallel for loop completed."

do_thing()
```

#### Getting Started

##### Windows

-  Unzip `scar.zip` to some folder, then add that folder to system PATH.
-  Run the `setup.cmd` script

##### Linux (experimental)

-  Get prebuilt version from releases.
-  Extract zip.
-  Add `scar` binary location to env.

##### MacOS (experimental)

-  Install the go toolchain
-  Clone the repository
-  Run `go build`
-  Add the folder with the executable to zsh profile (`.zprofile`).

#### Resources

Documentation is available [here](https://scarlang-docs.pages.dev).

The VSCode extension is available [here](https://marketplace.visualstudio.com/items?itemName=NavidM.scar).

---

<font color="grey">(Under construction)</font>

<img src="assets/wip.png" style="border-radius: 5px;" width=45%>
