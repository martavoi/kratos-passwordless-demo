package internal

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ory/kratos-client-go/v25"
)

// RegistrationState holds state for the two-step registration flow.
// CSRF token is not required for native/API flows.
type RegistrationState struct {
	FlowID string
	Traits map[string]interface{}
}

// CreateRegistrationFlow creates a native registration flow and returns state for step 1.
func CreateRegistrationFlow(ctx context.Context, k *KratosClient) (*RegistrationState, error) {
	LogKratosRequest("CreateNativeRegistrationFlow", nil)
	flow, resp, err := k.Frontend().CreateNativeRegistrationFlow(ctx).Execute()
	status := 0
	if resp != nil {
		status = resp.StatusCode
	}
	LogKratosResponse("CreateNativeRegistrationFlow", status, flow, err)
	if err != nil {
		return nil, formatAPIError(err, resp)
	}
	if k.verbose {
		DumpJSON("CreateRegistrationFlow response", flow)
	}
	return &RegistrationState{
		FlowID: flow.GetId(),
		Traits:    map[string]interface{}{"phone": ""},
	}, nil
}

// SubmitRegistrationPhone submits the phone number (step 1). After this, SMS is sent.
// Kratos may return 422 with updated flow when code input is required - we treat that as success.
func SubmitRegistrationPhone(ctx context.Context, k *KratosClient, state *RegistrationState, phone string) error {
	state.Traits["phone"] = phone
	body := client.NewUpdateRegistrationFlowWithCodeMethod("code", state.Traits)

	LogKratosRequest("UpdateRegistrationFlow(phone)", body)
	result, resp, err := k.Frontend().UpdateRegistrationFlow(ctx).
		Flow(state.FlowID).
		UpdateRegistrationFlowBody(client.UpdateRegistrationFlowWithCodeMethodAsUpdateRegistrationFlowBody(body)).
		Execute()
	status := 0
	if resp != nil {
		status = resp.StatusCode
	}
	LogKratosResponse("UpdateRegistrationFlow(phone)", status, result, err)

	if err != nil {
		// 422 or 400 = flow needs code input (SMS was sent)
		// Known Kratos bug #4432: returns 400 even when successful for code flow
		if resp != nil && (resp.StatusCode == 422 || resp.StatusCode == 400) {
			return nil // SMS sent, proceed to code step
		}
		return formatAPIError(err, resp)
	}
	if k.verbose {
		DumpJSON("SubmitRegistrationPhone response", result)
	}
	// 200 with session = one-step completion (unusual)
	if tok, ok := result.GetSessionTokenOk(); ok && tok != nil && *tok != "" {
		return nil
	}
	if cw, ok := result.GetContinueWithOk(); ok && len(cw) > 0 {
		if t := GetSessionTokenFromContinueWith(cw); t != "" {
			return nil
		}
	}
	return nil
}

// SubmitRegistrationCode submits the OTP code (step 2). Returns session token on success.
func SubmitRegistrationCode(ctx context.Context, k *KratosClient, state *RegistrationState, code string) (string, error) {
	body := client.NewUpdateRegistrationFlowWithCodeMethod("code", state.Traits)
	body.SetCode(code)

	LogKratosRequest("UpdateRegistrationFlow(code)", body)
	result, resp, err := k.Frontend().UpdateRegistrationFlow(ctx).
		Flow(state.FlowID).
		UpdateRegistrationFlowBody(client.UpdateRegistrationFlowWithCodeMethodAsUpdateRegistrationFlowBody(body)).
		Execute()
	status := 0
	if resp != nil {
		status = resp.StatusCode
	}
	LogKratosResponse("UpdateRegistrationFlow(code)", status, result, err)

	if err != nil {
		// Known Kratos bug #4432: can return 400 even when code was accepted and session created
		if resp != nil && (resp.StatusCode == 400 || resp.StatusCode == 422) {
			// Try to extract session from response in case client still unmarshalled it
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
		DumpJSON("SubmitRegistrationCode response", result)
	}

	// SessionToken can be in result directly or in continue_with
	if tok, ok := result.GetSessionTokenOk(); ok && tok != nil && *tok != "" {
		return *tok, nil
	}
	if cw, ok := result.GetContinueWithOk(); ok {
		if t := GetSessionTokenFromContinueWith(cw); t != "" {
			return t, nil
		}
	}
	return "", fmt.Errorf("no session token in registration response")
}

func formatAPIError(err error, _ interface{}) error {
	return err
}

var _ = json.Marshal
