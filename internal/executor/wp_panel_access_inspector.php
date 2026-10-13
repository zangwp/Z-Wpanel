<?php
$yub_token = getenv('YUB_WPANEL_RUNNER_TOKEN'); $yub_sent = false;
$yub_send = function($ok, $data = [], $code = '') use (&$yub_sent, $yub_token) {
    if ($yub_sent) return; $yub_sent = true;
    file_put_contents('php://fd/3', json_encode(['token' => $yub_token, 'ok' => $ok, 'data' => $data, 'error_code' => $code], JSON_UNESCAPED_SLASHES));
};
register_shutdown_function(function() use (&$yub_sent, $yub_send) { if (!$yub_sent) $yub_send(false, [], 'runtime_unavailable'); });
$yub_raw = stream_get_contents(STDIN, 65537); $yub_input = json_decode($yub_raw, true);
if (PHP_SAPI !== 'cli' || !preg_match('/^[0-9a-f]{32}$/D', (string)$yub_token) || strlen($yub_raw) > 65536 || !is_array($yub_input)) { $yub_send(false, [], 'invalid_input'); exit(2); }
$yub_root = $yub_input['root'] ?? '';
if (!is_string($yub_root) || !is_dir($yub_root) || realpath($yub_root) !== rtrim($yub_root, '/')) { $yub_send(false, [], 'invalid_root'); exit(2); }
$_SERVER['HTTP_HOST'] = $yub_input['domain']; $_SERVER['SERVER_NAME'] = $yub_input['domain'];
$_SERVER['REQUEST_URI'] = '/'; $_SERVER['REQUEST_METHOD'] = 'GET'; $_SERVER['SCRIPT_FILENAME'] = $yub_root . '/index.php';
if (!empty($yub_input['https'])) { $_SERVER['HTTPS'] = 'on'; $_SERVER['SERVER_PORT'] = 443; } else { $_SERVER['SERVER_PORT'] = 80; }
if ($yub_input['phase'] === 'installation') { define('SHORTINIT', true); define('WP_INSTALLING', true); }
define('WP_USE_THEMES', false); if (!defined('WP_HTTP_BLOCK_EXTERNAL')) define('WP_HTTP_BLOCK_EXTERNAL', true);
if (!defined('DISABLE_WP_CRON')) define('DISABLE_WP_CRON', true);
chdir($yub_root); ob_start(); require $yub_root . '/wp-load.php'; ob_end_clean();
global $wpdb;
if (is_multisite()) { $yub_send(false, [], 'multisite_unsupported'); exit(1); }
if (DB_NAME !== $yub_input['db_name'] || $wpdb->prefix !== $yub_input['table_prefix']) { $yub_send(false, [], 'database_mismatch'); exit(1); }
if ($yub_input['phase'] === 'installation') { $yub_send(true, ['installed' => is_blog_installed()]); exit; }
if (!is_blog_installed()) { $yub_send(false, [], 'not_installed'); exit(1); }
if (!empty($yub_input['candidate_suffix'])) {
    $suffix = $yub_input['candidate_suffix'];
    if (!is_string($suffix) || !preg_match('/^[a-z][a-z0-9_-]{2,63}$/D', $suffix)) { $yub_send(false, [], 'invalid_suffix'); exit(1); }
    if (yub_wpanel_access_suffix_conflict($suffix)) { $yub_send(false, [], 'suffix_conflict'); exit(1); }
}
$yub_users = [];
foreach (get_users(['role' => 'administrator', 'orderby' => 'ID', 'order' => 'ASC', 'number' => 101]) as $user) {
    $yub_users[] = ['id' => (int)$user->ID, 'login' => $user->user_login, 'display_name' => $user->display_name, 'proof' => yub_wpanel_access_administrator_proof($user)];
}
if (count($yub_users) > 100) { $yub_send(false, [], 'too_many_administrators'); exit(1); }
$yub_siteurl = (string)get_option('siteurl'); $yub_login = wp_login_url();
$yub_expected = rtrim($yub_siteurl, '/') . '/' . ($yub_input['login_suffix'] ?: 'wp-login.php');
$yub_send(true, ['installed' => true, 'site_url' => $yub_siteurl, 'login_url' => $yub_login, 'conflicting_plugins' => yub_wpanel_access_login_conflicts($yub_expected, $yub_login), 'authentication_plugins' => yub_wpanel_access_authentication_conflicts(), 'administrators' => $yub_users, 'permalinks_enabled' => (string)get_option('permalink_structure') !== '', 'curl_unix_available' => function_exists('curl_init') && defined('CURLOPT_UNIX_SOCKET_PATH')]);
