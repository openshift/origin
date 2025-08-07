// Package mcp defines the core types and interfaces for the Model Context Protocol (MCP).
// MCP is a protocol for communication between LLM-powered applications and their supporting services.
package mcp

import (
	"encoding/json"
	"fmt"
	"maps"
	"strconv"

	"github.com/yosida95/uritemplate/v3"
	"net/http"
)

type MCPMethod string

const (
	// MethodInitialize initiates connection and negotiates protocol capabilities.
	// https://modelcontextprotocol.io/specification/2024-11-05/basic/lifecycle/#initialization
	MethodInitialize MCPMethod = "initialize"

	// MethodPing verifies connection liveness between client and server.
	// https://modelcontextprotocol.io/specification/2024-11-05/basic/utilities/ping/
	MethodPing MCPMethod = "ping"

	// MethodResourcesList lists all available server resources.
	// https://modelcontextprotocol.io/specification/2024-11-05/server/resources/
	MethodResourcesList MCPMethod = "resources/list"

	// MethodResourcesTemplatesList provides URI templates for constructing resource URIs.
	// https://modelcontextprotocol.io/specification/2024-11-05/server/resources/
	MethodResourcesTemplatesList MCPMethod = "resources/templates/list"

	// MethodResourcesRead retrieves content of a specific resource by URI.
	// https://modelcontextprotocol.io/specification/2024-11-05/server/resources/
	MethodResourcesRead MCPMethod = "resources/read"

	// MethodPromptsList lists all available prompt templates.
	// https://modelcontextprotocol.io/specification/2024-11-05/server/prompts/
	MethodPromptsList MCPMethod = "prompts/list"

	// MethodPromptsGet retrieves a specific prompt template with filled parameters.
	// https://modelcontextprotocol.io/specification/2024-11-05/server/prompts/
	MethodPromptsGet MCPMethod = "prompts/get"

	// MethodToolsList lists all available executable tools.
	// https://modelcontextprotocol.io/specification/2024-11-05/server/tools/
	MethodToolsList MCPMethod = "tools/list"

	// MethodToolsCall invokes a specific tool with provided parameters.
	// https://modelcontextprotocol.io/specification/2024-11-05/server/tools/
	MethodToolsCall MCPMethod = "tools/call"

	// MethodSetLogLevel configures the minimum log level for client
	// https://modelcontextprotocol.io/specification/2025-03-26/server/utilities/logging
	MethodSetLogLevel MCPMethod = "logging/setLevel"

	// MethodNotificationResourcesListChanged notifies when the list of available resources changes.
	// https://modelcontextprotocol.io/specification/2025-03-26/server/resources#list-changed-notification
	MethodNotificationResourcesListChanged = "notifications/resources/list_changed"

	MethodNotificationResourceUpdated = "notifications/resources/updated"

	// MethodNotificationPromptsListChanged notifies when the list of available prompt templates changes.
	// https://modelcontextprotocol.io/specification/2025-03-26/server/prompts#list-changed-notification
	MethodNotificationPromptsListChanged = "notifications/prompts/list_changed"

	// MethodNotificationToolsListChanged notifies when the list of available tools changes.
	// https://spec.modelcontextprotocol.io/specification/2024-11-05/server/tools/list_changed/
	MethodNotificationToolsListChanged = "notifications/tools/list_changed"
)

type URITemplate struct {
	*uritemplate.Template
}

func (t *URITemplate) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Raw())
}

func (t *URITemplate) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	template, err := uritemplate.New(raw)
	if err != nil {
		return err
	}
	t.Template = template
	return nil
}

/* JSON-RPC types */

// JSONRPCMessage represents either a JSONRPCRequest, JSONRPCNotification, JSONRPCResponse, or JSONRPCError
type JSONRPCMessage any

// LATEST_PROTOCOL_VERSION is the most recent version of the MCP protocol.
const LATEST_PROTOCOL_VERSION = "2025-06-18"

// ValidProtocolVersions lists all known valid MCP protocol versions.
var ValidProtocolVersions = []string{
	LATEST_PROTOCOL_VERSION,
	"2025-03-26",
	"2024-11-05",
}

// JSONRPC_VERSION is the version of JSON-RPC used by MCP.
const JSONRPC_VERSION = "2.0"

// ProgressToken is used to associate progress notifications with the original request.
type ProgressToken any

// Cursor is an opaque token used to represent a cursor for pagination.
type Cursor string

