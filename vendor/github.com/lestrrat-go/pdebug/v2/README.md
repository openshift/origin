# pdebug

[![Build Status](https://travis-ci.org/lestrrat-go/pdebug.svg?branch=master)](https://travis-ci.org/lestrrat-go/pdebug)

[![GoDoc](https://godoc.org/github.com/lestrrat-go/pdebug/v2?status.svg)](https://godoc.org/github.com/lestrrat-go/pdebug/v2)

Utilities for my print debugging fun. YMMV

# Synopsis

![optimized](https://pbs.twimg.com/media/CbiqhzLUUAIN_7o.png)

# Description

`pdebug` is a collection of tools to make **p**rint **debug**ging easier.

You can control the output of the debug prints via build tags:

```
# compile in the debug code, but only print the trace messages
# when the environment variable PDEBUG_TRACE is specified
go build -tags debug ...arguments...
```

```
# compile in the debug code, and always print the trace messages
go build -tags debug0 ...arguments...
```

Without the tags, nothing will be printed, and `pdebug` constructs
will be properly optimized out from the code. This way you can safely
pepper your code with debug statements and leasve them there without
worrying about them leaking in your production code.

When `debug0` tag is specified, the messages will always be printed.

When `debug` tag is specified, the messages will only appear when
`PDEBUG_TRACE=1` is specified at run time

# Printing

Single messages can be printed by simpley invoking `pdebug.Printf`.
When you disable pdebug, these function calls are effectively compiled out.

```go
import (
	"context"

	pdebug "github.com/lestrrat-go/pdebug/v2"
)

func main() {
	ctx := pdebug.Context(nil)
	pdebug.Printf(ctx, "Hello, World!")

	...
}
```

# Markers

There are occasions when you would like to see the context in which
the particular print statement was invoked from. For this purpose
we have a "Marker" to mark entry points and exit points of particular
code segments:

```go
func main() {
	Foo(pdebug.Context(nil))
}

func Foo(ctx context.Context) {
	g := pdebug.Marker(ctx, "Foo") // ... START Foo
	defer g.End()                  // ... END   Foo (elapsed=...)

	pdebug.Printf("hello, world!") // This statement is printed in between START/END, indented
	...
}


```

The above will result in an output like below

```
|DEBUG| START Foo
|DEBUG|   hello, world!
|DEBUG| END Foo (elapsed=1.23μs)
```

The marker can also print out error values in a defer hook by binding
the pointer to an error. This is one of the rare cases where a named
return value is handy:

```go
func Foo(ctx context.Context) (err error) {
	g := pdebug.Marker(ctx, "Foo").BindError(&err)
	defer g.End()

	// much later ...
	return errors.New("something awful happend!")
}
```

This will include the textual representation of the error in the print
statement of the exit point.

```
|DEBUG| START Foo
...
|DEBUG| END Foo (elapsed=1.23μs, error=something awful happened!)
```

# Switch

For other cases where you want to include code that only gets executed
during debugging, you can use the constant boolean defined in this package.

```go
func foo(ctx context.Context) {
	if pdebug.Enabled {
		...
	}

	...
}
```

These sections will be constant folded when `pdebug` is disabled.

# Options

Options store values that are referenced from the logging functions.
You must use the new `context.Context` object that is created from the options.

## WithOutput(context.Context, Clock)

Specifies the Clock object to calculate the time. You probably do not need to use
this unless you are testing or doing something extremely evil

## WithOutput(context.Context, io.Writer)

Specifies where the logs are to be stored.

## WithPrefix(context.Context, string)

Specifies a static string that is prefixed in each line of the messages. Default is `|DEBUG| `

## WithTimestamp(context.Context, bool)

Enable or disable the insertion of timestamps.


