package domain

import (
	"errors"
	"testing"
	"time"
)

// Tests for the shift validation rules. These functions are the only thing
// standing between a malformed request and either a Postgres enum error or a
// row that contradicts itself, such as a shift marked "assigned" to nobody.
//
// Every case asserts the *kind* of error, not merely that one came back.
// handleServiceError (internal/handler/errors.go) turns a *ValidationError
// or a known sentinel into a 400 carrying a useful message, and anything
// else into a logged 500. A validator that rejected bad input with a plain
// errors.New would still "return an error" while showing the user an
// internal server error, so returning the right type is the actual contract.

func strPtr(s string) *string { return &s }

// requireClientFacing fails unless err is one handleServiceError can turn
// into a 4xx with a message, rather than falling through to its 500 case.
func requireClientFacing(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if _, ok := errors.AsType[*ValidationError](err); ok {
		return
	}
	// The sentinels below are the ones handleServiceError maps explicitly.
	if errors.Is(err, ErrInvalidShiftTimes) {
		return
	}
	t.Errorf("error %v is neither a *ValidationError nor a mapped sentinel, so it would reach the client as a 500", err)
}

func TestValidateShiftTimes(t *testing.T) {
	base := time.Date(2026, 3, 14, 9, 0, 0, 0, time.UTC)

	cases := []struct {
		name    string
		start   time.Time
		end     time.Time
		wantErr bool
	}{
		{"start before end", base, base.Add(8 * time.Hour), false},
		{"one nanosecond long is still a range", base, base.Add(time.Nanosecond), false},
		// Equal times mean a zero-length shift. Before() is strict, so this
		// is rejected, which is what keeps a shift from occupying no time.
		{"start equals end", base, base, true},
		{"start after end", base.Add(time.Hour), base, true},
		// A shift crossing midnight is ordinary for overnight work, not an
		// error. Only ordering matters here, never wall-clock appearance.
		{"overnight shift crossing midnight", base.Add(14 * time.Hour), base.Add(22 * time.Hour), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateShiftTimes(tc.start, tc.end)
			if tc.wantErr {
				// This one returns a sentinel rather than a ValidationError,
				// because the handler has a dedicated case for it.
				if !errors.Is(err, ErrInvalidShiftTimes) {
					t.Errorf("ValidateShiftTimes() error = %v, want ErrInvalidShiftTimes", err)
				}
				requireClientFacing(t, err)
			} else if err != nil {
				t.Errorf("ValidateShiftTimes() error = %v, want nil", err)
			}
		})
	}
}

func TestValidateShiftStatus(t *testing.T) {
	cases := []struct {
		name    string
		status  ShiftStatus
		wantErr bool
	}{
		{"draft", Draft, false},
		{"assigned", Assigned, false},
		{"uncovered", Uncovered, false},
		{"covered", Covered, false},
		{"cancelled", Cancelled, false},

		{"empty string", ShiftStatus(""), true},
		{"unknown value", ShiftStatus("pending"), true},
		// The Postgres enum is case sensitive, so accepting this here would
		// only move the failure to the driver and turn a 400 into a 500.
		{"correct value in the wrong case", ShiftStatus("Draft"), true},
		{"value with surrounding whitespace", ShiftStatus(" draft "), true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateShiftStatus(tc.status)
			if tc.wantErr {
				requireClientFacing(t, err)
			} else if err != nil {
				t.Errorf("ValidateShiftStatus(%q) error = %v, want nil", tc.status, err)
			}
		})
	}
}

// The create path is where status and assignee can contradict each other,
// so the rules are about coherence between the two rather than either alone.
func TestValidateShiftCreateStatus(t *testing.T) {
	cases := []struct {
		name    string
		status  ShiftStatus
		userId  *string
		wantErr bool
	}{
		// Draft is deliberately permissive either way: sketching a schedule
		// with tentative pre-assignments is a real workflow.
		{"draft with no assignee", Draft, nil, false},
		{"draft with an assignee", Draft, strPtr("user-1"), false},

		{"assigned with an assignee", Assigned, strPtr("user-1"), false},
		// The contradiction this function exists to prevent: a row claiming
		// to be assigned to nobody.
		{"assigned with no assignee", Assigned, nil, true},

		{"uncovered with no assignee", Uncovered, nil, false},
		// Uncovered means the slot is open, so naming someone contradicts it
		// just as directly, in the opposite direction.
		{"uncovered with an assignee", Uncovered, strPtr("user-1"), true},

		// Covered and Cancelled both describe something that happened to a
		// shift that already exists, and each has its own endpoint. Allowing
		// them at creation would let a client set the label without ever
		// performing the transition.
		{"covered with an assignee", Covered, strPtr("user-1"), true},
		{"covered with no assignee", Covered, nil, true},
		{"cancelled with an assignee", Cancelled, strPtr("user-1"), true},
		{"cancelled with no assignee", Cancelled, nil, true},

		{"unknown status", ShiftStatus("nonsense"), nil, true},
		{"empty status", ShiftStatus(""), nil, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateShiftCreateStatus(tc.status, tc.userId)
			if tc.wantErr {
				requireClientFacing(t, err)
			} else if err != nil {
				t.Errorf("ValidateShiftCreateStatus(%q) error = %v, want nil", tc.status, err)
			}
		})
	}
}

// The generic PATCH must not be able to reach the two statuses that have
// dedicated transition endpoints, or a client could set the label without
// performing the transition, e.g. marking a shift assigned while leaving
// user_id NULL.
func TestValidateShiftUpdateStatus(t *testing.T) {
	cases := []struct {
		name    string
		status  ShiftStatus
		wantErr bool
	}{
		{"draft", Draft, false},
		{"uncovered", Uncovered, false},
		{"covered", Covered, false},

		{"assigned belongs to the assign endpoint", Assigned, true},
		{"cancelled belongs to the cancel endpoint", Cancelled, true},

		{"unknown status", ShiftStatus("nonsense"), true},
		{"empty status", ShiftStatus(""), true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateShiftUpdateStatus(tc.status)
			if tc.wantErr {
				requireClientFacing(t, err)
			} else if err != nil {
				t.Errorf("ValidateShiftUpdateStatus(%q) error = %v, want nil", tc.status, err)
			}
		})
	}
}

// ValidateShiftRange bounds how much a single listing query can ask for. The
// boundary cases are the point: the check is `>` MaxShiftRange, so exactly
// the maximum is allowed and anything past it is not.
func TestValidateShiftRange(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name    string
		from    time.Time
		to      time.Time
		wantErr bool
	}{
		{"a single week", from, from.Add(7 * 24 * time.Hour), false},
		{"exactly the maximum range", from, from.Add(MaxShiftRange), false},
		// One nanosecond past the limit. Written relative to MaxShiftRange
		// so the case follows the constant if the limit is ever retuned,
		// while still pinning that the comparison stays exclusive.
		{"one nanosecond past the maximum", from, from.Add(MaxShiftRange + 1), true},
		{"far past the maximum", from, from.Add(50 * 365 * 24 * time.Hour), true},

		{"empty range", from, from, true},
		{"reversed range", from.Add(24 * time.Hour), from, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateShiftRange(tc.from, tc.to)
			if tc.wantErr {
				requireClientFacing(t, err)
			} else if err != nil {
				t.Errorf("ValidateShiftRange() error = %v, want nil", err)
			}
		})
	}
}
