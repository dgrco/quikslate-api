package domain

import (
	"testing"
)

func TestValidateEmail(t *testing.T) {
	cases := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{
			name:    "valid email",
			email:   "test@testing.com",
			wantErr: false,
		},
		{
			name:    "valid email with shortest domain postfix",
			email:   "test@testing.so",
			wantErr: false,
		},
		{
			name:    "valid email with multiple dots",
			email:   "test@testing.co.uk",
			wantErr: false,
		},
		{
			name:    "email missing @ symbol",
			email:   "testexample.com",
			wantErr: true,
		},
		{
			name:    "email missing .",
			email:   "test@examplecom",
			wantErr: true,
		},
		{
			name:    "email missing prefix",
			email:   "@example.com",
			wantErr: true,
		},
		{
			name:    "email missing domain",
			email:   "test@",
			wantErr: true,
		},
		{
			name:    "email missing domain prefix",
			email:   "test@.com",
			wantErr: true,
		},
		{
			name:    "email dot prefix too short",
			email:   "test@testing.c",
			wantErr: true,
		},
		{
			name:    "email too long",
			email:   "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA@BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB.com",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateEmail(tc.email)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("email %q should be invalid, got nil", tc.email)
				}
			} else if err != nil {
				t.Fatalf("email %q should be valid, got %v", tc.email, err)
			}
		})
	}
}

func TestValidateUserName(t *testing.T) {
	cases := []struct {
		name     string
		username string
		wantErr  bool
	}{
		{
			name:     "valid name",
			username: "James Dean",
			wantErr:  false,
		},
		{
			name:     "empty name",
			username: " ",
			wantErr:  true,
		},
		{
			name:     "name too long",
			username: "iiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiii",
			wantErr:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateUserName(tc.username)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("name %q should be invalid, got nil", tc.username)
				}
			} else if err != nil {
				t.Fatalf("name %q should be valid, got %v", tc.username, err)
			}
		})
	}
}
