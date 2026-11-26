package agent

import (
	"net/http"
	"testing"
	"time"

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

func TestAuthFailurePausesThenRetries(t *testing.T) {
	agent := &Agent{logger: logger.NewNopLogger()}
	agent.markPermanentAuthFailure(&PanelHTTPError{Operation: "heartbeat", StatusCode: http.StatusUnauthorized})

	if !agent.authRetryPaused(time.Now().Add(authFailureRetryInterval - time.Second)) {
		t.Fatal("expected the agent to stay quiet right after a 401")
	}
	if agent.authRetryPaused(time.Now().Add(authFailureRetryInterval + time.Second)) {
		t.Fatal("expected the agent to try the panel again after the retry interval")
	}
	if agent.authFailureStop {
		t.Fatal("expected the pause to be lifted so the next heartbeat re-registers")
	}
}
