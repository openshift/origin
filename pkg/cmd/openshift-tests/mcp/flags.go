package mcp

import (
	"fmt"

	"github.com/spf13/pflag"
)

type MCPFlags struct {
	ListenAddress string
	Mode          string
}

func NewMCPFlags() *MCPFlags {
	return &MCPFlags{
		ListenAddress: ":8080",
		Mode:          "http",
	}
}

func (f *MCPFlags) BindFlags(flags *pflag.FlagSet) {
	flags.StringVarP(&f.ListenAddress, "listen-address", "l", f.ListenAddress, "Address and port to listen on for the MCP server (only used in http mode)")
	flags.StringVarP(&f.Mode, "mode", "m", f.Mode, "Server mode: 'stdio' for standard input/output communication, 'http' for HTTP server")
}

func (f *MCPFlags) ToOptions() (*MCPOptions, error) {
	if f.Mode != "stdio" && f.Mode != "http" {
		return nil, fmt.Errorf("invalid mode %q: must be 'stdio' or 'http'", f.Mode)
	}
	return &MCPOptions{
		ListenAddress: f.ListenAddress,
		Mode:          f.Mode,
	}, nil
}