// Meta is metadata attached to a request's parameters. This can include fields
// formally defined by the protocol or other arbitrary data.
type Meta struct {
	// If specified, the caller is requesting out-of-band progress
	// notifications for this request (as represented by
	// notifications/progress). The value of this parameter is an
	// opaque token that will be attached to any subsequent
	// notifications. The receiver is not obligated to provide these
	// notifications.
	ProgressToken ProgressToken

	// AdditionalFields are any fields present in the Meta that are not
	// otherwise defined in the protocol.
	AdditionalFields map[string]any
}

func (m *Meta) MarshalJSON() ([]byte, error) {
	raw := make(map[string]any)
	if m.ProgressToken != nil {
		raw["progressToken"] = m.ProgressToken
	}
	maps.Copy(raw, m.AdditionalFields)

	return json.Marshal(raw)
}

func (m *Meta) UnmarshalJSON(data []byte) error {
	raw := make(map[string]any)
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	m.ProgressToken = raw["progressToken"]
	delete(raw, "progressToken")
	m.AdditionalFields = raw
	return nil
}

func NewMetaFromMap(m map[string]any) *Meta {
	progressToken := m["progressToken"]
	if progressToken != nil {
		delete(m, "progressToken")
	}

	return &Meta{
		ProgressToken:    progressToken,
		AdditionalFields: m,
	}
}

type Request struct {
	Method string        `json:"method"`
	Params RequestParams `json:"params,omitempty"`
}

type RequestParams struct {
	Meta *Meta `json:"_meta,omitempty"`
}

type Params map[string]any

type Notification struct {
	Method string             `json:"method"`
	Params NotificationParams `json:"params,omitempty"`
}

type NotificationParams struct {
	// This parameter name is reserved by MCP to allow clients and
	// servers to attach additional metadata to their notifications.
	Meta map[string]any `json:"_meta,omitempty"`

	// Additional fields can be added to this map
	AdditionalFields map[string]any `json:"-"`
}

// MarshalJSON implements custom JSON marshaling
func (p NotificationParams) MarshalJSON() ([]byte, error) {
	// Create a map to hold all fields
	m := make(map[string]any)

	// Add Meta if it exists
	if p.Meta != nil {
		m["_meta"] = p.Meta
	}

	// Add all additional fields
	for k, v := range p.AdditionalFields {
		// Ensure we don't override the _meta field
		if k != "_meta" {
			m[k] = v
		}
	}

	return json.Marshal(m)
}

// UnmarshalJSON implements custom JSON unmarshaling
func (p *NotificationParams) UnmarshalJSON(data []byte) error {
	// Create a map to hold all fields
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	// Initialize maps if they're nil
	if p.Meta == nil {
		p.Meta = make(map[string]any)
	}
	if p.AdditionalFields == nil {
		p.AdditionalFields = make(map[string]any)
	}

	// Process all fields
	for k, v := range m {
		if k == "_meta" {
			// Handle Meta field
			if meta, ok := v.(map[string]any); ok {
				p.Meta = meta
			}
		} else {
			// Handle additional fields
			p.AdditionalFields[k] = v
		}
	}

	return nil
}

type Result struct {
	// This result property is reserved by the protocol to allow clients and
	// servers to attach additional metadata to their responses.
	Meta *Meta `json:"_meta,omitempty"`
}

// RequestId is a uniquely identifying ID for a request in JSON-RPC.
// It can be any JSON-serializable value, typically a number or string.
type RequestId struct {
	value any
}

// NewRequestId creates a new RequestId with the given value
func NewRequestId(value any) RequestId {
	return RequestId{value: value}
}

// Value returns the underlying value of the RequestId
func (r RequestId) Value() any {
	return r.value
}

// String returns a string representation of the RequestId
func (r RequestId) String() string {
	switch v := r.value.(type) {
	case string:
		return "string:" + v
	case int64:
		return "int64:" + strconv.FormatInt(v, 10)
	case float64:
		if v == float64(int64(v)) {
			return "int64:" + strconv.FormatInt(int64(v), 10)
		}
		return "float64:" + strconv.FormatFloat(v, 'f', -1, 64)
	case nil:
		return "<nil>"
	default:
		return "unknown:" + fmt.Sprintf("%v", v)
	}
}

// IsNil returns true if the RequestId is nil
func (r RequestId) IsNil() bool {
	return r.value == nil
}

func (r RequestId) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.value)
}

