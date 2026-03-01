package health

import (
	"os/exec"
	"runtime"
	"time"
)

// MemStats holds memory statistics for the health report.
type MemStats struct {
	AllocMB float64 `json:"alloc_mb"`
	SysMB   float64 `json:"sys_mb"`
	NumGC   uint32  `json:"num_gc"`
}

// HealthReport represents the health check response.
type HealthReport struct {
	Status        string            `json:"status"`
	Version       string            `json:"version"`
	UptimeSeconds float64           `json:"uptime_seconds"`
	Memory        MemStats          `json:"memory"`
	Backends      map[string]string `json:"backends"`
}

// backends is the list of CLI backends agentmux depends on.
var backends = []string{"claude", "gemini"}

// Check builds a HealthReport by inspecting runtime memory, uptime, and backend availability.
func Check(version string, startTime time.Time) HealthReport {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	report := HealthReport{
		Version:       version,
		UptimeSeconds: time.Since(startTime).Seconds(),
		Memory: MemStats{
			AllocMB: float64(m.Alloc) / (1024 * 1024),
			SysMB:   float64(m.Sys) / (1024 * 1024),
			NumGC:   m.NumGC,
		},
		Backends: make(map[string]string, len(backends)),
	}

	allOK := true
	for _, name := range backends {
		if _, err := exec.LookPath(name); err != nil {
			report.Backends[name] = "not found"
			allOK = false
		} else {
			report.Backends[name] = "ok"
		}
	}

	if allOK {
		report.Status = "ok"
	} else {
		report.Status = "degraded"
	}

	return report
}
