package test

import "regexp"

// ansiRegex contains a regex for stripping out ANSI control sequences. See https://github.com/acarl005/stripansi/blob/master/stripansi.go
var ansiRegex = regexp.MustCompile("[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))")

// StripANSI removes ANSI control sequences from a string.
func StripANSI(s []byte) []byte {
	return ansiRegex.ReplaceAll(s, []byte(""))
}