func (r *RequestId) UnmarshalJSON(data []byte) error {

	if string(data) == "null" {
		r.value = nil
		return nil
	}

	// Try unmarshaling as string first
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		r.value = s
		return nil
	}

	// JSON numbers are unmarshaled as float64 in Go
	var f float64
	if err := json.Unmarshal(data, &f); err == nil {
		if f == float64(int64(f)) {
			r.value = int64(f)
		} else {
			r.value = f
		}
		return nil
	}

	return fmt.Errorf("invalid request id: %s", string(data))
}

// JSONRPCRequest represents a request that expects a response.
type JSONRPCRequest struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      RequestId `json:"id"`
	Params  any       `json:"params,omitempty"`
	Request
}

// JSONRPCNotification represents a notification which does not expect a response.
type JSONRPCNotification struct {
	JSONRPC string `json:"jsonrpc"`
	Notification
}

// JSONRPCResponse represents a successful (non-error) response to a request.
type JSONRPCResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      RequestId `json:"id"`
	Result  any       `json:"result"`
}

// JSONRPCError represents a non-successful (error) response to a request.
type JSONRPCError struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      RequestId `json:"id"`
	Error   struct {
		// The error type that occurred.
		Code int `json:"code"`
		// A short description of the error. The message SHOULD be limited
		// to a concise single sentence.
		Message string `json:"message"`
		// Additional information about the error. The value of this member
		// is defined by the sender (e.g. detailed error information, nested errors etc.).
		Data any `json:"data,omitempty"`
	} `json:"error"`
}

// Standard JSON-RPC error codes
const (
	PARSE_ERROR      = -32700
	INVALID_REQUEST  = -32600
	METHOD_NOT_FOUND = -32601
	INVALID_PARAMS   = -32602
	INTERNAL_ERROR   = -32603
)

// MCP error codes
const (
	RESOURCE_NOT_FOUND = -32002
)

/* Empty result */

// EmptyResult represents a response that indicates success but carries no data.
type EmptyResult Result

/* Cancellation */

// CancelledNotification can be sent by either side to indicate that it is
// cancelling a previously-issued request.
//
// The request SHOULD still be in-flight, but due to communication latency, it
// is always possible that this notification MAY arrive after the request has
// already finished.
//
// This notification indicates that the result will be unused, so any
// associated processing SHOULD cease.
//
// A client MUST NOT attempt to cancel its `initialize` request.
type CancelledNotification struct {
	Notification
	Params CancelledNotificationParams `json:"params"`
}

type CancelledNotificationParams struct {
	// The ID of the request to cancel.
	//
	// This MUST correspond to the ID of a request previously issued
	// in the same direction.
	RequestId RequestId `json:"requestId"`

	// An optional string describing the reason for the cancellation. This MAY
	// be logged or presented to the user.
	Reason string `json:"reason,omitempty"`
}

/* Initialization */

// InitializeRequest is sent from the client to the server when it first
// connects, asking it to begin initialization.
type InitializeRequest struct {
	Request
	Params InitializeParams `json:"params"`
	Header http.Header      `json:"-"`
}

type InitializeParams struct {
	// The latest version of the Model Context Protocol that the client supports.
	// The client MAY decide to support older versions as well.
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ClientCapabilities `json:"capabilities"`
	ClientInfo      Implementation     `json:"clientInfo"`
}

// InitializeResult is sent after receiving an initialize request from the
// client.
type InitializeResult struct {
	Result
	// The version of the Model Context Protocol that the server wants to use.
	// This may not match the version that the client requested. If the client cannot
	// support this version, it MUST disconnect.
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      Implementation     `json:"serverInfo"`
	// Instructions describing how to use the server and its features.
	//
	// This can be used by clients to improve the LLM's understanding of
	// available tools, resources, etc. It can be thought of like a "hint" to the model.
	// For example, this information MAY be added to the system prompt.
	Instructions string `json:"instructions,omitempty"`
}

// InitializedNotification is sent from the client to the server after
// initialization has finished.
type InitializedNotification struct {
	Notification
}

// ClientCapabilities represents capabilities a client may support. Known
// capabilities are defined here, in this schema, but this is not a closed set: any
// client can define its own, additional capabilities.
type ClientCapabilities struct {
	// Experimental, non-standard capabilities that the client supports.
	Experimental map[string]any `json:"experimental,omitempty"`
	// Present if the client supports listing roots.
	Roots *struct {
		// Whether the client supports notifications for changes to the roots list.
		ListChanged bool `json:"listChanged,omitempty"`
	} `json:"roots,omitempty"`
	// Present if the client supports sampling from an LLM.
	Sampling *struct{} `json:"sampling,omitempty"`
}

