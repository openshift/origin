package mcp

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/mark3labs/mcp-go/server"
)

type MCPOptions struct {
	// ListenAddress is the address to listen on for the MCP server (only used in http mode).
	ListenAddress string

	// Mode is the server mode: 'stdio' (for local CLI invocation) or 'http'.
	Mode string
}

func (o *MCPOptions) Run() error {
	log.WithFields(log.Fields{
		"mode":           o.Mode,
		"listen_address": o.ListenAddress,
	}).Info("Initializing MCP server")

	// Create hooks for monitoring MCP protocol events
	hooks := &server.Hooks{}

	// Monitor session registration/unregistration
	hooks.AddOnRegisterSession(func(ctx context.Context, session server.ClientSession) {
		log.WithField("session_id", session.SessionID()).Info("MCP client session registered")
	})

	hooks.AddOnUnregisterSession(func(ctx context.Context, session server.ClientSession) {
		log.WithField("session_id", session.SessionID()).Info("MCP client session unregistered")
	})

	// Create the MCP server with common configuration
	mcpServer := server.NewMCPServer(
		"openshift-tests MCP Server",
		"0.0.1",
		server.WithLogging(),
		server.WithToolCapabilities(false),
		server.WithPromptCapabilities(false),
		server.WithRecovery(),
		server.WithHooks(hooks),
	)
	log.Debug("Created MCP server instance")

	registerTools(mcpServer)
	log.Info("All MCP tools registered successfully")

	registerPrompts(mcpServer)
	log.Info("All MCP prompts registered successfully")

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start the server based on the selected mode
	switch o.Mode {
	case "stdio":
		log.Info("Starting stdio MCP server (press Ctrl+C to stop)")
		log.Debug("Stdio server will read from stdin and write to stdout")

		// Run stdio server in a goroutine
		errChan := make(chan error, 1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.WithField("panic", r).Error("Stdio server panicked")
					errChan <- fmt.Errorf("stdio server panicked: %v", r)
				}
			}()

			log.Debug("Starting stdio server listener")
			err := server.ServeStdio(mcpServer)
			if err != nil {
				log.WithError(err).Error("Stdio server terminated with error")
			} else {
				log.Info("Stdio server terminated normally")
			}
			errChan <- err
		}()

		// Wait for either completion or signal
		select {
		case err := <-errChan:
			if err != nil {
				log.WithError(err).Error("Stdio server failed")
			}
			return err
		case sig := <-sigChan:
			log.WithField("signal", sig).Info("Received signal, shutting down stdio server...")
			return nil
		}

	case "http":
		httpServer := server.NewStreamableHTTPServer(mcpServer)
		log.WithField("address", o.ListenAddress).Info("Starting HTTP MCP server (press Ctrl+C to stop)")
		log.WithField("endpoint", fmt.Sprintf("http://localhost%s/mcp", o.ListenAddress)).Info("MCP server will be available at endpoint")
		log.Info("Server will log all MCP protocol events and connection state changes")

		// Start HTTP server in a goroutine
		errChan := make(chan error, 1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.WithField("panic", r).Error("HTTP server panicked")
					errChan <- fmt.Errorf("HTTP server panicked: %v", r)
				}
			}()

			log.Debug("Starting HTTP server listener")
			err := httpServer.Start(o.ListenAddress)
			if err != nil {
				log.WithError(err).Error("HTTP server terminated with error")
			} else {
				log.Info("HTTP server terminated normally")
			}
			errChan <- err
		}()

		// Wait for either completion or signal
		select {
		case err := <-errChan:
			if err != nil {
				log.WithError(err).Error("HTTP server failed")
				// Check for common error conditions
				if strings.Contains(err.Error(), "address already in use") {
					log.WithField("address", o.ListenAddress).Error("Port is already in use - another service may be running on this port")
				} else if strings.Contains(err.Error(), "permission denied") {
					log.WithField("address", o.ListenAddress).Error("Permission denied - you may need elevated privileges to bind to this port")
				}
			}
			return err
		case sig := <-sigChan:
			log.WithField("signal", sig).Info("Received signal, shutting down HTTP server...")
			// Gracefully shutdown the HTTP server
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()

			log.Debug("Initiating graceful shutdown")
			if shutdownErr := httpServer.Shutdown(shutdownCtx); shutdownErr != nil {
				log.WithError(shutdownErr).Error("Error during graceful shutdown")
			} else {
				log.Info("HTTP server shutdown completed successfully")
			}
			return nil
		}

	default:
		return fmt.Errorf("unsupported mode: %s", o.Mode)
	}
}

func NewMCPCommand() *cobra.Command {
	f := NewMCPFlags()

	mcpCmd := &cobra.Command{
		Use:   "mcp",
		Short: "Run an openshift-tests MCP server",
		Long: templates.LongDesc(`The openshift-tests MCP server allows you to interact with an instance of openshift-tests using your favorite LLM.

		The server can run in two modes:
		- stdio: Communicates via standard input/output (suitable for direct integration with LLM tools)
		- http: Runs as an HTTP server on a specified address and port (suitable for network-based access)`),

		SilenceUsage:  true,
		SilenceErrors: true,
		PreRunE: func(command *cobra.Command, args []string) error {
			// Only validate flags, don't require cluster access at startup
			// Cluster access will be checked when tools are actually called
			_, err := f.ToOptions()
			return err
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			o, err := f.ToOptions()
			if err != nil {
				return errors.WithMessage(err, "error converting to options")
			}

			if err := o.Run(); err != nil {
				return errors.WithMessage(err, "error running MCP server")
			}
			return nil
		},
	}
	f.BindFlags(mcpCmd.Flags())
	return mcpCmd
}
