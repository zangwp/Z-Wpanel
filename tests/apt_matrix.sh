#!/bin/bash
# Exercise the actual installer repositories in a disposable distribution container.
set -euo pipefail
[[ "${CI:-}" == true && -f /.dockerenv && $EUID == 0 ]] || { echo "Run only in a disposable CI container" >&2; exit 1; }
installer=/src/install.sh
export DEBIAN_FRONTEND=noninteractive LC_ALL=C
apt-get update
apt-get install -y --no-install-recommends python3 ca-certificates
printf '#!/bin/sh\nexit 101\n' > /usr/sbin/policy-rc.d
chmod 0755 /usr/sbin/policy-rc.d
INSTALL_WORKDIR=$(mktemp -d)
trap 'rm -rf "$INSTALL_WORKDIR"' EXIT
APT_SOURCES_MUTATED=false
PREFER_CN=false
PHP_SOURCE_MODE=official
log_info() { printf '%s\n' "$*"; }
log_warn() { printf '%s\n' "$*" >&2; }
log_error() { printf '%s\n' "$*" >&2; exit 1; }
# Load only these installer function definitions and literal constants. Never run main/repair/purge.
python3 - "$installer" > "$INSTALL_WORKDIR/repositories.sh" <<'PY'
import re,sys
s=open(sys.argv[1],encoding='utf-8').read()
constants=['PHP_SERIES','MARIADB_SERIES','MARIADB_APT_KEY_FINGERPRINT','REDIS_APT_KEY_SHA256','MIN_REDIS_PACKAGE_VERSION','DEBSURY_KEYRING_PACKAGE','DEBSURY_KEYRING_VERSION','DEBSURY_KEYRING_SHA256','PHP_KEYRING_MAX_BYTES']
functions=['install_base_dependencies','install_runtime_dependencies','file_size_within_limit','download_file','apt_package_available','php_package_available','assert_managed_source_target','set_php_source_meta','configure_php_source','apt_candidate_version','install_verified_apt_key','configure_ubuntu_php_repository','select_php_source','configure_nginx_repository','configure_mariadb_repository','configure_redis_repository']
for key in constants:
 m=re.search(r'^'+key+r'=.*$',s,re.M);assert m,key;print(m[0])
for name in functions:
 m=re.search(r'^'+name+r'\(\) \{[^\n]*\}\s*$',s,re.M)
 if m is None:m=re.search(r'^'+name+r'\(\) \{\n.*?^\}',s,re.M|re.S)
 assert m,name;print(m[0])
PY
source "$INSTALL_WORKDIR/repositories.sh"
install_base_dependencies
source /etc/os-release
PLATFORM_ID=$ID
PLATFORM_VERSION=$VERSION_ID
PLATFORM_CODENAME=$VERSION_CODENAME
PLATFORM_ARCH=$(dpkg --print-architecture)
select_php_source "$PLATFORM_CODENAME"
configure_nginx_repository
configure_mariadb_repository
configure_redis_repository
install_runtime_dependencies
for command in gpg python3 rsync ssh sshpass logrotate jpegoptim optipng; do command -v "$command" >/dev/null; done
python3 -c 'import pyinotify'
"php${PHP_SERIES}" -m > "$INSTALL_WORKDIR/php-modules"
for module in curl dom exif fileinfo gd intl mbstring mysqli openssl pdo_mysql redis SimpleXML xml xmlreader xmlwriter zip "Zend OPcache"; do
    grep -Fxq "$module" "$INSTALL_WORKDIR/php-modules"
done
nginx -t
"php-fpm${PHP_SERIES}" -t
nginx -v
"php${PHP_SERIES}" --version
mariadbd --version
redis-server --version