// ServerCapabilities represents capabilities that a server may support. Known
// capabilities are defined here, in this schema, but this is not a closed set: any
// server can define its own, additional capabilities.
type ServerCapabilities struct {
	// Experimental, non-standard capabilities that the server supports.
	Experimental map[string]any `json:"experimental,omitempty"`
	// Present if the server supports sending log messages to the client.
	Logging *struct{} `json:"logging,omitempty"`
	// Present if the server offers any prompt templates.
	Prompts *struct {
		// Whether this server supports notifications for changes to the prompt list.
		ListChanged bool `json:"listChanged,omitempty"`
	} `json:"prompts,omitempty"`
	// Present if the server offers any resources to read.
	Resources *struct {
		// Whether this server supports subscribing to resource updates.
		Subscribe bool `json:"subscribe,omitempty"`
		// Whether this server supports notifications for changes to the resource
		// list.
		ListChanged bool `json:"listChanged,omitempty"`
	} `json:"resources,omitempty"`
	// Present if the server supports sending sampling requests to clients.
	Sampling *struct{} `json:"sampling,omitempty"`
	// Present if the server offers any tools to call.
	Tools *struct {
		// Whether this server supports notifications for changes to the tool list.
		ListChanged bool `json:"listChanged,omitempty"`
	} `json:"tools,omitempty"`
}

// Implementation describes the name and version of an MCP implementation.
type Implementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

/* Ping */

// PingRequest represents a ping, issued by either the server or the client,
// to check that the other party is still alive. The receiver must promptly respond,
// or else may be disconnected.
type PingRequest struct {
	Request
	Header http.Header `json:"-"`
}

/* Progress notifications */

// ProgressNotification is an out-of-band notification used to inform the
// receiver of a progress update for a long-running request.
type ProgressNotification struct {
	Notification
	Params ProgressNotificationParams `json:"params"`
}

type ProgressNotificationParams struct {
	// The progress token which was given in the initial request, used to
	// associate this notification with the request that is proceeding.
	ProgressToken ProgressToken `json:"progressToken"`
	// The progress thus far. This should increase every time progress is made,
	// even if the total is unknown.
	Progress float64 `json:"progress"`
	// Total number of items to process (or total progress required), if known.
	Total float64 `json:"total,omitempty"`
	// Message related to progress. This should provide relevant human-readable
	// progress information.
	Message string `json:"message,omitempty"`
}

/* Pagination */

type PaginatedRequest struct {
	Request
	Params PaginatedParams `json:"params,omitempty"`
}

type PaginatedParams struct {
	// An opaque token representing the current pagination position.
	// If provided, the server should return results starting after this cursor.
	Cursor Cursor `json:"cursor,omitempty"`
}

type PaginatedResult struct {
	Result
	// An opaque token representing the pagination position after the last
	// returned result.
	// If present, there may be more results available.
	NextCursor Cursor `json:"nextCursor,omitempty"`
}

/* Resources */

// ListResourcesRequest is sent from the client to request a list of resources
// the server has.
type ListResourcesRequest struct {
	PaginatedRequest
	Header http.Header `json:"-"`
}

// ListResourcesResult is the server's response to a resources/list request
// from the client.
type ListResourcesResult struct {
	PaginatedResult
	Resources []Resource `json:"resources"`
}

// ListResourceTemplatesRequest is sent from the client to request a list of
// resource templates the server has.
type ListResourceTemplatesRequest struct {
	PaginatedRequest
	Header http.Header `json:"-"`
}

// ListResourceTemplatesResult is the server's response to a
// resources/templates/list request from the client.
type ListResourceTemplatesResult struct {
	PaginatedResult
	ResourceTemplates []ResourceTemplate `json:"resourceTemplates"`
}

// ReadResourceRequest is sent from the client to the server, to read a
// specific resource URI.
type ReadResourceRequest struct {
	Request
	Header http.Header        `json:"-"`
	Params ReadResourceParams `json:"params"`
}

type ReadResourceParams struct {
	// The URI of the resource to read. The URI can use any protocol; it is up
	// to the server how to interpret it.
	URI string `json:"uri"`
	// Arguments to pass to the resource handler
	Arguments map[string]any `json:"arguments,omitempty"`
}

// ReadResourceResult is the server's response to a resources/read request
// from the client.
type ReadResourceResult struct {
	Result
	Contents []ResourceContents `json:"contents"` // Can be TextResourceContents or BlobResourceContents
}

// ResourceListChangedNotification is an optional notification from the server
// to the client, informing it that the list of resources it can read from has
// changed. This may be issued by servers without any previous subscription from
// the client.
type ResourceListChangedNotification struct {
	Notification
}

