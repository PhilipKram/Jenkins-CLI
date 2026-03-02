package oauth

import (
	"testing"
)

func TestGenerateVerifier(t *testing.T) {
	v1, err := generateVerifier()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(v1) < 43 {
		t.Errorf("verifier too short: %d chars", len(v1))
	}

	// Each call should produce a unique value
	v2, err := generateVerifier()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v1 == v2 {
		t.Error("two calls to generateVerifier returned the same value")
	}
}

func TestChallengeFromVerifier(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	challenge := challengeFromVerifier(verifier)

	if challenge == "" {
		t.Error("challenge should not be empty")
	}
	if challenge == verifier {
		t.Error("challenge should differ from verifier (S256)")
	}

	// Same verifier should produce the same challenge
	challenge2 := challengeFromVerifier(verifier)
	if challenge != challenge2 {
		t.Error("same verifier should produce same challenge")
	}
}

func TestGenerateState(t *testing.T) {
	s1, err := generateState()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s1) < 16 {
		t.Errorf("state too short: %d chars", len(s1))
	}

	s2, err := generateState()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s1 == s2 {
		t.Error("two calls to generateState returned the same value")
	}
}
