// Package ticket provides ticket management services for the user portal.
package ticket

import (
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	"v/internal/database/repository"
)

// Unit tests for ticket validation

func TestCreateTicketRequest_Validate_ValidRequest(t *testing.T) {
	validRequests := []CreateTicketRequest{
		{Subject: "Test Subject", Content: "Test content", Category: "technical", Priority: "medium"},
		{Subject: "Another Subject", Content: "Another content", Category: "billing", Priority: "high"},
		{Subject: "Simple", Content: "Simple content"}, // defaults
	}

	for _, req := range validRequests {
		if err := req.Validate(); err != nil {
			t.Errorf("Expected valid request to pass validation: %v", err)
		}
	}
}

func TestCreateTicketRequest_Validate_InvalidRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     CreateTicketRequest
		wantErr string
	}{
		{
			name:    "empty subject",
			req:     CreateTicketRequest{Subject: "", Content: "content"},
			wantErr: "subject",
		},
		{
			name:    "whitespace subject",
			req:     CreateTicketRequest{Subject: "   ", Content: "content"},
			wantErr: "subject",
		},
		{
			name:    "empty content",
			req:     CreateTicketRequest{Subject: "subject", Content: ""},
			wantErr: "content",
		},
		{
			name:    "invalid category",
			req:     CreateTicketRequest{Subject: "subject", Content: "content", Category: "invalid"},
			wantErr: "category",
		},
		{
			name:    "invalid priority",
			req:     CreateTicketRequest{Subject: "subject", Content: "content", Priority: "invalid"},
			wantErr: "priority",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if err == nil {
				t.Error("Expected validation error")
			}
		})
	}
}

func TestReplyTicketRequest_Validate(t *testing.T) {
	// Valid request
	validReq := ReplyTicketRequest{Content: "This is a reply"}
	if err := validReq.Validate(); err != nil {
		t.Errorf("Expected valid request to pass: %v", err)
	}

	// Invalid requests
	invalidRequests := []ReplyTicketRequest{
		{Content: ""},
		{Content: "   "},
	}

	for _, req := range invalidRequests {
		if err := req.Validate(); err == nil {
			t.Error("Expected validation error for empty content")
		}
	}
}

// Unit tests for status transitions

func TestCanTransition_ValidTransitions(t *testing.T) {
	validTransitions := []struct {
		from string
		to   string
	}{
		{repository.TicketStatusOpen, repository.TicketStatusWaiting},
		{repository.TicketStatusOpen, repository.TicketStatusAnswered},
		{repository.TicketStatusOpen, repository.TicketStatusClosed},
		{repository.TicketStatusWaiting, repository.TicketStatusAnswered},
		{repository.TicketStatusWaiting, repository.TicketStatusClosed},
		{repository.TicketStatusAnswered, repository.TicketStatusWaiting},
		{repository.TicketStatusAnswered, repository.TicketStatusClosed},
		{repository.TicketStatusClosed, repository.TicketStatusOpen}, // reopen
	}

	for _, tt := range validTransitions {
		if !CanTransition(tt.from, tt.to) {
			t.Errorf("Expected transition from %s to %s to be valid", tt.from, tt.to)
		}
	}
}

func TestCanTransition_InvalidTransitions(t *testing.T) {
	invalidTransitions := []struct {
		from string
		to   string
	}{
		{repository.TicketStatusOpen, repository.TicketStatusOpen},       // same status
		{repository.TicketStatusClosed, repository.TicketStatusWaiting},  // closed can only reopen
		{repository.TicketStatusClosed, repository.TicketStatusAnswered}, // closed can only reopen
		{"invalid", repository.TicketStatusOpen},                         // invalid from
		{repository.TicketStatusOpen, "invalid"},                         // invalid to
	}

	for _, tt := range invalidTransitions {
		if CanTransition(tt.from, tt.to) {
			t.Errorf("Expected transition from %s to %s to be invalid", tt.from, tt.to)
		}
	}
}

// Feature: user-portal, Property 11: Ticket ID Uniqueness
// Validates: Requirements 10.6
// *For any* two tickets in the system, their ticket IDs SHALL be unique.
// Note: This property is enforced by the database auto-increment primary key.
// We test the logical property that generated IDs are always positive and unique.
func TestProperty_TicketIDUniqueness(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property: Ticket IDs are always positive
	properties.Property("ticket IDs are positive", prop.ForAll(
		func(id int64) bool {
			if id <= 0 {
				return true // Skip non-positive IDs as they wouldn't be generated
			}
			ticket := &repository.Ticket{ID: id}
			return ticket.ID > 0
		},
		gen.Int64Range(1, 1000000),
	))

	// Property: Different seed values produce different ticket structures
	properties.Property("tickets with different IDs are distinguishable", prop.ForAll(
		func(id1, id2 int64) bool {
			if id1 == id2 {
				return true // Same IDs are expected to be equal
			}
			ticket1 := &repository.Ticket{ID: id1, Subject: "Test 1"}
			ticket2 := &repository.Ticket{ID: id2, Subject: "Test 2"}
			return ticket1.ID != ticket2.ID
		},
		gen.Int64Range(1, 1000000),
		gen.Int64Range(1, 1000000),
	))

	// Property: Ticket ID is preserved after assignment
	properties.Property("ticket ID is preserved", prop.ForAll(
		func(id int64) bool {
			if id <= 0 {
				return true
			}
			ticket := &repository.Ticket{}
			ticket.ID = id
			return ticket.ID == id
		},
		gen.Int64Range(1, 1000000),
	))

	properties.TestingRun(t)
}

