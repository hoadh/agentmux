package health

import "os/exec"

// BackendStatus represents the availability of a CLI backend.
type BackendStatus struct {
	Name      string `json:"name"`
	Available bool   `json:"available"`
}

// backends is the list of CLI backends agentmux depends on.
var backends = []string{"claude", "gemini"}

// CheckBackends returns availability status for each CLI backend.
func CheckBackends() []BackendStatus {
	results := make([]BackendStatus, 0, len(backends))
	for _, name := range backends {
		_, err := exec.LookPath(name)
		results = append(results, BackendStatus{Name: name, Available: err == nil})
	}
	return results
}
