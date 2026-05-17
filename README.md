## Scar

General purpose systems programming language with abstracted concurrency and parallelism constructs builtin.

No convoluted compile-time metaprogramming.

Optional garbage collection. The standard library will work with or without the GC.

Prioritizes:

-  immutability
-  minimalism
-  readability
-  easy parallelism

#### Example

```scar
def add[T](x: T, y: T)
	return x + y
end

def mix_add[A: i32|i64, B: f32|f64, C: f64](x: A, y: B) C
    return (x as C) + (y as C)
end

pub def main()
	@print("{d}\n", add[i32](120, 140))
	@print("{lf}\n", mix_add[i32, f32, f64](1, 2.0))
end
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
