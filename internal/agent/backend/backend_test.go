package backend

import (
	"testing"
)

func TestGetDefault(t *testing.T) {
	b, err := Get("")
	if err != nil {
		t.Fatalf("Get(\"\") error: %v", err)
	}
	if b.Name() != "claude" {
		t.Errorf("Name: got %q, want %q", b.Name(), "claude")
	}
}

func TestGetClaude(t *testing.T) {
	b, err := Get("claude")
	if err != nil {
		t.Fatalf("Get(\"claude\") error: %v", err)
	}
	if b.Name() != "claude" {
		t.Errorf("Name: got %q, want %q", b.Name(), "claude")
	}
}

func TestGetGemini(t *testing.T) {
	b, err := Get("gemini")
	if err != nil {
		t.Fatalf("Get(\"gemini\") error: %v", err)
	}
	if b.Name() != "gemini" {
		t.Errorf("Name: got %q, want %q", b.Name(), "gemini")
	}
}

func TestGetUnknown(t *testing.T) {
	_, err := Get("unknown")
	if err == nil {
		t.Fatal("expected error for unknown backend")
	}
}

func TestList(t *testing.T) {
	names := List()
	has := map[string]bool{}
	for _, n := range names {
		has[n] = true
	}
	if !has["claude"] {
		t.Error("List() missing claude")
	}
	if !has["gemini"] {
		t.Error("List() missing gemini")
	}
}
