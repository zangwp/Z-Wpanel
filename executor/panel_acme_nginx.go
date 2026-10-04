package executor

import (
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/zangwp/Z-Wpanel/config"
	"github.com/zangwp/Z-Wpanel/database"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

var panelACMEMu sync.Mutex
var panelACMEConfigDir = "/etc/nginx/conf.d"
var runPanelACMECommand = func(ctx context.Context, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, args[0], args[1:]...).CombinedOutput()
}

// A panel hostname has an HTTP challenge-only vhost; the login itself continues
// using the panel's TLS listener. Site vhosts and certificate files are retained.
func panelACMEWebRoot(domain string) (string, error) {
	panelACMEMu.Lock()
	defer panelACMEMu.Unlock()
	if config.AppConfig == nil {
		return "", fmt.Errorf("panel configuration unavailable")
	}
	var err error
	if domain, err = NormalizePanelDomain(domain); err != nil {
		return "", err
	}
	if db := database.GetDB(); db != nil {
		var n int
		if err := db.QueryRow("SELECT COUNT(*) FROM websites WHERE domain=? OR instr(',' || replace(aliases,' ', '') || ',', ',' || ? || ',')>0", domain, domain).Scan(&n); err != nil {
			return "", err
		}
		if n > 0 {
			return "", fmt.Errorf("use a separate hostname for panel HTTPS; this domain belongs to a website")
		}
	}
	root := filepath.Join(config.AppConfig.Paths.WWWRoot, ".yub-panel-acme")
	if strings.ContainsAny(root, "\n\r\";{}") {
		return "", fmt.Errorf("unsafe challenge root")
	}
	if err := ensurePanelACMEChallengeDirectory(root); err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(domain))
	path := filepath.Join(panelACMEConfigDir, fmt.Sprintf("yubwpanel-panel-acme-%x.conf", sum[:8]))
	if st, err := os.Lstat(path); err == nil {
		if !st.Mode().IsRegular() {
			return "", fmt.Errorf("unsafe Nginx challenge configuration path")
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	text := fmt.Sprintf("# YUB WPanel panel HTTP challenge\nserver {\n listen 80;\n listen [::]:80;\n server_name %s;\n location ^~ /.well-known/acme-challenge/ { root \"%s\"; default_type text/plain; }\n location / { return 404; }\n}\n", domain, root)
	old, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if err == nil && !strings.HasPrefix(string(old), "# YUB WPanel panel HTTP challenge\n") {
		return "", fmt.Errorf("refusing to replace administrator Nginx configuration")
	}
	if string(old) == text {
		return root, nil
	}
	existed := err == nil
	if err := writePanelTLSFile(path, []byte(text), 0644); err != nil {
		return "", err
	}
	restore := func() error {
		if existed {
			return writePanelTLSFile(path, old, 0644)
		}
		return os.Remove(path)
	}
	ctx, cancel := context.WithTimeout(context.Background(), SimpleVPSToolTimeout())
	defer cancel()
	for step, args := range [][]string{{"nginx", "-t"}, {"systemctl", "reload", "nginx"}} {
		if out, err := runPanelACMECommand(ctx, args...); err != nil {
			if restoreErr := restore(); restoreErr != nil {
				return "", fmt.Errorf("Nginx challenge configuration failed: %v: %s; recovery failed: %w", err, out, restoreErr)
			}
			if step == 1 {
				// A failed reload may have partially taken effect. Reload the restored configuration.
				recoveryCtx, recoveryCancel := context.WithTimeout(context.Background(), SimpleVPSToolTimeout())
				defer recoveryCancel()
				if _, recoveryErr := runPanelACMECommand(recoveryCtx, "nginx", "-t"); recoveryErr == nil {
					if _, recoveryErr = runPanelACMECommand(recoveryCtx, "systemctl", "reload", "nginx"); recoveryErr != nil {
						return "", fmt.Errorf("Nginx challenge reload failed: %v; recovery reload failed: %w", err, recoveryErr)
					}
				} else {
					return "", fmt.Errorf("Nginx challenge reload failed: %v; recovery check failed: %w", err, recoveryErr)
				}
			}
			return "", fmt.Errorf("Nginx challenge configuration failed: %w: %s; previous configuration restored", err, out)
		}
	}
	return root, nil
}

func ensurePanelACMEChallengeDirectory(root string) error {
	// Reject symlink ancestors rather than publishing ACME files outside the root.
	for _, p := range []string{root, filepath.Join(root, ".well-known"), filepath.Join(root, ".well-known", "acme-challenge")} {
		if st, err := os.Lstat(p); err == nil {
			if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("unsafe challenge directory")
			}
		} else if !os.IsNotExist(err) {
			return err
		} else if err := os.Mkdir(p, 0755); err != nil {
			return err
		}
	}
	return nil
}
