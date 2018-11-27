# goenumcheck

`goenumcheck` provides assertion that, given an "enum" type in Go, any `switch` statements ranging
over a value of that type cover all declared instances.

For example:

```go
package dessert

type iceCreamFlavour int

const (
  vanilla iceCreamFlavour = iota
  chocolate
  swirl
)

func announce(flavour iceCreamFlavour) {
  switch flavour {
  case vanilla:
    println("vanilla")
  case chocolate:
    println("chocolate")
  }
}
```

Running `goenumcheck` on this file will produce:

```
/tmp/dessert.go:12:2: uncovered cases for iceCreamFlavour enum switch
        - swirl (EC1000)
```

`goenumcheck` looks for types aliased to `int` or `int32`, which have package-level `const` values
of that type. For those types, when a `switch` is found ranging over a value of that type, all
`const` instances of that type must be explicitly mentioned in `case` statements in order to pass
the check.

See `goenumcheck -h` for more help.