// SubscribeRequest is sent from the client to request resources/updated
// notifications from the server whenever a particular resource changes.
type SubscribeRequest struct {
	Request
	Params SubscribeParams `json:"params"`
	Header http.Header     `json:"-"`
}

type SubscribeParams struct {
	// The URI of the resource to subscribe to. The URI can use any protocol; it
	// is up to the server how to interpret it.
	URI string `json:"uri"`
}

// UnsubscribeRequest is sent from the client to request cancellation of
// resources/updated notifications from the server. This should follow a previous
// resources/subscribe request.
type UnsubscribeRequest struct {
	Request
	Params UnsubscribeParams `json:"params"`
	Header http.Header       `json:"-"`
}

type UnsubscribeParams struct {
	// The URI of the resource to unsubscribe from.
	URI string `json:"uri"`
}

// ResourceUpdatedNotification is a notification from the server to the client,
// informing it that a resource has changed and may need to be read again. This
// should only be sent if the client previously sent a resources/subscribe request.
type ResourceUpdatedNotification struct {
	Notification
	Params ResourceUpdatedNotificationParams `json:"params"`
}
type ResourceUpdatedNotificationParams struct {
	// The URI of the resource that has been updated. This might be a sub-
	// resource of the one that the client actually subscribed to.
	URI string `json:"uri"`
}

// Resource represents a known resource that the server is capable of reading.
type Resource struct {
	Annotated
	// Meta is a metadata object that is reserved by MCP for storing additional information.
	Meta *Meta `json:"_meta,omitempty"`
	// The URI of this resource.
	URI string `json:"uri"`
	// A human-readable name for this resource.
	//
	// This can be used by clients to populate UI elements.
	Name string `json:"name"`
	// A description of what this resource represents.
	//
	// This can be used by clients to improve the LLM's understanding of
	// available resources. It can be thought of like a "hint" to the model.
	Description string `json:"description,omitempty"`
	// The MIME type of this resource, if known.
	MIMEType string `json:"mimeType,omitempty"`
}

// GetName returns the name of the resource.
func (r Resource) GetName() string {
	return r.Name
}

// ResourceTemplate represents a template description for resources available
// on the server.
type ResourceTemplate struct {
	Annotated
	// Meta is a metadata object that is reserved by MCP for storing additional information.
	Meta *Meta `json:"_meta,omitempty"`
	// A URI template (according to RFC 6570) that can be used to construct
	// resource URIs.
	URITemplate *URITemplate `json:"uriTemplate"`
	// A human-readable name for the type of resource this template refers to.
	//
	// This can be used by clients to populate UI elements.
	Name string `json:"name"`
	// A description of what this template is for.
	//
	// This can be used by clients to improve the LLM's understanding of
	// available resources. It can be thought of like a "hint" to the model.
	Description string `json:"description,omitempty"`
	// The MIME type for all resources that match this template. This should only
	// be included if all resources matching this template have the same type.
	MIMEType string `json:"mimeType,omitempty"`
}

// GetName returns the name of the resourceTemplate.
func (rt ResourceTemplate) GetName() string {
	return rt.Name
}

// ResourceContents represents the contents of a specific resource or sub-
// resource.
type ResourceContents interface {
	isResourceContents()
}

type TextResourceContents struct {
	// Meta is a metadata object that is reserved by MCP for storing additional information.
	Meta *Meta `json:"_meta,omitempty"`
	// The URI of this resource.
	URI string `json:"uri"`
	// The MIME type of this resource, if known.
	MIMEType string `json:"mimeType,omitempty"`
	// The text of the item. This must only be set if the item can actually be
	// represented as text (not binary data).
	Text string `json:"text"`
}

func (TextResourceContents) isResourceContents() {}

type BlobResourceContents struct {
	// Meta is a metadata object that is reserved by MCP for storing additional information.
	Meta *Meta `json:"_meta,omitempty"`
	// The URI of this resource.
	URI string `json:"uri"`
	// The MIME type of this resource, if known.
	MIMEType string `json:"mimeType,omitempty"`
	// A base64-encoded string representing the binary data of the item.
	Blob string `json:"blob"`
}

func (BlobResourceContents) isResourceContents() {}

/* Logging */

// SetLevelRequest is a request from the client to the server, to enable or
// adjust logging.
type SetLevelRequest struct {
	Request
	Params SetLevelParams `json:"params"`
	Header http.Header    `json:"-"`
}

