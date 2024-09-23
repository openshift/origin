# go-json

This is a fork of Go's `encoding/json` library. It adds a third target for
unmarshalling, `json.Node`.

Unmarshalling to a `Node` behaves similarly to unmarshalling to
`any`, except that it also records the offsets for the start and end
of the value that is unmarshalled and, if the value is part of a JSON
object, the offsets of the start and end of the object's key. The `Value`
field of the `Node` is unmarshalled to the same type as if it were
`any`, except in the case of arrays and objects:

| JSON type | Go type, unmarshalled to `any` | `Node.Value` type |
| --------- | ------------------------------ | ----------------- |
| Array     | `[]any`                        | `[]Node`          |
| Object    | `map[string]any`               | `map[string]Node` |
| Other     | `any`                          | `any`             |
