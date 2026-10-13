package handlers

import (
	"context"
	"strings"

	"testing"
)

func withFakeSystemctlUnits(t *testing.T, units string) {
	t.Helper()
	old := timeSyncCommand
	t.Cleanup(func() { timeSyncCommand = old })
	timeSyncCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if name != "systemctl" || len(args) < 2 || args[0] != "show" {
			t.Fatalf("unexpected unit probe: %s %v", name, args)
		}
		state := "not-found"
		for _, unit := range strings.Fields(units) {
			if args[1] == unit {
				state = "loaded"
			}
		}
		return []byte("Id=" + args[1] + "\nLoadState=" + state + "\nActiveState=inactive\n"), nil
	}
}

func TestDetectNTPTimeSyncUnitPrefersChrony(t *testing.T) {
	withFakeSystemctlUnits(t, "chrony.service systemd-timesyncd.service")
	if unit := detectNTPTimeSyncUnit(); unit != "chrony.service" {
		t.Fatalf("unit=%q, want chrony.service", unit)
	}
}

func TestDetectNTPTimeSyncUnitFallsBackToTimesyncd(t *testing.T) {
	withFakeSystemctlUnits(t, "systemd-timesyncd.service")
	if unit := detectNTPTimeSyncUnit(); unit != "systemd-timesyncd.service" {
		t.Fatalf("unit=%q, want systemd-timesyncd.service", unit)
	}
}

func TestDetectNTPTimeSyncUnitReturnsEmptyWhenNeitherExists(t *testing.T) {
	withFakeSystemctlUnits(t, "")
	if unit := detectNTPTimeSyncUnit(); unit != "" {
		t.Fatalf("unit=%q, want empty", unit)
	}
}

func withFakeSystemctlMasked(t *testing.T, maskedUnit, availableUnit string) {
	withFakeSystemctlUnits(t, availableUnit)
	previous := timeSyncCommand
	timeSyncCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if len(args) > 1 && args[1] == maskedUnit {
			return []byte("Id=" + maskedUnit + "\nLoadState=masked\nActiveState=inactive\n"), nil
		}
		return previous(ctx, name, args...)
	}
}

func TestDetectNTPTimeSyncUnitSkipsMaskedChrony(t *testing.T) {
	withFakeSystemctlMasked(t, "chrony.service", "systemd-timesyncd.service")
	if unit := detectNTPTimeSyncUnit(); unit != "systemd-timesyncd.service" {
		t.Fatalf("unit=%q, want systemd-timesyncd.service (chrony is masked)", unit)
	}
}