type SetLevelParams struct {
	// The level of logging that the client wants to receive from the server.
	// The server should send all logs at this level and higher (i.e., more severe) to
	// the client as notifications/logging/message.
	Level LoggingLevel `json:"level"`
}

// LoggingMessageNotification is a notification of a log message passed from
// server to client. If no logging/setLevel request has been sent from the client,
// the server MAY decide which messages to send automatically.
type LoggingMessageNotification struct {
	Notification
	Params LoggingMessageNotificationParams `json:"params"`
}

type LoggingMessageNotificationParams struct {
	// The severity of this log message.
	Level LoggingLevel `json:"level"`
	// An optional name of the logger issuing this message.
	Logger string `json:"logger,omitempty"`
	// The data to be logged, such as a string message or an object. Any JSON
	// serializable type is allowed here.
	Data any `json:"data"`
}

// LoggingLevel represents the severity of a log message.
//
// These map to syslog message severities, as specified in RFC-5424:
// https://datatracker.ietf.org/doc/html/rfc5424#section-6.2.1
type LoggingLevel string

const (
	LoggingLevelDebug     LoggingLevel = "debug"
	LoggingLevelInfo      LoggingLevel = "info"
	LoggingLevelNotice    LoggingLevel = "notice"
	LoggingLevelWarning   LoggingLevel = "warning"
	LoggingLevelError     LoggingLevel = "error"
	LoggingLevelCritical  LoggingLevel = "critical"
	LoggingLevelAlert     LoggingLevel = "alert"
	LoggingLevelEmergency LoggingLevel = "emergency"
)

var levelToInt = map[LoggingLevel]int{
	LoggingLevelDebug:     0,
	LoggingLevelInfo:      1,
	LoggingLevelNotice:    2,
	LoggingLevelWarning:   3,
	LoggingLevelError:     4,
	LoggingLevelCritical:  5,
	LoggingLevelAlert:     6,
	LoggingLevelEmergency: 7,
}

func (l LoggingLevel) ShouldSendTo(minLevel LoggingLevel) bool {
	ia, oka := levelToInt[l]
	ib, okb := levelToInt[minLevel]
	if !oka || !okb {
		return false
	}
	return ia >= ib
}

/* Sampling */

const (
	// MethodSamplingCreateMessage allows servers to request LLM completions from clients
	MethodSamplingCreateMessage MCPMethod = "sampling/createMessage"
)

// CreateMessageRequest is a request from the server to sample an LLM via the
// client. The client has full discretion over which model to select. The client
// should also inform the user before beginning sampling, to allow them to inspect
// the request (human in the loop) and decide whether to approve it.
type CreateMessageRequest struct {
	Request
	CreateMessageParams `json:"params"`
}

type CreateMessageParams struct {
	Messages         []SamplingMessage `json:"messages"`
	ModelPreferences *ModelPreferences `json:"modelPreferences,omitempty"`
	SystemPrompt     string            `json:"systemPrompt,omitempty"`
	IncludeContext   string            `json:"includeContext,omitempty"`
	Temperature      float64           `json:"temperature,omitempty"`
	MaxTokens        int               `json:"maxTokens"`
	StopSequences    []string          `json:"stopSequences,omitempty"`
	Metadata         any               `json:"metadata,omitempty"`
}

// CreateMessageResult is the client's response to a sampling/create_message
// request from the server. The client should inform the user before returning the
// sampled message, to allow them to inspect the response (human in the loop) and
// decide whether to allow the server to see it.
type CreateMessageResult struct {
	Result
	SamplingMessage
	// The name of the model that generated the message.
	Model string `json:"model"`
	// The reason why sampling stopped, if known.
	StopReason string `json:"stopReason,omitempty"`
}

// SamplingMessage describes a message issued to or received from an LLM API.
type SamplingMessage struct {
	Role    Role `json:"role"`
	Content any  `json:"content"` // Can be TextContent, ImageContent or AudioContent
}

type Annotations struct {
	// Describes who the intended customer of this object or data is.
	//
	// It can include multiple entries to indicate content useful for multiple
	// audiences (e.g., `["user", "assistant"]`).
	Audience []Role `json:"audience,omitempty"`

	// Describes how important this data is for operating the server.
	//
	// A value of 1 means "most important," and indicates that the data is
	// effectively required, while 0 means "least important," and indicates that
	// the data is entirely optional.
	Priority float64 `json:"priority,omitempty"`
}

// Annotated is the base for objects that include optional annotations for the
// client. The client can use annotations to inform how objects are used or
// displayed
type Annotated struct {
	Annotations *Annotations `json:"annotations,omitempty"`
}

type Content interface {
	isContent()
}

