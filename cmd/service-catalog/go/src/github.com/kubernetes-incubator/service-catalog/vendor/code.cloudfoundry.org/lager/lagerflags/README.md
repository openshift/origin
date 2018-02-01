lagerflags
========

**Note**: This repository should be imported as `code.cloudfoundry.org/lager/lagerflags`.

This library provides a flag called `logLevel`. The logger returned by
`lagerflags.New()` will use the value of that flag to determine the log level.

To use, simply import this package in your `main.go` and call `lagerflags.New(COMPONENT_NAME)` to get a logger.

For example:

```golang
package main

import (
    "flag"
    "fmt"

    "code.cloudfoundry.org/lager/lagerflags"
    "code.cloudfoundry.org/lager"
)

func main() {
    lagerflags.AddFlags(flag.CommandLine)

    flag.Parse()

    logger, reconfigurableSink := lagerflags.New("my-component")
    logger.Info("starting")

    // Display the current minimum log level
    fmt.Printf("Current log level is ")
    switch reconfigurableSink.GetMinLevel() {
    case lager.DEBUG:
        fmt.Println("debug")
    case lager.INFO:
        fmt.Println("info")
    case lager.ERROR:
        fmt.Println("error")
    case lager.FATAL:
        fmt.Println("fatal")
    }

    // Change the minimum log level dynamically
    reconfigurableSink.SetMinLevel(lager.ERROR)
    logger.Debug("will-not-log")
}
```

Running the program above as `go run main.go --logLevel debug` will generate the following output:

```
{"timestamp":"1464388983.540486336","source":"my-component","message":"my-component.starting","log_level":1,"data":{}}
Current log level is debug
```
