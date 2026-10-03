package executor

import (
	"bufio"
	_ "embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

//go:embed panel_cli.sh
var panelCommandScript string

const (
	panelCommandMarker      = "# YUB WPanel CLI — b"
	legacyYUBWCommandMarker = "# YUB WPanel CLI — yubw"
	legacyWPCommandMarker   = "# YUB WPanel CLI — wp"
	managedMarkerScanLines  = 5
)

// legacyWPCommandMarker is the exact second line of the pre-panel-CLI-rename
// script (see git history of this file). It's used to recognize a leftover
// /usr/local/bin/wp created by an older version of this panel, as opposed
// to a real WP-CLI install that happens to live at the same path. Matching
// requires an exact, line-anchored match — not a substring — so it can't be
// tripped by unrelated text elsewhere in a user's own script.

// EnsurePanelCommands installs the short lowercase and uppercase CLI entry
// points. Because these are generic one-character names, existing paths are
// replaced only when they already carry YUB WPanel's exact ownership marker.
func EnsurePanelCommands() {
	paths := []string{"/usr/local/bin/b", "/usr/local/bin/B"}
	if err := migratePanelCommandsAt(paths, "/usr/local/bin/yubw"); err != nil {
		log.Printf("面板 b/B 命令迁移失败: %v", err)
		return
	}
	removeLegacyWPCommand()
}

func migratePanelCommandsAt(paths []string, legacyPath string) error {
	if err := ensurePanelCommandsAt(paths...); err != nil {
		return err
	}
	if err := removeManagedCommandAt(legacyPath, legacyYUBWCommandMarker); err != nil {
		return fmt.Errorf("remove managed legacy command %s: %w", legacyPath, err)
	}
	return nil
}

func ensurePanelCommandsAt(paths ...string) error {
	if len(paths) == 0 {
		return fmt.Errorf("no panel command paths supplied")
	}
	for _, path := range paths {
		replaceable, err := managedCommandPathReplaceable(path, panelCommandMarker)
		if err != nil {
			return fmt.Errorf("inspect %s: %w", path, err)
		}
		if !replaceable {
			return fmt.Errorf("refusing to replace non-YUB command at %s", path)
		}
	}
	for _, path := range paths {
		if err := writeFileAtomic(path, []byte(panelCommandScript), 0755); err != nil {
			return fmt.Errorf("install %s: %w", path, err)
		}
	}
	return nil
}

func managedCommandPathReplaceable(path, marker string) (bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, nil
	}
	return fileHasExactMarker(path, marker)
}

// removeLegacyWPCommand cleans up the old /usr/local/bin/wp shortcut from
// versions prior to the dedicated panel command, so it stops shadowing WP-CLI
// install. It only removes the file if one of its first few lines is an
// exact match for legacyWPCommandMarker; a user-installed WP-CLI (or any
// other file) at the same path is left untouched.
func removeLegacyWPCommand() {
	if err := removeManagedCommandAt("/usr/local/bin/wp", legacyWPCommandMarker); err != nil {
		log.Printf("清理旧版 wp 命令失败 (/usr/local/bin/wp): %v", err)
	}
}

// removeLegacyWPCommandAt implements removeLegacyWPCommand against an
// explicit path so it can be exercised against a temp file in tests.
func removeLegacyWPCommandAt(legacyPath string) {
	if err := removeManagedCommandAt(legacyPath, legacyWPCommandMarker); err != nil {
		log.Printf("清理旧版 wp 命令失败 (%s): %v", legacyPath, err)
	}
}

func removeManagedCommandAt(path, marker string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	matched, err := fileHasExactMarker(path, marker)
	if err != nil {
		return err
	}
	if !matched {
		return nil
	}
	return os.Remove(path)
}

// fileHasExactMarker only scans the first few lines, so an unrelated file is
// never read in full merely because it occupies a managed command path.
func fileHasExactMarker(path, marker string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for i := 0; i < managedMarkerScanLines && scanner.Scan(); i++ {
		if scanner.Text() == marker {
			return true, nil
		}
	}
	return false, scanner.Err()
}

// writeFileAtomic writes data to path via a temp file + rename in the same
// directory, so a concurrent reader (e.g. someone running b mid-upgrade)
// never observes a partially-written script.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".yub-wpanel-cli-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
