package internal

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ory/kratos-client-go/v25"
)

// LoginState holds state for the two-step login flow.
type LoginState struct {
	FlowID     string
	CsrfToken  string // from flow UI; required by Kratos on UpdateLoginFlow when present
	Identifier string // phone (or email) from step 1; must be re-sent when submitting code
}

// CreateLoginFlow creates a native login flow and returns state for step 1.
func CreateLoginFlow(ctx context.Context, k *KratosClient) (*LoginState, error) {
	LogKratosRequest("CreateNativeLoginFlow", nil)
	flow, resp, err := k.Frontend().CreateNativeLoginFlow(ctx).Execute()
	status := 0
	if resp != nil {
		status = resp.StatusCode
	}
	LogKratosResponse("CreateNativeLoginFlow", status, flow, err)
	if err != nil {
		return nil, formatAPIError(err, resp)
	}
	if k.verbose {
		DumpJSON("CreateLoginFlow response", flow)
	}
	csrf := GetCsrfTokenFromLoginFlow(flow)
	return &LoginState{
		FlowID:    flow.GetId(),
		CsrfToken: csrf,
	}, nil
}

// SubmitLoginPhone submits the phone number (step 1). After this, SMS is sent.
func SubmitLoginPhone(ctx context.Context, k *KratosClient, state *LoginState, phone string) error {
	state.Identifier = phone // required when submitting code in step 2
	body := client.NewUpdateLoginFlowWithCodeMethod(state.CsrfToken, "code")
	body.SetIdentifier(phone)

	LogKratosRequest("UpdateLoginFlow(phone)", body)
	result, resp, err := k.Frontend().UpdateLoginFlow(ctx).
		Flow(state.FlowID).
		UpdateLoginFlowBody(client.UpdateLoginFlowWithCodeMethodAsUpdateLoginFlowBody(body)).
		Execute()
	status := 0
	if resp != nil {
		status = resp.StatusCode
	}
	LogKratosResponse("UpdateLoginFlow(phone)", status, result, err)

	if err != nil {
		// 422 or 400 = SMS sent, proceed to code step
		// Kratos may return 400 for code flow even when successful (issue #4432)
		if resp != nil && (resp.StatusCode == 422 || resp.StatusCode == 400) {
			return nil
		}
		return formatAPIError(err, resp)
	}
	if k.verbose {
		DumpJSON("SubmitLoginPhone response", result)
	}
	if tok, ok := result.GetSessionTokenOk(); ok && tok != nil && *tok != "" {
		return nil
	}
	if cw, ok := result.GetContinueWithOk(); ok && len(cw) > 0 {
		if GetSessionTokenFromContinueWith(cw) != "" {
			return nil
		}
	}
	return nil
}

// SubmitLoginCode submits the OTP code (step 2). Returns session token on success.
// Kratos requires the identifier (phone) from step 1 to be sent again.
func SubmitLoginCode(ctx context.Context, k *KratosClient, state *LoginState, code string) (string, error) {
	body := client.NewUpdateLoginFlowWithCodeMethod(state.CsrfToken, "code")
	body.SetCode(code)
	if state.Identifier != "" {
		body.SetIdentifier(state.Identifier)
	}

	LogKratosRequest("UpdateLoginFlow(code)", body)
	result, resp, err := k.Frontend().UpdateLoginFlow(ctx).
		Flow(state.FlowID).
		UpdateLoginFlowBody(client.UpdateLoginFlowWithCodeMethodAsUpdateLoginFlowBody(body)).
		Execute()
	status := 0
	if resp != nil {
		status = resp.StatusCode
	}
	LogKratosResponse("UpdateLoginFlow(code)", status, result, err)

	if err != nil {
		// Known Kratos bug #4432: can return 400 even when code was accepted and session created
		if resp != nil && (resp.StatusCode == 400 || resp.StatusCode == 422) {
			if result != nil {
				if tok, ok := result.GetSessionTokenOk(); ok && tok != nil && *tok != "" {
					return *tok, nil
				}
				if cw, ok := result.GetContinueWithOk(); ok {
					if t := GetSessionTokenFromContinueWith(cw); t != "" {
						return t, nil
					}
				}
			}
		}
		return "", formatAPIError(err, resp)
	}
	if k.verbose {
		DumpJSON("SubmitLoginCode response", result)
	}
	if tok, ok := result.GetSessionTokenOk(); ok && tok != nil && *tok != "" {
		return *tok, nil
	}
	if cw, ok := result.GetContinueWithOk(); ok {
		if t := GetSessionTokenFromContinueWith(cw); t != "" {
			return t, nil
		}
	}
	return "", fmt.Errorf("no session token in login response")
}

var _ = json.Marshal
