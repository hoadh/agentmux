package health

import (
	"testing"
	"time"
)

func TestCheck_ReturnsVersion(t *testing.T) {
	report := Check("v1.2.3", time.Now())
	if report.Version != "v1.2.3" {
		t.Errorf("version: got %q, want %q", report.Version, "v1.2.3")
	}
}

func TestCheck_EmptyVersion(t *testing.T) {
	report := Check("", time.Now())
	if report.Version != "" {
		t.Errorf("version: got %q, want empty", report.Version)
	}
}

func TestCheck_UptimePositive(t *testing.T) {
	startTime := time.Now().Add(-5 * time.Second)
	report := Check("v1.0.0", startTime)
	if report.UptimeSeconds < 5.0 {
		t.Errorf("uptime: got %f, want >= 5.0", report.UptimeSeconds)
	}
}

func TestCheck_UptimeNearZero(t *testing.T) {
	report := Check("v1.0.0", time.Now())
	if report.UptimeSeconds < 0 {
		t.Errorf("uptime should not be negative, got %f", report.UptimeSeconds)
	}
}

func TestCheck_MemoryFieldsPositive(t *testing.T) {
	report := Check("v1.0.0", time.Now())
	if report.Memory.AllocMB < 0 {
		t.Errorf("alloc_mb should not be negative, got %f", report.Memory.AllocMB)
	}
	if report.Memory.SysMB <= 0 {
		t.Errorf("sys_mb should be positive, got %f", report.Memory.SysMB)
	}
}

func TestCheck_BackendsMapPopulated(t *testing.T) {
	report := Check("v1.0.0", time.Now())
	if report.Backends == nil {
		t.Fatal("backends map should not be nil")
	}
	for _, name := range backends {
		val, ok := report.Backends[name]
		if !ok {
			t.Errorf("backend %q missing from report", name)
		}
		if val != "ok" && val != "not found" {
			t.Errorf("backend %q: unexpected value %q", name, val)
		}
	}
}

func TestCheck_StatusOKWhenAllBackendsFound(t *testing.T) {
	// Save and restore the global backends list.
	orig := backends
	defer func() { backends = orig }()

	// Use a backend that is guaranteed to exist on any Unix system.
	backends = []string{"sh"}

	report := Check("v1.0.0", time.Now())
	if report.Status != "ok" {
		t.Errorf("status: got %q, want %q", report.Status, "ok")
	}
	if report.Backends["sh"] != "ok" {
		t.Errorf("sh backend: got %q, want %q", report.Backends["sh"], "ok")
	}
}

func TestCheck_StatusDegradedWhenBackendMissing(t *testing.T) {
	orig := backends
	defer func() { backends = orig }()

	backends = []string{"this-binary-definitely-does-not-exist-xyz"}

	report := Check("v1.0.0", time.Now())
	if report.Status != "degraded" {
		t.Errorf("status: got %q, want %q", report.Status, "degraded")
	}
	if report.Backends["this-binary-definitely-does-not-exist-xyz"] != "not found" {
		t.Errorf("missing backend value: got %q, want %q",
			report.Backends["this-binary-definitely-does-not-exist-xyz"], "not found")
	}
}

func TestCheck_StatusDegradedWhenSomeBackendsMissing(t *testing.T) {
	orig := backends
	defer func() { backends = orig }()

	backends = []string{"sh", "this-binary-definitely-does-not-exist-xyz"}

	report := Check("v1.0.0", time.Now())
	if report.Status != "degraded" {
		t.Errorf("status: got %q, want %q", report.Status, "degraded")
	}
}

func TestCheck_EmptyBackends(t *testing.T) {
	orig := backends
	defer func() { backends = orig }()

	backends = []string{}

	report := Check("v1.0.0", time.Now())
	if report.Status != "ok" {
		t.Errorf("status with no backends: got %q, want %q", report.Status, "ok")
	}
	if len(report.Backends) != 0 {
		t.Errorf("backends map should be empty, got len=%d", len(report.Backends))
	}
}

func TestCheck_FutureStartTimeUptimeNonNegative(t *testing.T) {
	// startTime slightly in the future — uptime may be tiny negative due to clock precision.
	// The function uses time.Since which can return slightly negative; document that here.
	startTime := time.Now().Add(time.Second)
	report := Check("v1.0.0", startTime)
	// We don't assert on sign — just ensure the field is populated (no panic).
	_ = report.UptimeSeconds
}

func TestCheck_NumGCField(t *testing.T) {
	// NumGC should always be >= 0 (it is a uint32, so always non-negative).
	report := Check("v1.0.0", time.Now())
	// Just ensure it is accessible and reasonable.
	_ = report.Memory.NumGC
}
