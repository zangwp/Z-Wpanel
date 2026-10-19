package executor

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/zangwp/Z-Wpanel/internal/config"
)

var siteSecurityNginxInclude = regexp.MustCompile(`\binclude\s+(?:"([^"\r\n]+)"|'([^'\r\n]+)'|([^;\s]+))\s*;`)

// Read only bounded configuration files under the managed Nginx directories.
// Includes affect routing and request guards even when the vhost is unchanged.
func (r *websiteSecurityFingerprintReader) nginxInputs() (map[string]string, bool) {
	if !r.nginxRead {
		r.nginxRead = true
		r.nginx, r.nginxOK = r.readNginxInputs()
	}
	return r.nginx, r.nginxOK
}

func (r *websiteSecurityFingerprintReader) readNginxInputs() (map[string]string, bool) {
	cfg := config.AppConfig
	root := filepath.Dir(filepath.Clean(cfg.Paths.NginxSitesAvailable))
	roots := []string{cfg.Paths.NginxSitesAvailable, root, nginxCustomDir}
	files := make(map[string]string)
	budget := int64(1024 * 1024)
	var visit func(string, int) bool
	visit = func(path string, depth int) bool {
		path = filepath.Clean(path)
		if _, exists := files[path]; exists {
			return true
		}
		if depth > 4 || len(files) >= 32 || !filepath.IsAbs(path) || strings.ContainsAny(path, "$\r\n") {
			return false
		}
		managedRoot := ""
		for _, candidate := range roots {
			if filepath.IsAbs(candidate) && pathWithin(candidate, path, false) {
				managedRoot = candidate
				break
			}
		}
		if managedRoot == "" {
			return false
		}
		data, err := readSiteSecurityFile(managedRoot, path, 256*1024)
		if err != nil || int64(len(data)) > budget {
			return false
		}
		budget -= int64(len(data))
		digest, ok := r.file(managedRoot, path, 256*1024, false)
		if !ok {
			return false
		}
		files[path] = digest
		for _, match := range siteSecurityNginxInclude.FindAllSubmatch(siteSecurityNginxWithoutComments(data), -1) {
			include := ""
			for _, value := range match[1:] {
				if len(value) > 0 {
					include = string(value)
					break
				}
			}
			if !filepath.IsAbs(include) {
				include = filepath.Join(root, include)
			}
			if strings.ContainsAny(include, "$\r\n") {
				return false
			}
			if strings.ContainsAny(include, "*?[") {
				matches, err := filepath.Glob(include)
				if err != nil || len(matches) > 32 {
					return false
				}
				// A newly matching file also changes the digest of an empty glob.
				files["glob:"+include] = strings.Join(matches, "\n")
				for _, name := range matches {
					if !visit(name, depth+1) {
						return false
					}
				}
			} else if !visit(include, depth+1) {
				return false
			}
		}
		return true
	}
	if !visit(r.site.NginxConfPath, 0) {
		return nil, false
	}
	// These generated maps hold the guards referenced by managed server blocks.
	// Missing maps are represented as absent; existing unsafe files fail closed.
	global := filepath.Join(root, "conf.d", "yubwpanel-log.conf")
	if _, err := os.Lstat(global); os.IsNotExist(err) {
		files[global] = "absent"
	} else if err != nil || !visit(global, 0) {
		return nil, false
	}
	return files, true
}

func siteSecurityNginxWithoutComments(data []byte) []byte {
	clean := append([]byte(nil), data...)
	var quote byte
	for i := 0; i < len(clean); i++ {
		if clean[i] == '\\' {
			i++
			continue
		}
		if quote != 0 {
			if clean[i] == quote {
				quote = 0
			}
			continue
		}
		if clean[i] == '\'' || clean[i] == '"' {
			quote = clean[i]
		} else if clean[i] == '#' {
			for i < len(clean) && clean[i] != '\n' {
				clean[i] = ' '
				i++
			}
		}
	}
	return clean
}
