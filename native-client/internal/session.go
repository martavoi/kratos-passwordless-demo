package internal

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ory/kratos-client-go/v25"
)

// WhoAmI calls the ToSession endpoint with the session token and returns the session/identity.
func WhoAmI(ctx context.Context, k *KratosClient, sessionToken string) (*client.Session, error) {
	LogKratosRequest("ToSession", map[string]string{"X-Session-Token": "present"})
	session, resp, err := k.Frontend().ToSession(ctx).
		XSessionToken(sessionToken).
		Execute()
	status := 0
	if resp != nil {
		status = resp.StatusCode
	}
	LogKratosResponse("ToSession", status, session, err)

	if err != nil {
		return nil, formatAPIError(err, resp)
	}
	if k.verbose {
		DumpJSON("ToSession (whoami) response", session)
	}
	return session, nil
}

// FormatSession returns a human-readable summary of the session.
func FormatSession(s *client.Session) string {
	if s == nil {
		return ""
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Sprintf("%+v", s)
	}
	return string(b)
}

var _ = json.Marshal
