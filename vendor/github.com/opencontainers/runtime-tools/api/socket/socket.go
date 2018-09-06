package socket

// Common is the normal data for messages passed on the console socket.
type Common struct {
	// Type of message being passed
	Type string `json:"type"`
}

// TerminalRequest is the normal data for messages passing a pseudoterminal master.
type TerminalRequest struct {
	Common

	// Container ID for the container whose pseudoterminal master is being set.
	Container string `json:"container"`
}

// Response is the normal data for response messages.
type Response struct {
	Common

	// Message is a phrase describing the response.
	Message string `json:"message,omitempty"`
}