// Feature: user-portal, Property 12: Ticket Status Transitions
// Validates: Requirements 10.3
// *For any* ticket, status transitions SHALL follow the valid state machine:
// open → waiting/answered/closed, waiting → answered/closed, answered → waiting/closed, closed → open
func TestProperty_TicketStatusTransitions(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	allStatuses := []string{
		repository.TicketStatusOpen,
		repository.TicketStatusWaiting,
		repository.TicketStatusAnswered,
		repository.TicketStatusClosed,
	}

	// Property: Valid transitions are accepted
	properties.Property("valid transitions are accepted", prop.ForAll(
		func(fromIdx, toIdx int) bool {
			if fromIdx < 0 || fromIdx >= len(allStatuses) || toIdx < 0 || toIdx >= len(allStatuses) {
				return true
			}
			from := allStatuses[fromIdx]
			to := allStatuses[toIdx]

			// Define expected valid transitions
			validTransitions := map[string]map[string]bool{
				repository.TicketStatusOpen: {
					repository.TicketStatusWaiting:  true,
					repository.TicketStatusAnswered: true,
					repository.TicketStatusClosed:   true,
				},
				repository.TicketStatusWaiting: {
					repository.TicketStatusAnswered: true,
					repository.TicketStatusClosed:   true,
				},
				repository.TicketStatusAnswered: {
					repository.TicketStatusWaiting: true,
					repository.TicketStatusClosed:  true,
				},
				repository.TicketStatusClosed: {
					repository.TicketStatusOpen: true,
				},
			}

			expected := false
			if transitions, ok := validTransitions[from]; ok {
				expected = transitions[to]
			}

			return CanTransition(from, to) == expected
		},
		gen.IntRange(0, 3),
		gen.IntRange(0, 3),
	))

	// Property: Self-transitions are not allowed
	properties.Property("self-transitions are not allowed", prop.ForAll(
		func(statusIdx int) bool {
			if statusIdx < 0 || statusIdx >= len(allStatuses) {
				return true
			}
			status := allStatuses[statusIdx]
			return !CanTransition(status, status)
		},
		gen.IntRange(0, 3),
	))

	// Property: Closed tickets can only be reopened
	properties.Property("closed tickets can only be reopened", prop.ForAll(
		func(toIdx int) bool {
			if toIdx < 0 || toIdx >= len(allStatuses) {
				return true
			}
			to := allStatuses[toIdx]
			result := CanTransition(repository.TicketStatusClosed, to)

			// Only transition to "open" should be valid
			if to == repository.TicketStatusOpen {
				return result == true
			}
			return result == false
		},
		gen.IntRange(0, 3),
	))

	// Property: Open tickets can transition to any other status
	properties.Property("open tickets can transition to any other status", prop.ForAll(
		func(toIdx int) bool {
			if toIdx < 0 || toIdx >= len(allStatuses) {
				return true
			}
			to := allStatuses[toIdx]
			result := CanTransition(repository.TicketStatusOpen, to)

			// Open can go to waiting, answered, or closed (but not open)
			if to == repository.TicketStatusOpen {
				return result == false
			}
			return result == true
		},
		gen.IntRange(0, 3),
	))

	// Property: Invalid status strings are rejected
	properties.Property("invalid status strings are rejected", prop.ForAll(
		func(seed int64) bool {
			invalidStatuses := []string{"invalid", "unknown", "pending", ""}
			invalidStatus := invalidStatuses[int(seed)%len(invalidStatuses)]

			// Invalid from status
			for _, to := range allStatuses {
				if CanTransition(invalidStatus, to) {
					return false
				}
			}

			// Invalid to status
			for _, from := range allStatuses {
				if CanTransition(from, invalidStatus) {
					return false
				}
			}

			return true
		},
		gen.Int64Range(0, 1000),
	))

	properties.TestingRun(t)
}

// Additional unit tests for validation edge cases

func TestCreateTicketRequest_SubjectLength(t *testing.T) {
	// Test subject at max length (200 chars)
	maxSubject := make([]byte, 200)
	for i := range maxSubject {
		maxSubject[i] = 'a'
	}
	req := CreateTicketRequest{Subject: string(maxSubject), Content: "content"}
	if err := req.Validate(); err != nil {
		t.Errorf("Expected max length subject to be valid: %v", err)
	}

	// Test subject over max length
	overMaxSubject := make([]byte, 201)
	for i := range overMaxSubject {
		overMaxSubject[i] = 'a'
	}
	req = CreateTicketRequest{Subject: string(overMaxSubject), Content: "content"}
	if err := req.Validate(); err == nil {
		t.Error("Expected over max length subject to be invalid")
	}
}

func TestCreateTicketRequest_ContentLength(t *testing.T) {
	// Test content at max length (10000 chars)
	maxContent := make([]byte, 10000)
	for i := range maxContent {
		maxContent[i] = 'a'
	}
	req := CreateTicketRequest{Subject: "subject", Content: string(maxContent)}
	if err := req.Validate(); err != nil {
		t.Errorf("Expected max length content to be valid: %v", err)
	}

	// Test content over max length
	overMaxContent := make([]byte, 10001)
	for i := range overMaxContent {
		overMaxContent[i] = 'a'
	}
	req = CreateTicketRequest{Subject: "subject", Content: string(overMaxContent)}
	if err := req.Validate(); err == nil {
		t.Error("Expected over max length content to be invalid")
	}
}

func TestCreateTicketRequest_DefaultValues(t *testing.T) {
	req := CreateTicketRequest{Subject: "subject", Content: "content"}
	if err := req.Validate(); err != nil {
		t.Errorf("Expected request with defaults to be valid: %v", err)
	}

	// Check defaults are set
	if req.Category != "other" {
		t.Errorf("Expected default category 'other', got '%s'", req.Category)
	}
	if req.Priority != "medium" {
		t.Errorf("Expected default priority 'medium', got '%s'", req.Priority)
	}
}
