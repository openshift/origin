// +build linux,cgo darwin,cgo

package log

const (
	ANSI_DEFAULT = "\033[39m"
	ANSI_RED     = "\033[31m"
	ANSI_GREEN   = "\033[32m"
	ANSI_YELLOW  = "\033[33m"
	ANSI_BLUE    = "\033[34m"
	ANSI_RESET   = "\033[0m"
)
