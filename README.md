## Scar

General purpose systems programming language with a high-level, ruby-like syntax. Aims to achieve low-level control while still being readable and concise.

#### Example

```scar
def add[T](x: T, y: T) T
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

#### Resources

- [Documentation](https://scarlang-docs.pages.dev)
- [VSCode Extension](https://marketplace.visualstudio.com/items?itemName=NavidM.scar)

---

<font color="grey">(Under construction)</font>
