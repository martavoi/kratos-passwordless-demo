package internal

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ory/kratos-client-go/v25"
)

// KratosClient wraps the Ory Kratos API client with optional verbose logging.
type KratosClient struct {
	api     *client.APIClient
	verbose bool
}

// NewKratosClient creates a Kratos client for the given base URL.
func NewKratosClient(baseURL string, verbose bool) *KratosClient {
	cfg := client.NewConfiguration()
	cfg.Servers = []client.ServerConfiguration{
		{URL: baseURL},
	}
	api := client.NewAPIClient(cfg)
	return &KratosClient{api: api, verbose: verbose}
}

// Frontend returns the Frontend API service.
func (c *KratosClient) Frontend() client.FrontendAPI {
	return c.api.FrontendAPI
}



// The Kratos client uses the default HTTP client; we need to wrap it.
// For now we use the client as-is and log in our wrapper functions.
func (c *KratosClient) InstallLogging() {
	// The kratos-client-go doesn't easily allow custom transports without forking.
	// We'll log in our registration/login/session helpers by inspecting the flow responses.
	_ = c
}

// GetCsrfTokenFromLoginFlow extracts the csrf_token from a login flow's UI nodes.
// Kratos requires this token on UpdateLoginFlow even for native/API flows.
func GetCsrfTokenFromLoginFlow(flow *client.LoginFlow) string {
	if flow == nil {
		return ""
	}
	ui := flow.GetUi()
	nodes := ui.GetNodes()
	for _, n := range nodes {
		attr := n.GetAttributes()
		inp, ok := attr.GetActualInstance().(*client.UiNodeInputAttributes)
		if !ok || inp.GetName() != "csrf_token" {
			continue
		}
		if v := inp.GetValue(); v != nil {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}

// GetSessionTokenFromContinueWith extracts ory_session_token from continue_with.
func GetSessionTokenFromContinueWith(continueWith []client.ContinueWith) string {
	for _, cw := range continueWith {
		actual := cw.GetActualInstanceValue()
		if actual == nil {
			continue
		}
		switch v := actual.(type) {
		case *client.ContinueWithSetOrySessionToken:
			return v.GetOrySessionToken()
		case client.ContinueWithSetOrySessionToken:
			return v.GetOrySessionToken()
		}
	}
	return ""
}

// DumpJSON appends a pretty-printed struct to the in-app API log (no stdout/stderr; safe for TUI).
func DumpJSON(label string, v interface{}) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return
	}
	AppendAPILogLine("[" + label + "]")
	for _, line := range strings.Split(string(b), "\n") {
		AppendAPILogLine("  " + line)
	}
}

// LogKratosRequest logs a Kratos API request to the in-app buffer only (no stderr; safe for TUI).
func LogKratosRequest(method string, body interface{}) {
	raw := ""
	if body != nil {
		b, err := json.Marshal(body)
		if err == nil {
			raw = string(b)
		}
	}
	AppendAPILogLine("→ REQUEST " + method)
	if raw != "" {
		AppendAPILogLine("  body: " + raw)
	}
}

// errWithBody is satisfied by the client's GenericOpenAPIError (e.g. on 4xx responses).
type errWithBody interface {
	Body() []byte
}

// LogKratosResponse logs a Kratos API response to the in-app buffer only (no stderr; safe for TUI).
// For 4xx responses the client often returns body in the error; pass err so we log the raw response body when body is nil.
func LogKratosResponse(method string, status int, body interface{}, err error) {
	raw := ""
	if body != nil {
		b, marshalErr := json.Marshal(body)
		if marshalErr == nil {
			raw = string(b)
		}
	}
	// On 4xx the parsed body is often nil; use error's Body() so we still log the response body
	if (raw == "" || raw == "null") && err != nil {
		if e, ok := err.(errWithBody); ok {
			if b := e.Body(); len(b) > 0 {
				// Try to pretty-print JSON for readability
				var out strings.Builder
				if json.Valid(b) {
					var j interface{}
					if json.Unmarshal(b, &j) == nil {
						enc := json.NewEncoder(&out)
						enc.SetIndent("", "  ")
						if enc.Encode(j) == nil {
							raw = strings.TrimSuffix(out.String(), "\n")
						}
					}
				}
				if raw == "" {
					raw = string(b)
				}
			}
		}
	}
	AppendAPILogLine("← RESPONSE " + method + " status=" + fmt.Sprintf("%d", status))
	if raw != "" {
		AppendAPILogLine("  body: " + raw)
	}
}

