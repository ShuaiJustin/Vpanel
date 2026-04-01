package agent

import (
	"net/http"
	"testing"

	"v/internal/logger"
)

func TestMarkPermanentAuthFailureStopsFurtherRetries(t *testing.T) {
	agent := &Agent{logger: logger.NewNopLogger()}

	agent.markPermanentAuthFailure(&PanelHTTPError{
		Operation:  "registration",
		StatusCode: http.StatusUnauthorized,
		Body:       `{"success":false,"message":"Invalid or revoked token"}`,
	})

	if !agent.authFailureStop {
		t.Fatal("expected authFailureStop to be set")
	}
	if agent.authFailureReason == "" {
		t.Fatal("expected authFailureReason to be recorded")
	}
	if agent.registered {
		t.Fatal("expected registered to be false after permanent auth failure")
	}
}
