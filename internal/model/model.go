// Package model defines the plain data structures shared by every layer of
// Volt (httpcore, store, and the platform bindings). The types mirror the
// frontend's TypeScript data models so a request, collection, environment, or
// history entry serializes to identical JSON on desktop and Android.
package model

// Method is an HTTP verb supported by the request editor.
type Method = string

// BodyType identifies how a request body is encoded.
type BodyType = string

// AuthType identifies the authorization scheme applied to a request.
type AuthType = string

// ApiKeyLocation identifies where an API key is placed on a request.
type ApiKeyLocation = string

// Supported Method values.
const (
	MethodGet     Method = "GET"
	MethodPost    Method = "POST"
	MethodPut     Method = "PUT"
	MethodPatch   Method = "PATCH"
	MethodDelete  Method = "DELETE"
	MethodHead    Method = "HEAD"
	MethodOptions Method = "OPTIONS"
)

// Supported BodyType values.
const (
	BodyNone       BodyType = "none"
	BodyJSON       BodyType = "json"
	BodyText       BodyType = "text"
	BodyFormData   BodyType = "form-data"
	BodyURLEncoded BodyType = "urlencoded"
)

// Supported AuthType values.
const (
	AuthNone   AuthType = "none"
	AuthBearer AuthType = "bearer"
	AuthBasic  AuthType = "basic"
	AuthAPIKey AuthType = "apikey"
)

// Supported ApiKeyLocation values.
const (
	APIKeyInHeader ApiKeyLocation = "header"
	APIKeyInQuery  ApiKeyLocation = "query"
)

// KeyValue is a generic enabled key/value pair used for query parameters,
// headers, and form fields.
type KeyValue struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Enabled bool   `json:"enabled"`
}

// BodySpec describes the request body and how it should be encoded.
type BodySpec struct {
	Type       BodyType   `json:"type"`
	Raw        string     `json:"raw"`        // json/text content
	FormFields []KeyValue `json:"formFields"` // form-data / urlencoded
}

// AuthSpec describes the authorization configuration for a request.
type AuthSpec struct {
	Type           AuthType       `json:"type"`
	BearerToken    string         `json:"bearerToken"`
	BasicUser      string         `json:"basicUser"`
	BasicPass      string         `json:"basicPass"`
	APIKeyName     string         `json:"apiKeyName"`
	APIKeyValue    string         `json:"apiKeyValue"`
	APIKeyLocation ApiKeyLocation `json:"apiKeyLocation"`
}

// RawRequest is a fully-configured but unsaved request as built in the editor.
type RawRequest struct {
	Method  Method     `json:"method"`
	URL     string     `json:"url"`
	Params  []KeyValue `json:"params"`
	Headers []KeyValue `json:"headers"`
	Body    BodySpec   `json:"body"`
	Auth    AuthSpec   `json:"auth"`
}

// Request is the canonical name used by httpcore for a raw, in-editor request.
type Request = RawRequest

// SavedRequest is a RawRequest persisted within a Collection or Folder.
// Name must be 1..255 characters.
type SavedRequest struct {
	RawRequest
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Folder is a nestable container for SavedRequests and other Folders.
// Nesting is bounded at 10 levels deep.
type Folder struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Folders  []Folder       `json:"folders"`
	Requests []SavedRequest `json:"requests"`
}

// Collection is a named, ordered grouping of SavedRequests and Folders.
type Collection struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Folders  []Folder       `json:"folders"`
	Requests []SavedRequest `json:"requests"`
	Order    int            `json:"order"`
}

// Variable is a named value referenced in requests via the {{name}} syntax.
// Name must be 1..128 characters and unique within its Environment; value may
// be 0..4096 characters.
type Variable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Environment is a named set of Variables. Name must be 1..64 characters and
// unique among Environments. At most one Environment is active at a time.
type Environment struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Variables []Variable `json:"variables"`
	Active    bool       `json:"active"`
}

// HistoryEntry is a persisted record of an executed request. Error is "" on a
// successful execution.
type HistoryEntry struct {
	ID         string     `json:"id"`
	Method     Method     `json:"method"`
	URL        string     `json:"url"`
	Status     int        `json:"status"`
	DurationMs int64      `json:"durationMs"`
	At         int64      `json:"at"`
	Error      string     `json:"error"`
	Request    RawRequest `json:"request"`
}

// Settings holds user-configurable application preferences. Defaults are
// System theme, TLS verification enabled, and a 30-second timeout.
type Settings struct {
	Theme          string `json:"theme"`          // "light" | "dark" | "system"
	TLSVerify      bool   `json:"tlsVerify"`      // default true
	TimeoutSeconds int    `json:"timeoutSeconds"` // 1..600, default 30
	ProxyURL       string `json:"proxyUrl"`       // stretch
}

// HTTPResponse is returned to the caller after executing a request. Truncated
// is true when the body exceeds 5 MB (5,242,880 bytes).
type HTTPResponse struct {
	Status     int        `json:"status"`
	StatusText string     `json:"statusText"`
	Headers    []KeyValue `json:"headers"`
	Body       string     `json:"body"`
	DurationMs int64      `json:"durationMs"`
	SizeBytes  int        `json:"sizeBytes"`
	Error      string     `json:"error"`
	Truncated  bool       `json:"truncated"`
}
