# JSON schema

This directory contains [JSON Schema][json-schema] for validating JSON covered by local specifications.
The runtime specification includes [a generic command line tool][validate] which may be used to validate JSON with these schemas.

## OCI Runtime Command Line Interface

The [Runtime Command Line Interface](../docs/command-line-interface.md) defines:

* [Terminal requests](../docs/command-line-interface.md#requests), which may be validated against [`socket-terminal-request.json`](socket-terminal-request.json).
* [Responses](../docs/command-line-interface.md#reqponses), which may be validated against [`socket-response.json`](socket-response.json).

[json-schema]: http://json-schema.org/
[validate]: https://github.com/opencontainers/runtime-spec/tree/v1.0.0-rc5/schema#utility
