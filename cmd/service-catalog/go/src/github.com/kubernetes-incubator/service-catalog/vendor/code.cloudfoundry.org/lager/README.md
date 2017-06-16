lager
=====

**Note**: This repository should be imported as `code.cloudfoundry.org/lager`.

Lager is a logging library for go.

## Usage

Instantiate a logger with the name of your component.

```go
import (
  "code.cloudfoundry.org/lager"
)

logger := lager.NewLogger("my-app")
```

### Sinks

Lager can write logs to a variety of destinations. You can specify the destinations
using Lager sinks:

To write to an arbitrary `Writer` object:

```go
logger.RegisterSink(lager.NewWriterSink(myWriter, lager.INFO))
```

### Emitting logs

Lager supports the usual level-based logging, with an optional argument for arbitrary key-value data.

```go
logger.Info("doing-stuff", lager.Data{
  "informative": true,
})
```

output:
```json
{ "source": "my-app", "message": "doing-stuff", "data": { "informative": true }, "timestamp": 1232345, "log_level": 1 }
```

Error messages also take an `Error` object:

```go
logger.Error("failed-to-do-stuff", errors.New("Something went wrong"))
```

output:
```json
{ "source": "my-app", "message": "failed-to-do-stuff", "data": { "error": "Something went wrong" }, "timestamp": 1232345, "log_level": 1 }
```

### Sessions

You can avoid repetition of contextual data using 'Sessions':

```go

contextualLogger := logger.Session("my-task", lager.Data{
  "request-id": 5,
})

contextualLogger.Info("my-action")
```

output:

```json
{ "source": "my-app", "message": "my-task.my-action", "data": { "request-id": 5 }, "timestamp": 1232345, "log_level": 1 }
```

## License

Lager is [Apache 2.0](https://github.com/cloudfoundry/lager/blob/master/LICENSE) licensed.
