package web

import "testing"

func TestHostAllowedLocksLoopbackBind(t *testing.T) {
	for _, host := range []string{"localhost:8787", "127.0.0.1:8787", "[::1]:8787"} {
		if !hostAllowed(host, "127.0.0.1:8787") {
			t.Fatalf("loopback Host %q should be allowed for loopback bind", host)
		}
	}
	for _, host := range []string{"evil.example:8787", "192.168.1.50:8787", ""} {
		if hostAllowed(host, "127.0.0.1:8787") {
			t.Fatalf("non-local Host %q should be rejected for loopback bind", host)
		}
	}
}

func TestPublicBindRequiresExplicitOptIn(t *testing.T) {
	if err := checkPublicBind("127.0.0.1:8787", false); err != nil {
		t.Fatalf("loopback bind should not require opt-in: %v", err)
	}
	if err := checkPublicBind("0.0.0.0:8787", false); err == nil {
		t.Fatal("public bind without opt-in should be refused")
	}
	if err := checkPublicBind("0.0.0.0:8787", true); err != nil {
		t.Fatalf("public bind with explicit opt-in should be allowed: %v", err)
	}
}

func TestTryStartJobSerializesBackgroundJobs(t *testing.T) {
	s := &server{}
	if !s.tryStartJob() {
		t.Fatal("first job should start")
	}
	if s.tryStartJob() {
		t.Fatal("second concurrent job should be rejected")
	}
	s.endJob()
	if !s.tryStartJob() {
		t.Fatal("job should start again after endJob")
	}
}
