sigintwrap is an executable wrapper used in the automated testing of s2i on
Windows.  In hack/test-stirunimage.sh it is verified that sending an interrupt
to a running s2i process (i.e. sending a SIGINT on Linux, or specifically
pressing CTRL+C or CTRL+BREAK on Windows) causes the process to clean up a
running Docker container before exiting.

Cygwin is used for Windows build and testing of s2i, but note that the s2i
binary itself has no dependency on Cygwin (in general, Go and the executables it
compiles have no knowledge or dependency on Cygwin).

For the above test to be valid and succeed on Windows, it is necessary to bridge
between receiving the SIGINT sent by the test framework (note: on Windows,
POSIX-style signal handling is implemented only in Cygwin-compiled/-aware
executables, in userspace) and Golang acting on a native CTRL+C/CTRL+BREAK
event.

sigintwrap is a Cygwin-compiled executable which spawns a named executable in a
new Windows process group and waits until it exits.  If before that time
sigintwrap receives a SIGINT signal it sends a synthetic CTRL+BREAK event to the
process it started.

This approach is used (rather than, say, simply using the
GenerateConsoleCtrlEvent API directly to create a synthetic event) because there
is no obvious and straightforward way to spawn a new process into a new Windows
process group in Cygwin without resorting to C, nor is there an obvious free
lightweight pre-existing utility which can be used to send a synthetic
CTRL+BREAK event to a given process.