// TextContent represents text provided to or from an LLM.
// It must have Type set to "text".
type TextContent struct {
	Annotated
	// Meta is a metadata object that is reserved by MCP for storing additional information.
	Meta *Meta  `json:"_meta,omitempty"`
	Type string `json:"type"` // Must be "text"
	// The text content of the message.
	Text string `json:"text"`
}

func (TextContent) isContent() {}

// ImageContent represents an image provided to or from an LLM.
// It must have Type set to "image".
type ImageContent struct {
	Annotated
	// Meta is a metadata object that is reserved by MCP for storing additional information.
	Meta *Meta  `json:"_meta,omitempty"`
	Type string `json:"type"` // Must be "image"
	// The base64-encoded image data.
	Data string `json:"data"`
	// The MIME type of the image. Different providers may support different image types.
	MIMEType string `json:"mimeType"`
}

func (ImageContent) isContent() {}

// AudioContent represents the contents of audio, embedded into a prompt or tool call result.
// It must have Type set to "audio".
type AudioContent struct {
	Annotated
	// Meta is a metadata object that is reserved by MCP for storing additional information.
	Meta *Meta  `json:"_meta,omitempty"`
	Type string `json:"type"` // Must be "audio"
	// The base64-encoded audio data.
	Data string `json:"data"`
	// The MIME type of the audio. Different providers may support different audio types.
	MIMEType string `json:"mimeType"`
}

func (AudioContent) isContent() {}

// ResourceLink represents a link to a resource that the client can access.
type ResourceLink struct {
	Annotated
	Type string `json:"type"` // Must be "resource_link"
	// The URI of the resource.
	URI string `json:"uri"`
	// The name of the resource.
	Name string `json:"name"`
	// The description of the resource.
	Description string `json:"description"`
	// The MIME type of the resource.
	MIMEType string `json:"mimeType"`
}

func (ResourceLink) isContent() {}

// EmbeddedResource represents the contents of a resource, embedded into a prompt or tool call result.
//
// It is up to the client how best to render embedded resources for the
// benefit of the LLM and/or the user.
type EmbeddedResource struct {
	Annotated
	// Meta is a metadata object that is reserved by MCP for storing additional information.
	Meta     *Meta            `json:"_meta,omitempty"`
	Type     string           `json:"type"`
	Resource ResourceContents `json:"resource"`
}

func (EmbeddedResource) isContent() {}

// ModelPreferences represents the server's preferences for model selection,
// requested of the client during sampling.
//
// Because LLMs can vary along multiple dimensions, choosing the "best" modelis
// rarely straightforward.  Different models excel in different areas—some are
// faster but less capable, others are more capable but more expensive, and so
// on. This interface allows servers to express their priorities across multiple
// dimensions to help clients make an appropriate selection for their use case.
//
// These preferences are always advisory. The client MAY ignore them. It is also
// up to the client to decide how to interpret these preferences and how to
// balance them against other considerations.
type ModelPreferences struct {
	// Optional hints to use for model selection.
	//
	// If multiple hints are specified, the client MUST evaluate them in order
	// (such that the first match is taken).
	//
	// The client SHOULD prioritize these hints over the numeric priorities, but
	// MAY still use the priorities to select from ambiguous matches.
	Hints []ModelHint `json:"hints,omitempty"`

	// How much to prioritize cost when selecting a model. A value of 0 means cost
	// is not important, while a value of 1 means cost is the most important
	// factor.
	CostPriority float64 `json:"costPriority,omitempty"`

	// How much to prioritize sampling speed (latency) when selecting a model. A
	// value of 0 means speed is not important, while a value of 1 means speed is
	// the most important factor.
	SpeedPriority float64 `json:"speedPriority,omitempty"`

	// How much to prioritize intelligence and capabilities when selecting a
	// model. A value of 0 means intelligence is not important, while a value of 1
	// means intelligence is the most important factor.
	IntelligencePriority float64 `json:"intelligencePriority,omitempty"`
}

// ModelHint represents hints to use for model selection.
//
// Keys not declared here are currently left unspecified by the spec and are up
// to the client to interpret.
type ModelHint struct {
	// A hint for a model name.
	//
	// The client SHOULD treat this as a substring of a model name; for example:
	//  - `claude-3-5-sonnet` should match `claude-3-5-sonnet-20241022`
	//  - `sonnet` should match `claude-3-5-sonnet-20241022`, `claude-3-sonnet-20240229`, etc.
	//  - `claude` should match any Claude model
	//
	// The client MAY also map the string to a different provider's model name or
	// a different model family, as long as it fills a similar niche; for example:
	//  - `gemini-1.5-flash` could match `claude-3-haiku-20240307`
	Name string `json:"name,omitempty"`
}

