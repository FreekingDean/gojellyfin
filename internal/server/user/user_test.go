package user

import (
	"context"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

func TestServer_ForgotPassword(t *testing.T) {
	server := New(nil, nil)

	response, err := server.ForgotPassword(context.Background(), api.ForgotPasswordRequestObject{
		JSONBody: &api.ForgotPasswordJSONRequestBody{EnteredUsername: "Dean"},
	})
	if err != nil {
		t.Fatalf("ForgotPassword returned %v", err)
	}

	result, ok := response.(api.ForgotPassword200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want api.ForgotPassword200JSONResponse", response)
	}
	if *result.Action != api.ContactAdmin {
		t.Errorf("Action = %v, want %v", *result.Action, api.ContactAdmin)
	}
	if result.PinFile != nil || result.PinExpirationDate != nil {
		t.Errorf("PinFile = %v, PinExpirationDate = %v, want both unset", result.PinFile, result.PinExpirationDate)
	}
}

func TestServer_ForgotPasswordPin(t *testing.T) {
	server := New(nil, nil)

	response, err := server.ForgotPasswordPin(context.Background(), api.ForgotPasswordPinRequestObject{
		JSONBody: &api.ForgotPasswordPinJSONRequestBody{Pin: "0000"},
	})
	if err != nil {
		t.Fatalf("ForgotPasswordPin returned %v", err)
	}

	result, ok := response.(api.ForgotPasswordPin200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want api.ForgotPasswordPin200JSONResponse", response)
	}
	if *result.Success {
		t.Error("Success = true, want false")
	}
	if len(*result.UsersReset) != 0 {
		t.Errorf("UsersReset = %v, want empty", *result.UsersReset)
	}
}
