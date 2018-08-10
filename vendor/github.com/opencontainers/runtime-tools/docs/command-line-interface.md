# OCI Runtime Command Line Interface

This file defines the OCI Runtime Command Line Interface version 1.0.1.
It is one of potentially several [runtime APIs supported by the runtime compliance test suite](runtime-compliance-testing.md#supported-apis).

## Notation

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD", "SHOULD NOT", "RECOMMENDED",  "NOT RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be interpreted as described in [RFC 2119][rfc2119].

The key words "unspecified", "undefined", and "implementation-defined" are to be interpreted as described in the [rationale for the C99 standard][c99-unspecified].

## Compliance

This specification targets compliance criteria according to the role of a participant in runtime invocation.
Requirements are placed on both runtimes and runtime callers, depending on what behavior is being constrained by the requirement.
An implementation is compliant if it satisfies all the applicable MUST, REQUIRED, and SHALL requirements.
An implementation is not compliant if it fails to satisfy one or more of the applicable MUST, REQUIRED, and SHALL requirements.

## Versioning

The command line interface is versioned with [SemVer v2.0.0][semver].
The command line interface version is independent of the OCI Runtime Specification as a whole (which is tied to the [configuration format][runtime-spec-version].
For example, if a caller is compliant with version 1.1 of the command line interface, they are compatible with all runtimes that support any 1.1 or later release of the command line interface, but are not compatible with a runtime that supports 1.0 and not 1.1.

## Global usage

The runtime MUST provide an executable (called `funC` in the following examples).
That executable MUST support commands with the following template:

```
$ funC [global-options] <COMMAND> [command-specific-options] <command-specific-arguments>
```

The runtime MUST support the entire API defined in this specification.
Runtimes MAY also support additional options and commands as extensions to this API, but callers interested in OCI-runtime portability SHOULD NOT rely on those extensions.

## Global options

None are required, but the runtime MAY support options that start with at least one hyphen.
Global options MAY take positional arguments (e.g. `--log-level debug`).
Command names MUST NOT start with hyphens.
The option parsing MUST be such that `funC <COMMAND>` is unambiguously an invocation of `<COMMAND>` (even for commands not specified in this document).
If the runtime is invoked with an unrecognized command, it MUST exit with a nonzero exit code and MAY log a warning to stderr.
Beyond the above rules, the behavior of the runtime in the presence of commands and options not specified in this document is unspecified.

## Character encodings

This API specification does not cover character encodings, but runtimes SHOULD conform to their native operating system.
For example, POSIX systems define [`LANG` and related environment variables][posix-lang] for [declaring][posix-locale-encoding] [locale-specific character encodings][posix-encoding], so a runtime in an `en_US.UTF-8` locale SHOULD write its [state](#state) to stdout in [UTF-8][].

## Commands

### create

[Create][create] a container from a [bundle directory][bundle].

* *Arguments*
    * *`<ID>`* Set the container ID to create.
* *Options*
    * *`--bundle <PATH>`* Override the path to the [bundle directory][bundle] (defaults to the current working directory).
    * *`--console-socket <FD>`* The runtime MUST pass the [pseudoterminal master][posix_openpt.3] through the open socket at file descriptor `<FD>`; the protocol is [described below](#console-socket).
    * *`--pid-file <PATH>`* The runtime MUST write the container PID to this path.
* *Standard streams:*
    * If [`process.terminal`][process] is true:
        * *stdin:* The runtime MUST NOT attempt to read from its stdin.
        * *stdout:* The handling of stdout is unspecified.
        * *stderr:* The runtime MAY print diagnostic messages to stderr, and the format for those lines is not specified in this document.
    * If [`process.terminal`][process] is not true:
        * *stdin:* The runtime MUST pass its stdin file descriptor through to the container process without manipulation or modification.
          "Without manipulation or modification" means that the runtime MUST not seek on the file descriptor, or close it, or read or write to it, or [`ioctl`][ioctl.3] it, or perform any other action on it besides passing it through to the container process.

          When using a container to drop privileges, note that providing a privileged terminal's file descriptor may allow the container to [execute privileged operations via `TIOCSTI`][TIOCSTI-security] or other [TTY ioctls][tty_ioctl.4].
          On Linux, [`TIOCSTI` requires `CAP_SYS_ADMIN`][capabilities.7] unless the target terminal is the caller's [controlling terminal][controlling-terminal].
        * *stdout:* The runtime MUST pass its stdout file descriptor through to the container process without manipulation or modification.
        * *stderr:* When `create` exists with a zero code, the runtime MUST pass its stderr file descriptor through to the container process without manipulation or modification.
          When `create` exits with a non-zero code, the runtime MAY print diagnostic messages to stderr, and the format for those lines is not specified in this document.
* *Environment variables*
    * *`LISTEN_FDS`:* The number of file descriptors passed.
      For example, `LISTEN_FDS=2` would mean that the runtime MUST pass file descriptors 3 and 4 to the container process (in addition to the standard streams) to support [socket activation][systemd-listen-fds].
* *Exit code:* Zero if the container was successfully created and non-zero on errors.

Callers MAY block on this command's successful exit to trigger post-create activity.

#### Console socket

The [`AF_UNIX`][unix-socket] used by [`--console-socket`](#create) handles request and response messages between a runtime and server.
The socket type MUST be [`SOCK_SEQPACKET`][socket-types] or [`SOCK_STREAM`][socket-types].
The server MUST send a single response for each runtime request.
The [normal data][socket-queue] ([`msghdr.msg_iov*`][socket.h]) of all messages MUST be [UTF-8][] [JSON](glossary.md#json).

There are [JSON Schemas](../schema/README.md#oci-runtime-command-line-interface) and [Go bindings](../api/socket/socket.go) for the messages specified in this section.

##### Requests

All requests MUST contain a **`type`** property whose value MUST one of the following strings:

* `terminal`, if the request is passing a [pseudoterminal master][posix_openpt.3].
    When `type` is `terminal`, the request MUST also contain the following properties:

    * **`container`** (string, REQUIRED) The container ID, as set by [create](#create).

    The message's [ancillary data][socket-queue] (`msg_control*`) MUST contain at least one [`cmsghdr`][socket.h]).
    The first `cmsghdr` MUST have:

    * `cmsg_type` set to [`SOL_SOCKET`][socket.h],
    * `cmsg_level` set to [`SCM_RIGHTS`][socket.h],
    * `cmsg_len` greater than or equal to `CMSG_LEN(sizeof(int))`, and
    * `((int*)CMSG_DATA(cmsg))[0]` set to the pseudoterminal master file descriptor.

##### Responses

All responses MUST contain a **`type`** property whose value MUST be one of the following strings:

* `success`, if the request was successfully processed.
* `error`, if the request was not successfully processed.

In addition, responses MAY contain any of the following properties:

* **`message`** (string, OPTIONAL) A phrase describing the response.

#### Example

```
# in a bundle directory with a process that echos "hello" and exits 42
$ test -t 1 && echo 'stdout is a terminal'
stdout is a terminal
$ funC create hello-1 <&- >stdout 2>stderr
$ echo $?
0
$ wc stdout
0 0 0 stdout
$ funC start hello-1
$ echo $?
0
$ cat stdout
hello
$ block-on-exit-and-collect-exit-code hello-1
$ echo $?
42
$ funC delete hello-1
$ echo $?
0
```

#### Container process exit

The [example's](#example) `block-on-exit-and-collect-exit-code` is platform-specific and is not specified in this document.
On Linux, it might involve an ancestor process which had set [`PR_SET_CHILD_SUBREAPER`][prctl.2] and collected the container PID [from the state][state], or a process that was [ptracing][ptrace.2] the container process for [`exit_group`][exit_group.2], although both of those race against the container process exiting before the watcher is monitoring.

### start

[Start][start] the user-specified code from [`process`][process].

* *Arguments*
    * *`<ID>`* The container to start.
* *Standard streams:*
    * *stdin:* The runtime MUST NOT attempt to read from its stdin.
    * *stdout:* The handling of stdout is unspecified.
    * *stderr:* The runtime MAY print diagnostic messages to stderr, and the format for those lines is not specified in this document.
* *Exit code:* Zero if the container was successfully started and non-zero on errors.

Callers MAY block on this command's successful exit to trigger post-start activity.

See [create](#example) for an example.

### state

[Request][state-request] the container [state][state].

* *Arguments*
    * *`<ID>`* The container whose state is being requested.
* *Standard streams:*
    * *stdin:* The runtime MUST NOT attempt to read from its stdin.
    * *stdout:* The runtime MUST print the [state JSON][state] to its stdout.
    * *stderr:* The runtime MAY print diagnostic messages to stderr, and the format for those lines is not specified in this document.
* *Exit code:* Zero if the state was successfully written to stdout and non-zero on errors.

#### Example

```
$ funC create sleeper-1
$ funC state sleeper-1
{
  "ociVersion": "1.0.0-rc1",
  "id": "sleeper-1",
  "status": "created",
  "pid": 4422,
  "bundlePath": "/containers/sleeper",
  "annotations" {
    "myKey": "myValue"
  }
}
$ echo $?
0
```

### kill

[Send a signal][kill] to the container process.

* *Arguments*
    * *`<ID>`* The container being signaled.
* *Options*
    * *`--signal <SIGNAL>`* The signal to send (defaults to `TERM`).
      The runtime MUST support `TERM` and `KILL` signals with [the POSIX semantics][posix-signals].
      The runtime MAY support additional signal names.
      On platforms that support [POSIX signals][posix-signals], the runtime MUST implement this command using POSIX signals.
      On platforms that do not support POSIX signals, the runtime MAY implement this command with alternative technology as long as `TERM` and `KILL` retain their POSIX semantics.
      Runtime authors on non-POSIX platforms SHOULD submit documentation for their TERM implementation to this specificiation, so runtime callers can configure the container process to gracefully handle the signals.
* *Standard streams:*
    * *stdin:* The runtime MUST NOT attempt to read from its stdin.
    * *stdout:* The handling of stdout is unspecified.
    * *stderr:* The runtime MAY print diagnostic messages to stderr, and the format for those lines is not specified in this document.
* *Exit code:* Zero if the signal was successfully sent to the container process and non-zero on errors.
  Successfully sent does not mean that the signal was successfully received or handled by the container process.

#### Example

```
# in a bundle directory where the container process ignores TERM
$ funC create sleeper-1
$ funC start sleeper-1
$ funC kill sleeper-1
$ echo $?
0
$ funC kill --signal KILL sleeper-1
$ echo $?
0
```

### delete

[Release](#delete) container resources after the container process has exited.

* *Arguments*
    * *`<ID>`* The container to delete.
* *Standard streams:*
    * *stdin:* The runtime MUST NOT attempt to read from its stdin.
    * *stdout:* The handling of stdout is unspecified.
    * *stderr:* The runtime MAY print diagnostic messages to stderr, and the format for those lines is not specified in this document.
* *Exit code:* Zero if the container was successfully deleted and non-zero on errors.

See [create](#example) for an example.

[bundle]: https://github.com/opencontainers/runtime-spec/blob/v1.0.0-rc4/bundle.md
[c99-unspecified]: http://www.open-std.org/jtc1/sc22/wg14/www/C99RationaleV5.10.pdf#page=18
[capabilities.7]: http://man7.org/linux/man-pages/man7/capabilities.7.html
[controlling-terminal]: http://pubs.opengroup.org/onlinepubs/9699919799/basedefs/V1_chap11.html#tag_11_01_03
[create]: https://github.com/opencontainers/runtime-spec/blob/v1.0.0-rc4/runtime.md#create
[delete]: https://github.com/opencontainers/runtime-spec/blob/v1.0.0-rc4/runtime.md#delete
[exit_group.2]: http://man7.org/linux/man-pages/man2/exit_group.2.html
[ioctl.3]: http://pubs.opengroup.org/onlinepubs/9699919799/
[kill]: https://github.com/opencontainers/runtime-spec/blob/v1.0.0-rc4/runtime.md#kill
[kill.2]: http://man7.org/linux/man-pages/man2/kill.2.html
[process]: https://github.com/opencontainers/runtime-spec/blob/v1.0.0-rc4/config.md#process
[posix-encoding]: http://pubs.opengroup.org/onlinepubs/9699919799/basedefs/V1_chap06.html#tag_06_02
[posix-lang]: http://pubs.opengroup.org/onlinepubs/9699919799/basedefs/V1_chap08.html#tag_08_02
[posix-locale-encoding]: http://www.unicode.org/reports/tr35/#Bundle_vs_Item_Lookup
[posix_openpt.3]: http://pubs.opengroup.org/onlinepubs/9699919799/functions/posix_openpt.html
[posix-signals]: http://pubs.opengroup.org/onlinepubs/9699919799/basedefs/signal.h.html#tag_13_42_03
[prctl.2]: http://man7.org/linux/man-pages/man2/prctl.2.html
[ptrace.2]: http://man7.org/linux/man-pages/man2/ptrace.2.html
[semver]: http://semver.org/spec/v2.0.0.html
[socket-queue]: http://pubs.opengroup.org/onlinepubs/9699919799/functions/V2_chap02.html#tag_15_10_11
[socket-types]: http://pubs.opengroup.org/onlinepubs/9699919799/functions/V2_chap02.html#tag_15_10_06
[socket.h]: http://pubs.opengroup.org/onlinepubs/9699919799/basedefs/sys_socket.h.html
[standard-streams]: https://github.com/opencontainers/specs/blob/v0.1.1/runtime-linux.md#file-descriptors
[start]: https://github.com/opencontainers/runtime-spec/blob/v1.0.0-rc4/runtime.md#start
[state]: https://github.com/opencontainers/runtime-spec/blob/v1.0.0-rc4/runtime.md#state
[state-request]: https://github.com/opencontainers/runtime-spec/blob/v1.0.0-rc4/runtime.md#query-state
[systemd-listen-fds]: http://www.freedesktop.org/software/systemd/man/sd_listen_fds.html
[rfc2119]: https://tools.ietf.org/html/rfc2119
[runtime-spec-version]: https://github.com/opencontainers/runtime-spec/blob/v1.0.0-rc4/config.md#specification-version
[TIOCSTI-security]: https://bugs.debian.org/cgi-bin/bugreport.cgi?bug=628843
[tty_ioctl.4]: http://man7.org/linux/man-pages/man4/tty_ioctl.4.html
[unix-socket]: http://pubs.opengroup.org/onlinepubs/9699919799/functions/V2_chap02.html#tag_15_10_17
[UTF-8]: http://www.unicode.org/versions/Unicode8.0.0/ch03.pdf
