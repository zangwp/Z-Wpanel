package executor

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/zangwp/Z-Wpanel/internal/models"
)

func TestSiteBackupFPMJournalPrecedesConfigAndReload(t *testing.T) {
	f, _, _ := siteBackupPairedFixture(t)
	f.manifest.Phase = "restore_committed"
	original, err := os.ReadFile(f.site.PHPPoolPath)
	if err != nil {
		t.Fatal(err)
	}
	old := siteBackupReloadFPM
	t.Cleanup(func() { siteBackupReloadFPM = old })
	calls := 0
	siteBackupReloadFPM = func(context.Context) error {
		calls++
		data, err := os.ReadFile(filepath.Join(f.dir, "manifest.json"))
		if err != nil {
			t.Fatal(err)
		}
		var journal models.SiteBackupManifest
		if err = json.Unmarshal(data, &journal); err != nil {
			t.Fatal(err)
		}
		if !journal.FPMDisabled || journal.OriginalPHPPool != string(original) || journal.PHPPoolSHA256 == "" {
			t.Fatal("FPM changed before recoverable ownership was saved")
		}
		return nil
	}
	if err = f.disableFPM(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertFullRestoreFile(t, f.site.PHPPoolPath, f.fpmMaintenance())
	if err = f.restoreFPM(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertFullRestoreFile(t, f.site.PHPPoolPath, string(original))
	if f.manifest.FPMDisabled || calls != 2 {
		t.Fatal("pool recovery did not reload both states")
	}
	if f.manifest.Phase != "restore_committed" {
		t.Fatal("runtime journal update lost the committed restore phase")
	}
}

func TestSiteBackupFPMRestoreRejectsUnownedConfig(t *testing.T) {
	for _, kind := range []string{"external edit", "corrupt original"} {
		t.Run(kind, func(t *testing.T) {
			f, _, _ := siteBackupPairedFixture(t)
			old := siteBackupReloadFPM
			t.Cleanup(func() { siteBackupReloadFPM = old })
			siteBackupReloadFPM = func(context.Context) error { return nil }
			if err := f.disableFPM(context.Background()); err != nil {
				t.Fatal(err)
			}
			if kind == "external edit" {
				if err := os.WriteFile(f.site.PHPPoolPath, []byte("administrator replacement"), 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				f.manifest.OriginalPHPPool += "tampered"
			}
			before, err := os.ReadFile(f.site.PHPPoolPath)
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			siteBackupReloadFPM = func(context.Context) error { calls++; return nil }
			if f.restoreFPM(context.Background()) == nil {
				t.Fatal("unowned original accepted")
			}
			assertFullRestoreFile(t, f.site.PHPPoolPath, string(before))
			if calls != 0 || !f.manifest.FPMDisabled {
				t.Fatal("failed validation changed the recovery gate")
			}
		})
	}
}

func TestSiteBackupFPMReloadFailureRetainsGateAndRetries(t *testing.T) {
	f, _, _ := siteBackupPairedFixture(t)
	old := siteBackupReloadFPM
	t.Cleanup(func() { siteBackupReloadFPM = old })
	siteBackupReloadFPM = func(context.Context) error { return errors.New("simulated reload failure") }
	if f.disableFPM(context.Background()) == nil || !f.manifest.FPMDisabled {
		t.Fatal("failed reload lost recovery ownership")
	}
	calls := 0
	siteBackupReloadFPM = func(context.Context) error { calls++; return nil }
	if err := f.disableFPM(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatal("uncertain reload was not retried")
	}
}
