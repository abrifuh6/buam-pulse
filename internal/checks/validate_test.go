package checks

import "testing"

func TestValidateTargetRejectsPrivate(t *testing.T) {
	bad := []string{
		"http://localhost:8080",
		"http://127.0.0.1",
		"http://10.0.0.5/health",
		"http://192.168.1.1",
		"http://172.16.0.1",
		"http://169.254.169.254/latest/meta-data/", // cloud metadata
		"http://[::1]/",
		"http://100.100.100.100/", // CGNAT
		"http://api.internal/health",
		"www.example.com", // no scheme
		"ftp://example.com",
	}
	for _, target := range bad {
		if err := ValidateTarget("http", target); err == nil {
			t.Errorf("expected rejection for %q", target)
		}
	}
}

func TestValidateTargetAcceptsPublic(t *testing.T) {
	good := []string{
		"https://example.com",
		"http://example.com:8080/health",
		"https://api.github.com/status",
		"https://8.8.8.8/",
	}
	for _, target := range good {
		if err := ValidateTarget("http", target); err != nil {
			t.Errorf("expected %q to be accepted, got %v", target, err)
		}
	}
}

func TestValidateTCPTargets(t *testing.T) {
	if err := ValidateTarget("tcp", "example.com:5432"); err != nil {
		t.Errorf("expected public tcp target accepted: %v", err)
	}
	if err := ValidateTarget("tcp", "127.0.0.1:5432"); err == nil {
		t.Error("expected loopback tcp target rejected")
	}
	if err := ValidateTarget("tcp", "example.com"); err == nil {
		t.Error("expected missing port rejected")
	}
}
