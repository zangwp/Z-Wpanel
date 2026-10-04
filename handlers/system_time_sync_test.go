package handlers

import (
	"context"
	"strings"
	"testing"
)

func TestTimeSyncKeepsExistingProvider(t *testing.T) {
	oldUnit, oldCommand := ntpTimeSyncUnit, timeSyncCommand
	t.Cleanup(func() { ntpTimeSyncUnit, timeSyncCommand = oldUnit, oldCommand })
	ntpTimeSyncUnit = func() string { return "chrony.service" }
	var calls []string
	timeSyncCommand = func(_ context.Context, name string, args ...string) ([]byte, error) {
		calls = append(calls, name+" "+strings.Join(args, " "))
		return nil, nil
	}
	if err := StartSystemTimeSync(); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(calls, "\n")
	if strings.Contains(joined, "apt-get") || !strings.Contains(joined, "enable --now chrony.service") || !strings.Contains(joined, "restart chrony.service") {
		t.Fatalf("existing time provider was replaced or not started: %v", calls)
	}
}

func TestTimeSyncDoesNotReplaceMaskedProvider(t *testing.T) {
	oldUnit, oldCommand := ntpTimeSyncUnit, timeSyncCommand
	t.Cleanup(func() { ntpTimeSyncUnit, timeSyncCommand = oldUnit, oldCommand })
	ntpTimeSyncUnit = func() string { return "" }
	var installed bool
	timeSyncCommand = func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name == "apt-get" {
			installed = true
		}
		return []byte("masked\n"), nil
	}
	if err := StartSystemTimeSync(); err == nil || installed {
		t.Fatal("masked administrator provider must block installation")
	}
}