/* Autocomplete */

// CompleteRequest is a request from the client to the server, to ask for completion options.
type CompleteRequest struct {
	Request
	Params CompleteParams `json:"params"`
	Header http.Header    `json:"-"`
}

type CompleteParams struct {
	Ref      any `json:"ref"` // Can be PromptReference or ResourceReference
	Argument struct {
		// The name of the argument
		Name string `json:"name"`
		// The value of the argument to use for completion matching.
		Value string `json:"value"`
	} `json:"argument"`
}

// CompleteResult is the server's response to a completion/complete request
type CompleteResult struct {
	Result
	Completion struct {
		// An array of completion values. Must not exceed 100 items.
		Values []string `json:"values"`
		// The total number of completion options available. This can exceed the
		// number of values actually sent in the response.
		Total int `json:"total,omitempty"`
		// Indicates whether there are additional completion options beyond those
		// provided in the current response, even if the exact total is unknown.
		HasMore bool `json:"hasMore,omitempty"`
	} `json:"completion"`
}

// ResourceReference is a reference to a resource or resource template definition.
type ResourceReference struct {
	Type string `json:"type"`
	// The URI or URI template of the resource.
	URI string `json:"uri"`
}

// PromptReference identifies a prompt.
type PromptReference struct {
	Type string `json:"type"`
	// The name of the prompt or prompt template
	Name string `json:"name"`
}

/* Roots */

// ListRootsRequest is sent from the server to request a list of root URIs from the client. Roots allow
// servers to ask for specific directories or files to operate on. A common example
// for roots is providing a set of repositories or directories a server should operate
// on.
//
// This request is typically used when the server needs to understand the file system
// structure or access specific locations that the client has permission to read from.
type ListRootsRequest struct {
	Request
	Header http.Header `json:"-"`
}

// ListRootsResult is the client's response to a roots/list request from the server.
// This result contains an array of Root objects, each representing a root directory
// or file that the server can operate on.
type ListRootsResult struct {
	Result
	Roots []Root `json:"roots"`
}

// Root represents a root directory or file that the server can operate on.
type Root struct {
	// Meta is a metadata object that is reserved by MCP for storing additional information.
	Meta *Meta `json:"_meta,omitempty"`
	// The URI identifying the root. This *must* start with file:// for now.
	// This restriction may be relaxed in future versions of the protocol to allow
	// other URI schemes.
	URI string `json:"uri"`
	// An optional name for the root. This can be used to provide a human-readable
	// identifier for the root, which may be useful for display purposes or for
	// referencing the root in other parts of the application.
	Name string `json:"name,omitempty"`
}

// RootsListChangedNotification is a notification from the client to the
// server, informing it that the list of roots has changed.
// This notification should be sent whenever the client adds, removes, or modifies any root.
// The server should then request an updated list of roots using the ListRootsRequest.
type RootsListChangedNotification struct {
	Notification
}

// ClientRequest represents any request that can be sent from client to server.
type ClientRequest any

// ClientNotification represents any notification that can be sent from client to server.
type ClientNotification any

// ClientResult represents any result that can be sent from client to server.
type ClientResult any

// ServerRequest represents any request that can be sent from server to client.
type ServerRequest any

// ServerNotification represents any notification that can be sent from server to client.
type ServerNotification any

// ServerResult represents any result that can be sent from server to client.
type ServerResult any

type Named interface {
	GetName() string
}

// MarshalJSON implements custom JSON marshaling for Content interface
func MarshalContent(content Content) ([]byte, error) {
	return json.Marshal(content)
}

// UnmarshalContent implements custom JSON unmarshaling for Content interface
func UnmarshalContent(data []byte) (Content, error) {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	contentType, ok := raw["type"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid type field")
	}

	switch contentType {
	case "text":
		var content TextContent
		err := json.Unmarshal(data, &content)
		return content, err
	case "image":
		var content ImageContent
		err := json.Unmarshal(data, &content)
		return content, err
	case "audio":
		var content AudioContent
		err := json.Unmarshal(data, &content)
		return content, err
	case "resource_link":
		var content ResourceLink
		err := json.Unmarshal(data, &content)
		return content, err
	case "resource":
		var content EmbeddedResource
		err := json.Unmarshal(data, &content)
		return content, err
	default:
		return nil, fmt.Errorf("unknown content type: %s", contentType)
	}
}
