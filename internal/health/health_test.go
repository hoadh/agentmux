package health

import "testing"

func TestCheckBackends_ReturnsAllBackends(t *testing.T) {
	results := CheckBackends()
	if len(results) != len(backends) {
		t.Fatalf("expected %d backends, got %d", len(backends), len(results))
	}
	for i, b := range results {
		if b.Name != backends[i] {
			t.Errorf("backend %d: got name %q, want %q", i, b.Name, backends[i])
		}
	}
}

func TestCheckBackends_AvailabilityIsBool(t *testing.T) {
	results := CheckBackends()
	for _, b := range results {
		// Available is a bool; just verify it doesn't panic and is set.
		_ = b.Available
	}
}

func TestCheckBackends_KnownBackendFound(t *testing.T) {
	orig := backends
	defer func() { backends = orig }()

	// "sh" is guaranteed to exist on any Unix system.
	backends = []string{"sh"}

	results := CheckBackends()
	if len(results) != 1 {
		t.Fatalf("expected 1 backend, got %d", len(results))
	}
	if !results[0].Available {
		t.Errorf("sh should be available")
	}
}

func TestCheckBackends_MissingBackend(t *testing.T) {
	orig := backends
	defer func() { backends = orig }()

	backends = []string{"this-binary-definitely-does-not-exist-xyz"}

	results := CheckBackends()
	if len(results) != 1 {
		t.Fatalf("expected 1 backend, got %d", len(results))
	}
	if results[0].Available {
		t.Errorf("nonexistent binary should not be available")
	}
}

func TestCheckBackends_EmptyList(t *testing.T) {
	orig := backends
	defer func() { backends = orig }()

	backends = []string{}

	results := CheckBackends()
	if len(results) != 0 {
		t.Errorf("expected 0 backends, got %d", len(results))
	}
}

func TestCheckBackends_MixedAvailability(t *testing.T) {
	orig := backends
	defer func() { backends = orig }()

	backends = []string{"sh", "this-binary-definitely-does-not-exist-xyz"}

	results := CheckBackends()
	if len(results) != 2 {
		t.Fatalf("expected 2 backends, got %d", len(results))
	}
	if !results[0].Available {
		t.Errorf("sh should be available")
	}
	if results[1].Available {
		t.Errorf("nonexistent binary should not be available")
	}
}
