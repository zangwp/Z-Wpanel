package router

import (
	"database/sql"
	"embed"
	"html/template"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/zangwp/Z-Wpanel/config"
	"github.com/zangwp/Z-Wpanel/database"
	"github.com/zangwp/Z-Wpanel/executor"
	"github.com/zangwp/Z-Wpanel/handlers"
	"github.com/zangwp/Z-Wpanel/i18n"
	"github.com/zangwp/Z-Wpanel/middleware"

	"github.com/gin-gonic/gin"
)

var panelVersion string

var i18nKeys = []string{
	"common.disabled",
	"common.enabled",
	"ssh_port.apply",
	"ssh_port.confirm",
	"ssh_port.confirm_begin",
	"ssh_port.deadline",
	"ssh_port.saved",
	"ssh_port.transition",
	"common.close",
	"common.help",
	"common.loading",
	"cron.actions",
	"cron.backup_duplicate_check_failed",
	"cron.backup_mode",
	"cron.backup_site_files",
	"cron.cancel",
	"cron.command",
	"cron.command_placeholder",
	"cron.content",
	"cron.copies",
	"cron.create",
	"cron.create_job",
	"cron.cron_expression_placeholder",
	"cron.custom_command",
	"cron.custom_cron_expression",
	"cron.daily_2am",
	"cron.delete",
	"cron.duplicate_backup_task",
	"cron.edit",
	"cron.edit_job",
	"cron.every_15_days",
	"cron.every_hour",
	"cron.every_minute",
	"cron.execution_logs",
	"cron.full_backup_help",
	"cron.full_site_backup",
	"cron.incremental_first_full",
	"cron.incremental_keep_help",
	"cron.job_name",
	"cron.keep_local_backups",
	"cron.last_manual_run",
	"cron.monthly_1st",
	"cron.no_jobs",
	"cron.no_system_jobs",
	"cron.notify_on_backup_failure",
	"cron.overview_help",
	"cron.panel_jobs_help",
	"cron.panel_jobs_title",
	"cron.readonly",
	"cron.run_command",
	"cron.run_schedule",
	"cron.save",
	"cron.schedule",
	"cron.select_website",
	"cron.site_settings",
	"cron.smart_incremental_help",
	"cron.source",
	"cron.ssl_renewal_help",
	"cron.ssl_renewal_title",
	"cron.status",
	"cron.system_jobs",
	"cron.system_jobs_help",
	"cron.system_update_failed",
	"cron.target_site",
	"cron.task_type",
	"cron.type",
	"cron.user",
	"cron.view_logs",
	"cron.weekly_sunday_3am",
	"cron.wp_cron_call",
	"cron.wp_cron_config_help_prefix",
	"cron.wp_cron_config_help_suffix",
	"cron.wp_cron_replace_help",
	"cron.wp_cron_replace_title",
	"cron.wp_cron_summary",
	"firewall.accept_policy_warning",
	"firewall.accept_policy_warning_title",
	"firewall.access_active",
	"firewall.access_add",
	"firewall.access_apply",
	"firewall.access_apply_confirm",
	"firewall.access_closed",
	"firewall.access_confirm",
	"firewall.access_conflicts",
	"firewall.access_deadline",
	"firewall.access_discard",
	"firewall.access_draft",
	"firewall.access_draft_help",
	"firewall.access_duplicate_port",
	"firewall.access_everyone",
	"firewall.access_existing",
	"firewall.access_help",
	"firewall.access_invalid_port",
	"firewall.access_legacy",
	"firewall.access_listening",
	"firewall.access_local",
	"firewall.access_loopback",
	"firewall.access_not_listening",
	"firewall.access_pending",
	"firewall.access_preserve",
	"firewall.access_preview",
	"firewall.access_public",
	"firewall.access_saved",
	"firewall.access_scope",
	"firewall.access_sources_hint",
	"firewall.access_steps",
	"firewall.access_title",
	"firewall.access_unavailable",
	"firewall.access_unsaved",
	"firewall.access_verified",
	"firewall.access_verify",
	"firewall.actions",
	"firewall.all_levels",
	"firewall.all_sources",
	"firewall.all_types",
	"firewall.any_source",
	"firewall.any_source_warning",
	"firewall.applied",
	"firewall.ban_count",
	"firewall.ban_duration",
	"firewall.ban_history",
	"firewall.banned_at",
	"firewall.close_port",
	"firewall.cloud_security_group_notice",
	"firewall.confirm_any_source",
	"firewall.confirm_close_port",
	"firewall.confirm_enable_protection",
	"firewall.confirm_open_port",
	"firewall.copy_info",
	"firewall.copy_report",
	"firewall.core_ports_summary",
	"firewall.count",
	"firewall.count_or_found",
	"firewall.created_at",
	"firewall.current_bans",
	"firewall.current_ip_help",
	"firewall.custom_service",
	"firewall.database_listener_warning",
	"firewall.description",
	"firewall.description_placeholder",
	"firewall.duration_10m",
	"firewall.duration_1h",
	"firewall.duration_24h",
	"firewall.duration_minutes",
	"firewall.duration_required",
	"firewall.enable_protected_mode",
	"firewall.event_types",
	"firewall.expires_at",
	"firewall.exposure_allowed_by_default",
	"firewall.exposure_local_only",
	"firewall.exposure_rule_dependent",
	"firewall.file_security_events",
	"firewall.file_security_events_help",
	"firewall.fill_ban",
	"firewall.firewall_anomalies",
	"firewall.firewall_anomalies_help",
	"firewall.firewall_backend",
	"firewall.got_it",
	"firewall.hit_count",
	"firewall.input_policy",
	"firewall.ip_address",
	"firewall.last_seen",
	"firewall.level",
	"firewall.listening_ports",
	"firewall.listening_ports_summary",
	"firewall.local_only",
	"firewall.lookup_ip",
	"firewall.managed_port_rules",
	"firewall.managed_rule_count",
	"firewall.managed_rule_count_help",
	"firewall.manual_ban_ip",
	"firewall.network_listening",
	"firewall.next_page",
	"firewall.no_current_bans",
	"firewall.no_file_security_events",
	"firewall.no_history",
	"firewall.no_managed_port_rules",
	"firewall.no_wp_suspicious_access",
	"firewall.open_port",
	"firewall.path",
	"firewall.path_samples",
	"firewall.pending_reconcile",
	"firewall.permanent_ban",
	"firewall.permanent_open",
	"firewall.port",
	"firewall.port_closed",
	"firewall.port_opened",
	"firewall.port_safety_notice",
	"firewall.ports_help",
	"firewall.ports_title",
	"firewall.prev_page",
	"firewall.protection_confirmation_failed",
	"firewall.protection_enabled",
	"firewall.protection_rollback_help",
	"firewall.protection_rollback_pending",
	"firewall.protocol",
	"firewall.reading_local_logs",
	"firewall.reason",
	"firewall.refresh",
	"firewall.refreshing_file_security_events",
	"firewall.reset_filters",
	"firewall.risk",
	"firewall.rule_404_detail",
	"firewall.rule_404_title",
	"firewall.rule_ban_detail",
	"firewall.rule_ban_title",
	"firewall.rule_login_detail",
	"firewall.rule_login_title",
	"firewall.rule_panel_detail",
	"firewall.rule_panel_title",
	"firewall.rule_proxy_detail",
	"firewall.rule_proxy_title",
	"firewall.rule_scan_detail",
	"firewall.rule_scan_title",
	"firewall.rule_summary_404",
	"firewall.rule_summary_login",
	"firewall.rule_summary_panel",
	"firewall.rule_summary_scan",
	"firewall.rule_summary_ssh",
	"firewall.rule_summary_web",
	"firewall.rule_web_detail",
	"firewall.rule_web_nginx_detail",
	"firewall.rule_web_protection_detail",
	"firewall.rule_web_title",
	"firewall.rules_dialog_intro",
	"firewall.rules_dialog_title",
	"firewall.rules_help",
	"firewall.rules_scope_detail",
	"firewall.rules_scope_title",
	"firewall.scope_any",
	"firewall.scope_current_ip",
	"firewall.scope_specific",
	"firewall.search",
	"firewall.search_ip_placeholder",
	"firewall.service_preset",
	"firewall.service_presets",
	"firewall.site",
	"firewall.source",
	"firewall.source_persistent_firewall",
	"firewall.source_required",
	"firewall.source_scope",
	"firewall.source_scope_help",
	"firewall.status",
	"firewall.temporary_open",
	"firewall.type",
	"firewall.unban",
	"firewall.unbanned_at",
	"firewall.unknown_process",
	"firewall.validity",
	"firewall.wp_suspicious_access",
	"firewall.wp_suspicious_access_cache_help",
	"firewall.wp_suspicious_access_help",
	"nav.ai_diagnostics",
	"nav.alert",
	"nav.backups",
	"nav.cron",
	"nav.dashboard",
	"nav.databases",
	"nav.files",
	"nav.firewall",
	"nav.group_operations",
	"nav.group_overview",
	"nav.group_sites",
	"nav.group_system",
	"nav.help",
	"nav.log_analysis",
	"nav.logout",
	"nav.menu",
	"nav.security",
	"nav.settings",
	"nav.software",
	"nav.vps",
	"nav.websites",
	"nav.wordpress_overview",
	"security.active_bans",
	"security.add_group",
	"security.auto_update_official_whitelist",
	"security.ban_time",
	"security.bot_burst",
	"security.bot_burst_help",
	"security.bot_limit",
	"security.bot_limit_help",
	"security.bot_requests_help",
	"security.bot_requests_per_minute",
	"security.builtin",
	"security.burst",
	"security.burst_help",
	"security.burst_warning",
	"security.cancel_edit",
	"security.cdn_mode_disabled_missing_origin_ips",
	"security.cdn_ranges_required",
	"security.cdn_realip_compatible_help",
	"security.cdn_realip_conclusion",
	"security.cdn_realip_groups",
	"security.cdn_realip_strict_help",
	"security.cdn_realip_subtitle",
	"security.cdn_status_summary",
	"security.cdn_trust_status",
	"security.cloudflare_group_help",
	"security.compatible_group_help",
	"security.custom_whitelist",
	"security.delete",
	"security.description",
	"security.description_placeholder",
	"security.disabled",
	"security.edit",
	"security.enable_bot_limit",
	"security.enable_global_rate_limit",
	"security.enable_group",
	"security.enable_telemetry",
	"security.entries_count",
	"security.fail2ban_config",
	"security.fetch_now",
	"security.find_time",
	"security.firewall_runtime_note",
	"security.googlebot_ranges",
	"security.googlebot_ranges_help",
	"security.header_name_help",
	"security.help",
	"security.help_bot_summary",
	"security.help_cdn_trust",
	"security.help_fail2ban_summary",
	"security.help_rate_limit_summary",
	"security.help_sqli_boundary",
	"security.help_testing_note",
	"security.help_whitelist_scope",
	"security.help_whitelist_summary",
	"security.import_googlebot_ranges",
	"security.increment_ban_help",
	"security.max_retry",
	"security.name",
	"security.name_placeholder",
	"security.next_run",
	"security.next_run_unknown",
	"security.official_whitelist",
	"security.page_subtitle",
	"security.privacy_section_help",
	"security.rate_limit",
	"security.rate_limit_help",
	"security.real_ip_header",
	"security.refresh_status",
	"security.requests_per_minute",
	"security.requests_per_minute_help",
	"security.runtime_status",
	"security.runtime_status_help",
	"security.save_bot_limit",
	"security.save_custom_whitelist",
	"security.save_group",
	"security.save_path_whitelist",
	"security.save_rate_limit",
	"security.save_ssh_whitelist",
	"security.save_web_whitelist",
	"security.ssh_whitelist",
	"security.ssh_whitelist_help",
	"security.status_active_enabled",
	"security.status_active_not_enabled",
	"security.status_attention",
	"security.status_disabled_by_setting",
	"security.status_inactive",
	"security.status_normal",
	"security.status_unknown",
	"security.tab_privacy",
	"security.tab_protection",
	"security.tab_trust",
	"security.telemetry",
	"security.telemetry_help",
	"security.telemetry_url",
	"security.telemetry_url_help",
	"security.trusted_cdn_origin_ips",
	"security.trusted_cdn_origin_ips_help",
	"security.trusted_cdn_origin_ips_placeholder",
	"security.web_whitelist",
	"security.web_whitelist_help",
	"security.whitelist_management",
	"security.whitelist_scope_help",
	"security.whitelist_scope_title",
	"security.whitelist_timer",
	"security.wp_security_path_help",
	"security.wp_security_path_whitelist",
	"settings.account_saved_logout",
	"settings.account_security",
	"settings.actions",
	"settings.advanced_download_settings",
	"settings.ai_privacy_notice",
	"settings.ai_settings",
	"settings.ai_settings_help",
	"settings.ai_timeout_help",
	"settings.all_stable",
	"settings.all_statuses",
	"settings.apply_panel_certificate",
	"settings.auth_key_detail",
	"settings.auth_method",
	"settings.auth_methods",
	"settings.auth_password_detail",
	"settings.auth_s3_detail",
	"settings.auto_update",
	"settings.auto_update_help",
	"settings.backup_path_prefix",
	"settings.backup_target",
	"settings.backup_target_rsync",
	"settings.backup_target_s3",
	"settings.backup_time",
	"settings.basic_auth_desc",
	"settings.basic_auth_saved",
	"settings.basic_auth_title",
	"settings.basic_auth_username_required",
	"settings.certificate_expiry",
	"settings.check_domain",
	"settings.confirm_new_password_placeholder",
	"settings.connection_mode",
	"settings.connection_mode_advanced",
	"settings.connection_mode_auto",
	"settings.connection_mode_legacy",
	"settings.copy_command",
	"settings.copy_to_remote",
	"settings.create",
	"settings.current_password_placeholder",
	"settings.current_password_required_for_account",
	"settings.dismiss_status",
	"settings.download",
	"settings.effective_backup_path",
	"settings.enable_path_isolation",
	"settings.enable_remote_backup",
	"settings.file_size",
	"settings.filename",
	"settings.filter_status",
	"settings.generate_password_title",
	"settings.generate_random",
	"settings.github_proxy",
	"settings.github_proxy_help",
	"settings.go_ai_diagnostics",
	"settings.help",
	"settings.hide_update_list",
	"settings.hostname",
	"settings.isolate_backup_path",
	"settings.isolation_disabled_warning",
	"settings.keep_local_backup",
	"settings.last_status",
	"settings.last_success",
	"settings.last_system_update",
	"settings.leave_unchanged_placeholder",
	"settings.legacy_mode_warning",
	"settings.local_package_missing",
	"settings.local_package_status",
	"settings.log_search_placeholder",
	"settings.model",
	"settings.new_password_placeholder",
	"settings.new_version_prefix",
	"settings.new_version_suffix",
	"settings.next_page",
	"settings.no_backup_records",
	"settings.no_matching_updates",
	"settings.no_operation_logs",
	"settings.ntp_starting",
	"settings.ntp_waiting",
	"settings.operation",
	"settings.operation_create_site",
	"settings.operation_dns_apply_preset",
	"settings.operation_dns_restore_automatic",
	"settings.operation_firewall_port_close",
	"settings.operation_firewall_port_open",
	"settings.operation_firewall_protection_confirm",
	"settings.operation_firewall_protection_enable",
	"settings.operation_nftables_enable_boot",
	"settings.operation_panel_auto_update",
	"settings.operation_panel_manual_update",
	"settings.operation_redis_object_cache_clear",
	"settings.operation_swap_apply_custom",
	"settings.operation_swap_apply_recommended",
	"settings.operation_swap_remove_managed",
	"settings.operation_wp_file_editor",
	"settings.operation_wp_optimizations",
	"settings.operation_wp_package_auto_check",
	"settings.operation_wp_password_reset",
	"settings.operation_wp_update_checks",
	"settings.package_detail_title",
	"settings.page_description",
	"settings.panel_certificate_confirm",
	"settings.panel_certificate_running",
	"settings.panel_db_backup",
	"settings.panel_db_backup_help",
	"settings.panel_db_backup_warning",
	"settings.panel_domain",
	"settings.panel_domain_help",
	"settings.panel_domain_https",
	"settings.panel_domain_verified",
	"settings.panel_title",
	"settings.panel_version",
	"settings.password_keep",
	"settings.patch_only",
	"settings.prev_page",
	"settings.provider",
	"settings.query",
	"settings.recent_operation_logs",
	"settings.refresh_logs",
	"settings.release_delay_minutes",
	"settings.remote_backup",
	"settings.remote_backup_path",
	"settings.remote_backup_root_path",
	"settings.remote_disabled_help",
	"settings.remote_help_tip",
	"settings.remote_help_title",
	"settings.remote_helper_create_help",
	"settings.remote_helper_dedicated_user",
	"settings.remote_helper_delete_help",
	"settings.remote_helper_delete_strong",
	"settings.remote_helper_key_mode",
	"settings.remote_helper_password_mode",
	"settings.remote_helper_permission",
	"settings.remote_helper_title",
	"settings.remote_host",
	"settings.remote_host_placeholder",
	"settings.remote_req_firewall",
	"settings.remote_req_path",
	"settings.remote_req_ssh",
	"settings.remote_server_requirements",
	"settings.remote_test_saved_config_help",
	"settings.restore",
	"settings.rows_per_page",
	"settings.s3_endpoint_placeholder",
	"settings.s3_help_bucket",
	"settings.s3_help_endpoint",
	"settings.s3_help_multipart",
	"settings.s3_help_prefix",
	"settings.s3_help_region",
	"settings.s3_help_supported",
	"settings.s3_help_title",
	"settings.save",
	"settings.save_basic_auth",
	"settings.save_web_account",
	"settings.section_backups",
	"settings.section_general",
	"settings.section_logs",
	"settings.section_navigation",
	"settings.section_updates",
	"settings.section_wordpress",
	"settings.security_advice",
	"settings.security_cleanup",
	"settings.security_dedicated",
	"settings.security_key_path",
	"settings.server_settings",
	"settings.server_time",
	"settings.show_update_list",
	"settings.signature_timeout_minutes",
	"settings.size",
	"settings.ssh_key_generated_help",
	"settings.ssh_key_recommended",
	"settings.ssh_password",
	"settings.ssh_password_placeholder",
	"settings.ssh_port",
	"settings.status",
	"settings.sync",
	"settings.system_update_check_failed",
	"settings.system_update_help",
	"settings.system_update_status_checking_remaining",
	"settings.system_update_status_remaining",
	"settings.system_update_status_remaining_failed",
	"settings.system_update_warning",
	"settings.system_updates",
	"settings.target",
	"settings.target_version",
	"settings.time",
	"settings.time_help",
	"settings.time_service_missing",
	"settings.timeout_seconds",
	"settings.timezone",
	"settings.up_to_date",
	"settings.update_check_failed",
	"settings.update_do_not_close",
	"settings.update_filter_all",
	"settings.update_filter_regular",
	"settings.update_filter_security",
	"settings.update_mode",
	"settings.update_window",
	"settings.updated_at",
	"settings.username_change_requires_password",
	"settings.web_login_desc",
	"settings.web_login_title",
	"settings.web_username_required",
	"settings.workflow",
	"settings.workflow_delete_local",
	"settings.workflow_local_backup",
	"settings.workflow_transfer",
	"settings.wp_package",
	"settings.wp_package_auto_check",
	"settings.wp_package_auto_check_help",
	"settings.wp_package_busy",
	"settings.wp_package_download_failed",
	"settings.wp_package_download_invalid",
	"settings.wp_package_download_timeout",
	"settings.wp_package_help",
	"settings.wp_package_invalid",
	"settings.wp_package_last_check",
	"settings.wp_package_publish_failed",
	"settings.wp_package_remote_version",
	"settings.wp_package_too_large",
	"settings.wp_package_version",
	"software.actions",
	"software.advice_dynamic_rule",
	"software.advice_execution_rule",
	"software.advice_heavy",
	"software.advice_input_rule",
	"software.advice_intro",
	"software.advice_large",
	"software.advice_principles",
	"software.advice_scene",
	"software.advice_simple",
	"software.advice_standard",
	"software.advice_title",
	"software.advice_upload_rule",
	"software.apply_failed",
	"software.apply_failed_rollback_failed",
	"software.apply_failed_rolled_back",
	"software.assign_php_per_site",
	"software.auto_check_30s",
	"software.clear_log",
	"software.config_unchanged",
	"software.config_updated",
	"software.confirm_clear_log",
	"software.confirm_restart_service",
	"software.confirm_stop_service",
	"software.current_value",
	"software.development_tool_install_failed",
	"software.development_tools",
	"software.development_tools_help",
	"software.disabled",
	"software.enabled",
	"software.go_to_system_update",
	"software.got_it",
	"software.hide_recommendations",
	"software.log_cleared_placeholder",
	"software.log_load_failed",
	"software.log_load_failed_with_error",
	"software.log_loading",
	"software.log_title",
	"software.manage_service_updates",
	"software.manually_stopped",
	"software.mariadb_runtime_version_policy",
	"software.opcache_clear",
	"software.opcache_clear_confirm",
	"software.opcache_clear_failed",
	"software.opcache_cleared",
	"software.opcache_clearing",
	"software.opcache_max_accelerated_files_hint",
	"software.opcache_max_accelerated_files_label",
	"software.opcache_memory_consumption_hint",
	"software.opcache_memory_consumption_label",
	"software.page_help",
	"software.php_runtime_install_notice",
	"software.php_runtime_installed",
	"software.php_runtime_site_count",
	"software.php_runtime_version_policy",
	"software.php_runtimes",
	"software.php_runtimes_help",
	"software.php_version_policy",
	"software.php_version_policy_help",
	"software.primary_runtime",
	"software.process_guard",
	"software.process_guard_help",
	"software.recommend",
	"software.recommend_basis",
	"software.recommend_reason",
	"software.recommend_reason_php",
	"software.recommend_refreshed",
	"software.recommended_default",
	"software.recommending",
	"software.recovery_count",
	"software.redis_runtime_version_policy",
	"software.refresh_tools",
	"software.restart",
	"software.runtime_stack",
	"software.save_rebuild_fpm",
	"software.save_reload",
	"software.save_reloading",
	"software.service",
	"software.service_status",
	"software.start",
	"software.status",
	"software.stop",
	"software.suggested_value",
	"software.system_package_version_policy",
	"software.tab_runtime",
	"software.tab_tools",
	"software.tab_tuning",
	"software.tool_check_failed",
	"software.tool_partial",
	"software.tool_status_help",
	"software.update_available_candidate",
	"software.version",
	"software.view_logs",
	"software.wp_cli_not_cron",
	"software.wp_cli_proxy_download_failed",
	"software.wp_config_advice",
	"vps.advanced_lifecycle",
	"vps.apply_tool",
	"vps.backups",
	"vps.balanced",
	"vps.boot",
	"vps.choose_setting",
	"vps.cleanup",
	"vps.cleanup_help",
	"vps.command_check_update",
	"vps.command_menu",
	"vps.command_status",
	"vps.command_status_help",
	"vps.command_uninstall",
	"vps.command_uninstall_help",
	"vps.command_update",
	"vps.command_update_help",
	"vps.confirm_tool",
	"vps.congestion_control",
	"vps.copied",
	"vps.copy",
	"vps.cores",
	"vps.default",
	"vps.default_qdisc",
	"vps.destructive_note",
	"vps.disk",
	"vps.dns_applied",
	"vps.dns_apply",
	"vps.dns_confirm",
	"vps.dns_current",
	"vps.dns_international",
	"vps.dns_ipv4",
	"vps.dns_ipv6",
	"vps.dns_ipv6_unavailable",
	"vps.dns_mainland",
	"vps.dns_manager",
	"vps.dns_presets",
	"vps.dns_presets_help",
	"vps.dns_reason_external_manager",
	"vps.dns_reason_foreign_config",
	"vps.dns_reason_read_failed",
	"vps.dns_reason_unsupported",
	"vps.dns_restore",
	"vps.dns_restored",
	"vps.dns_settings",
	"vps.dns_test",
	"vps.dns_test_ok",
	"vps.dns_testing",
	"vps.dns_unavailable",
	"vps.full_uninstall",
	"vps.full_uninstall_help",
	"vps.ip_custom",
	"vps.ip_external_help",
	"vps.ip_help",
	"vps.ip_priority",
	"vps.ip_restore_managed",
	"vps.ip_state_default",
	"vps.ip_state_external",
	"vps.ip_state_managed",
	"vps.ip_state_unavailable",
	"vps.ip_unavailable_help",
	"vps.ip_view_rules",
	"vps.ipv4",
	"vps.ipv6",
	"vps.kernel_arch",
	"vps.lifecycle_help",
	"vps.lifecycle_title",
	"vps.load",
	"vps.load_failed",
	"vps.locale_help",
	"vps.maintenance_tab",
	"vps.manage_ports",
	"vps.memory",
	"vps.network_tuning",
	"vps.network_tuning_help",
	"vps.nftables_boot_enabled",
	"vps.nftables_boot_warning",
	"vps.nftables_enable_boot",
	"vps.nftables_enable_confirm",
	"vps.ntp_not_synced",
	"vps.ntp_synced",
	"vps.open_software",
	"vps.operating_system",
	"vps.overview_tab",
	"vps.preview",
	"vps.processor",
	"vps.reboot_required",
	"vps.refresh",
	"vps.refreshing",
	"vps.resource_snapshot",
	"vps.run_cleanup",
	"vps.running",
	"vps.safe_tools",
	"vps.service",
	"vps.service_health",
	"vps.service_health_help",
	"vps.signed_updates",
	"vps.state_activating",
	"vps.state_active",
	"vps.state_disabled",
	"vps.state_enabled",
	"vps.state_inactive",
	"vps.state_unknown",
	"vps.subtitle",
	"vps.swap_action_needed",
	"vps.swap_active_sites",
	"vps.swap_apply_recommended",
	"vps.swap_current",
	"vps.swap_custom_help",
	"vps.swap_custom_saved",
	"vps.swap_custom_title",
	"vps.swap_failed",
	"vps.swap_file",
	"vps.swap_file_size",
	"vps.swap_help",
	"vps.swap_managed",
	"vps.swap_no_sources",
	"vps.swap_none",
	"vps.swap_panel_managed",
	"vps.swap_partition",
	"vps.swap_reason_memory_pressure",
	"vps.swap_reason_multiple_wordpress",
	"vps.swap_reason_single_wordpress",
	"vps.swap_reason_swap_pressure",
	"vps.swap_reason_system_default",
	"vps.swap_reason_zram",
	"vps.swap_recommended",
	"vps.swap_recommended_applied",
	"vps.swap_recommended_swappiness",
	"vps.swap_remove",
	"vps.swap_remove_help",
	"vps.swap_remove_title",
	"vps.swap_removed",
	"vps.swap_safety_note",
	"vps.swap_satisfied",
	"vps.swap_save_custom",
	"vps.swap_save_swappiness",
	"vps.swap_sources",
	"vps.swap_sources_help",
	"vps.swap_swappiness_saved",
	"vps.swap_title",
	"vps.swap_working",
	"vps.system_language",
	"vps.system_overview",
	"vps.system_updates",
	"vps.timezone_ntp",
	"vps.title",
	"vps.tool_firewall",
	"vps.tool_firewall_help",
	"vps.tool_logs",
	"vps.tool_logs_help",
	"vps.tool_services",
	"vps.tool_services_help",
	"vps.tool_updates",
	"vps.tool_updates_help",
	"vps.tuning_help",
	"vps.uninstall_scope",
	"vps.uptime",
	"vps.virtualization",
	"vps.website_recommended",
	"vps.website_tuning",
	"website.access_log",
	"website.actions",
	"website.add_www_redirect",
	"website.add_www_redirect_help",
	"website.advanced_site_info",
	"website.alias_empty_hint",
	"website.alias_handling",
	"website.alias_not_configured",
	"website.alias_placeholder",
	"website.alias_preview_same_site",
	"website.alias_redirect_301",
	"website.alias_redirect_302",
	"website.alias_redirect_help",
	"website.alias_serve_same_site",
	"website.aliases",
	"website.aliases_one_per_line",
	"website.allow_plugin_read_ssl",
	"website.allow_xmlrpc",
	"website.analyze_logs",
	"website.apply_recommended_cache",
	"website.applying_recommended_cache",
	"website.auto_backup",
	"website.auto_fill_panel_domain",
	"website.auto_ssl",
	"website.auto_ssl_help",
	"website.back_to_list",
	"website.basic_info",
	"website.cache",
	"website.cache_status_checking",
	"website.cache_status_unknown",
	"website.cache_ttl_seconds",
	"website.cache_unlock_to_configure",
	"website.cancel",
	"website.cdn_real_ip",
	"website.cdn_unavailable_no_origin_ips",
	"website.cert_auto",
	"website.cert_manual",
	"website.cert_path",
	"website.change_db_password",
	"website.change_password",
	"website.change_site_url",
	"website.change_site_urls",
	"website.change_wp_admin",
	"website.change_wp_admin_title",
	"website.check_dns",
	"website.check_interval",
	"website.clean_default_plugins",
	"website.clear",
	"website.clear_cache",
	"website.clear_page_cache",
	"website.clear_redis_cache",
	"website.clearing_redis_cache",
	"website.cloudflare_realip_auto",
	"website.companion_active",
	"website.companion_checking",
	"website.companion_inactive",
	"website.companion_refresh",
	"website.companion_unknown",
	"website.companion_unlock_install",
	"website.compressed",
	"website.config_examples",
	"website.configured",
	"website.confirm_clear_redis_cache",
	"website.confirm_reinstall",
	"website.copies",
	"website.create_site",
	"website.create_title",
	"website.created",
	"website.created_at",
	"website.created_ssl_warning",
	"website.creating_site",
	"website.current",
	"website.current_home",
	"website.current_siteurl",
	"website.database",
	"website.database_label",
	"website.database_name",
	"website.database_password",
	"website.days",
	"website.db_backup",
	"website.db_password_php_placeholder",
	"website.db_password_wp_placeholder",
	"website.debug_display",
	"website.debug_display_help",
	"website.debug_mode_help",
	"website.default_placeholder",
	"website.delete",
	"website.delete_cert",
	"website.detail_nav_cache",
	"website.detail_nav_logs",
	"website.detail_nav_overview",
	"website.detail_nav_protection",
	"website.detail_subtitle",
	"website.detail_title",
	"website.details",
	"website.disable_application_passwords",
	"website.disable_application_passwords_help",
	"website.disable_file_editing",
	"website.disable_file_editing_help",
	"website.disable_file_editing_managed",
	"website.disable_wp_updates",
	"website.disable_wp_updates_help",
	"website.dns_check_attention",
	"website.dns_check_passed",
	"website.dns_checking",
	"website.dns_fix_before_ssl",
	"website.document_root",
	"website.document_root_detail_help",
	"website.document_root_help",
	"website.domain",
	"website.domain_change_ssl_warning",
	"website.domain_saved_manual_ssl",
	"website.domain_saved_ssl_not_reissued",
	"website.domain_saved_ssl_reissue_failed",
	"website.domain_saved_ssl_reissued",
	"website.domain_saved_ssl_state_failed",
	"website.download",
	"website.download_cert_package",
	"website.download_current_full_log",
	"website.download_current_log",
	"website.edit_domain",
	"website.edit_expiry_time",
	"website.enable_debug_mode",
	"website.enable_fastcgi_cache",
	"website.enable_free_ssl",
	"website.enable_other_cdn_realip",
	"website.enable_server_page_cache",
	"website.enable_site_monitoring",
	"website.enable_ssl",
	"website.enable_ssl_for",
	"website.error_log",
	"website.expiry_help",
	"website.expiry_optional",
	"website.expiry_time",
	"website.fastcgi_cache_how_it_works",
	"website.fastcgi_cache_point_key",
	"website.fastcgi_cache_point_php_skip",
	"website.fastcgi_cache_point_static",
	"website.fastcgi_cache_point_wp_skip",
	"website.file",
	"website.file_editing_save_failed",
	"website.file_editing_wordpress_only",
	"website.file_lock",
	"website.file_lock_apply_failed_notice",
	"website.file_lock_column",
	"website.file_lock_enabled_notice",
	"website.file_lock_help",
	"website.file_lock_invalid_mode",
	"website.file_lock_legacy_notice",
	"website.file_lock_limit_help",
	"website.file_lock_limit_title",
	"website.file_lock_limits_toggle",
	"website.file_lock_mode",
	"website.file_lock_plugin_optional",
	"website.file_lock_preview_disclaimer",
	"website.file_lock_preview_failed",
	"website.file_lock_preview_title",
	"website.file_short",
	"website.filename",
	"website.fullscreen",
	"website.generate_random_password",
	"website.history_log_download",
	"website.https_forced",
	"website.https_policy",
	"website.https_requires_ssl",
	"website.inherit_php_memory",
	"website.job_failed",
	"website.job_no_record",
	"website.job_started",
	"website.job_success",
	"website.keep",
	"website.last_ssl_failed",
	"website.list_tab",
	"website.loading",
	"website.location_level_config",
	"website.log_dir",
	"website.log_last_1000",
	"website.log_last_200",
	"website.log_last_5000",
	"website.log_preview_help",
	"website.logs",
	"website.logs_collapsed_help",
	"website.logs_tab_help",
	"website.main_domain",
	"website.maintenance_tab",
	"website.manage",
	"website.manage_php_memory",
	"website.manage_php_runtimes",
	"website.manual_ssl",
	"website.manual_ssl_domains_help",
	"website.memory_limit",
	"website.memory_limit_help",
	"website.minute_1",
	"website.minute_10",
	"website.minute_30",
	"website.minute_5",
	"website.modified_at",
	"website.monitoring",
	"website.monitoring_disabled_help",
	"website.monitoring_enabled_help",
	"website.new_home",
	"website.new_password_label",
	"website.new_siteurl",
	"website.nginx_config_scope_warning",
	"website.nginx_custom_config",
	"website.nginx_location_config_help",
	"website.nginx_pre_config_help",
	"website.no_backups",
	"website.no_downloadable_logs",
	"website.no_non_cloudflare_cdn_groups",
	"website.no_websites",
	"website.non_cloudflare_cdn_notice",
	"website.not_configured",
	"website.notices",
	"website.open_wp_admin",
	"website.optimization_conflict",
	"website.optimizer_not_needed",
	"website.optimizer_retire",
	"website.optimizer_retire_done",
	"website.page_cache",
	"website.page_cache_help",
	"website.password_reset_admin",
	"website.password_reset_admin_help",
	"website.password_reset_all",
	"website.password_reset_all_hides_link",
	"website.password_reset_allow",
	"website.password_reset_help",
	"website.password_reset_invalid_mode",
	"website.password_reset_mode",
	"website.password_reset_save_failed",
	"website.password_reset_wordpress_only",
	"website.performance_cache",
	"website.php_password_help",
	"website.php_runtime",
	"website.php_runtime_create_help",
	"website.php_runtime_switch",
	"website.php_runtime_switch_confirm",
	"website.php_runtime_switch_failed",
	"website.php_runtime_switch_help",
	"website.php_runtime_switched",
	"website.php_runtime_switching",
	"website.plugin_installed",
	"website.post_revisions",
	"website.post_revisions_help",
	"website.private_key_path",
	"website.project_root",
	"website.public_root_option",
	"website.quick_add_www_redirect",
	"website.recent_1000_lines",
	"website.recent_200_lines",
	"website.recent_5000_lines",
	"website.redirect_preview",
	"website.redis_cache_clear_failed",
	"website.redis_cache_cleared",
	"website.redis_object_cache",
	"website.redis_object_cache_help",
	"website.refresh",
	"website.refresh_cache_status",
	"website.refresh_list",
	"website.reinstall_warning",
	"website.reinstall_wordpress",
	"website.reissue",
	"website.reissue_ssl_domains",
	"website.reissue_ssl_domains_help",
	"website.reissue_ssl_for",
	"website.remove_unused_default_themes",
	"website.renewal_enabled",
	"website.renewal_help",
	"website.renewal_manual_help",
	"website.replace_cert",
	"website.replace_ssl_for",
	"website.rotated",
	"website.runtime_configuration",
	"website.scheduled_wp",
	"website.scheduled_wp_enable",
	"website.scheduled_wp_enabled",
	"website.scheduled_wp_example",
	"website.scheduled_wp_help",
	"website.scheduled_wp_last",
	"website.scheduled_wp_suspended",
	"website.security_log",
	"website.server_cache_advanced",
	"website.server_cache_advanced_help",
	"website.server_level_config",
	"website.server_page_cache",
	"website.site_logs",
	"website.site_monitoring",
	"website.site_root",
	"website.site_type",
	"website.size",
	"website.ssl_cert_content",
	"website.ssl_key_content",
	"website.ssl_management",
	"website.ssl_precheck_failed",
	"website.ssl_precheck_warning",
	"website.status",
	"website.status_action_unavailable",
	"website.sync_wp_home_mismatch",
	"website.sync_wp_siteurl_mismatch",
	"website.sync_wp_urls_domain_unchanged",
	"website.sync_wp_urls_prefix_required",
	"website.sync_wp_urls_read_failed",
	"website.sync_wp_urls_wordpress_only",
	"website.system_user",
	"website.table_prefix",
	"website.table_prefix_help",
	"website.table_prefix_placeholder",
	"website.time",
	"website.type",
	"website.update_checks_save_failed",
	"website.update_checks_wordpress_only",
	"website.user_label",
	"website.username",
	"website.view_site_handle_ssl",
	"website.wp_admin_account",
	"website.wp_admin_backup_failed",
	"website.wp_admin_backup_timeout",
	"website.wp_admin_database_mismatch",
	"website.wp_admin_destroy_sessions",
	"website.wp_admin_display_name",
	"website.wp_admin_email",
	"website.wp_admin_email_exists",
	"website.wp_admin_generate_password",
	"website.wp_admin_invalid_display_name",
	"website.wp_admin_invalid_email",
	"website.wp_admin_invalid_login",
	"website.wp_admin_invalid_params",
	"website.wp_admin_invalid_password",
	"website.wp_admin_invalid_site",
	"website.wp_admin_loading",
	"website.wp_admin_login",
	"website.wp_admin_login_exists",
	"website.wp_admin_multisite_unsupported",
	"website.wp_admin_non_transactional_engine",
	"website.wp_admin_none",
	"website.wp_admin_not_found",
	"website.wp_admin_password",
	"website.wp_admin_password_confirm",
	"website.wp_admin_prefix_required",
	"website.wp_admin_site_busy",
	"website.wp_admin_site_not_found",
	"website.wp_admin_sync_nicename",
	"website.wp_admin_sync_site_email",
	"website.wp_admin_transaction_failed",
	"website.wp_admin_verification_failed",
	"website.wp_admin_warning",
	"website.wp_admin_wordpress_only",
	"website.wp_admin_write_failed",
	"website.wp_initialization",
	"website.wp_optimization_advanced",
	"website.wp_optimizer_plugin_help",
	"website.xmlrpc_help",

	"software.nginx_runtime_help",

	"anomaly.disabled", "anomaly.pending", "anomaly.last_success", "anomaly.post_count", "anomaly.application_password_count", "anomaly.application_password_pending", "anomaly.database_object_count", "anomaly.database_object_pending",
	"anomaly.plugin_required", "anomaly.multisite_unsupported", "anomaly.site_busy", "anomaly.site_unavailable",
	"anomaly.sample_malformed", "anomaly.sample_too_large",
	"anomaly.busy", "anomaly.invalid", "anomaly.failed",
	"alert.type_wp_admin_change", "alert.type_wp_post_volume", "alert.type_wp_content_change", "alert.type_wp_content_volume", "alert.type_wp_setting_change", "alert.type_wp_application_password", "alert.type_wp_database_object", "alert.type_wp_code_integrity",
	"maintenance.locked", "maintenance.unlocked", "maintenance.unlocked_permanent", "maintenance.unlocking",
	"maintenance.relocking", "maintenance.relock_failed", "maintenance.unknown", "maintenance.state_unknown",
	"maintenance.operation_unavailable", "maintenance.verification_failed", "maintenance.password_required",
	"maintenance.lock_mode_required",
	"maintenance.copied", "maintenance.invalid_configuration",
	"auth.connect_failed",
	"auth.login_failed",
	"auth.missing_credentials",
	"auth.session_expired",
	"settings.saving",
	"security.save_settings",
	"backups.backup_auto",
	"backups.backup_manual",
	"backups.authorize_delete_tasks",
	"backups.authorize_rebuild_tasks",
	"backups.batch_saved",
	"backups.batch_saved_with_skips",
	"backups.confirm_rebuild_tasks",
	"backups.duplicate_mode_tasks",
	"backups.actions",
	"backups.clean_record",
	"backups.collapse_all",
	"backups.confirm_clean_record",
	"backups.confirm_delete_local",
	"backups.delete",
	"backups.deleted",
	"backups.download",
	"backups.expand_all",
	"backups.mode_full",
	"backups.mode_incremental",
	"backups.no_backups",
	"backups.summary_db_count",
	"backups.summary_file_count",
	"backups.transport_failed",
	"backups.transport_local",
	"backups.transport_missing",
	"backups.transport_synced",
	"backups.reconcile_remote_status",
	"backups.reconcile_success",
	"backups.reconciling",
	"backups.remote_disabled",
	"backups.remote_disabled_help",
	"backups.remote_enabled",
	"backups.remote_enabled_help",
	"backups.saving",
	"backups.remote_chain_healthy",
	"backups.remote_chain_cleanup_pending",
	"backups.remote_chain_repair_pending",
	"backups.remote_chain_rebuild",
	"backups.remote_chain_unknown",
	"backups.transport_synced_remote_only",
	"common.cancel",
	"common.confirm",
	"common.none",
	"common.operation_success",
	"common.save",
	"common.network_error",
	"common.operation_failed",
	"common.request_cancelled",
	"common.request_failed",
	"common.request_timeout",
	"common.saving",
	"common.service_busy",
	"common.service_exception",
	"cron.day_separator",
	"cron.full_backup",
	"cron.incremental_backup",
	"cron.schedule_daily",
	"cron.schedule_monthly",
	"cron.schedule_quarterly",
	"cron.schedule_weekly",
	"cron.weekday_friday",
	"cron.weekday_monday",
	"cron.weekday_saturday",
	"cron.weekday_sunday",
	"cron.weekday_thursday",
	"cron.weekday_tuesday",
	"cron.weekday_wednesday",
	"database.adminer_active_for",
	"database.adminer_database_password",
	"database.adminer_database_password_help",
	"database.adminer_disable",
	"database.adminer_disabled",
	"database.adminer_duration",
	"database.adminer_enable",
	"database.adminer_enable_failed",
	"database.adminer_enabled",
	"database.adminer_expires_at",
	"database.adminer_help",
	"database.adminer_indefinite",
	"database.adminer_minutes",
	"database.adminer_open",
	"database.adminer_starting",
	"database.adminer_stopping",
	"database.adminer_title",
	"database.adminer_warning",
	"dashboard.chart_load",
	"dashboard.chart_memory",
	"dashboard.close",
	"dashboard.swap_usage",
	"dashboard.system_updates",
	"dashboard.tooltip_time",
	"dashboard.update_available",
	"files.root_directory",
	"help.copy_failed",
	"help.email_copied",
	"help.wechat_copied",
	"settings.account_saved_please_relogin",
	"settings.ai_settings_saved",
	"settings.api_key_placeholder",
	"settings.auto_update_saved",
	"settings.available_count",
	"settings.backing_up",
	"settings.backup_finished",
	"settings.backup_now",
	"settings.basic_auth_password_min_length",
	"settings.check_updates",
	"settings.checking",
	"settings.command_copied",
	"settings.command_copied_short",
	"settings.connection_mode_advanced_help",
	"settings.connection_mode_auto_help",
	"settings.confirm_delete_backup",
	"settings.confirm_delete_local_wp_package",
	"settings.confirm_restore_db_backup",
	"settings.confirm_system_update",
	"settings.confirm_update_version",
	"settings.connection_failed",
	"settings.connection_failed_with_reason",
	"settings.connection_ok",
	"settings.connection_ok_with_latency",
	"settings.current_password_required",
	"settings.delete",
	"settings.deleted",
	"settings.deleting",
	"settings.disabled",
	"settings.downloaded",
	"settings.downloading",
	"settings.downloading_update_package",
	"settings.enabled",
	"settings.failed",
	"settings.new_password_min_length",
	"settings.no_changes",
	"settings.no_data",
	"settings.ntp_not_synced",
	"settings.ntp_synced",
	"settings.one_click_update",
	"settings.online_download",
	"settings.package_download_complete",
	"settings.package_not_installed",
	"settings.package_ready",
	"settings.wp_package_check_status_failed",
	"settings.wp_package_check_status_up_to_date",
	"settings.wp_package_check_status_updated",
	"settings.wp_package_never_checked",
	"settings.panel_restarting",
	"settings.passwords_not_match",
	"settings.preparing_update",
	"settings.proxy_required",
	"settings.refresh",
	"settings.restoring",
	"settings.panel_db_restore_status_unknown",
	"settings.save_account_settings",
	"settings.save_ai_settings",
	"settings.save_failed",
	"settings.save_settings",
	"settings.save_before_copy_command",
	"settings.saved",
	"settings.saving",
	"settings.success",
	"settings.running",
	"settings.waiting",
	"settings.skipped",
	"settings.info",
	"settings.unknown_status",
	"settings.system_update_already_running",
	"settings.system_update_completed",
	"settings.system_update_failed",
	"settings.system_update_start_failed",
	"settings.system_update_status_checking_packages",
	"settings.system_update_status_checking_services",
	"settings.system_update_status_failed",
	"settings.system_update_status_health_failed",
	"settings.system_update_status_interrupted",
	"settings.system_update_status_preflight",
	"settings.system_update_status_queued",
	"settings.system_update_status_refresh",
	"settings.system_update_status_start_failed",
	"settings.system_update_status_success",
	"settings.system_update_status_upgrading",
	"settings.test",
	"settings.test_connection",
	"settings.test_failed",
	"settings.testing",
	"settings.time_sync_triggered",
	"settings.total_records",
	"settings.unknown_error",
	"settings.up_to_date_message",
	"settings.update_completed_refresh",
	"settings.update_completed_restart",
	"settings.update_now",
	"settings.update_started",
	"settings.update_status_timeout",
	"settings.updated",
	"settings.updating",
	"settings.upload_failed",
	"settings.upload_package",
	"settings.upload_success",
	"settings.uploading",
	"settings.zip_only",
	"site_migration.connected",
	"site_migration.peer_status_paired",
	"site_migration.peer_status_pending",
	"site_migration.add_receiving_server",
	"site_migration.hide_pairing",
	"site_migration.copied",
	"site_migration.copy_failed",
	"site_migration.operation_failed",
	"site_migration.package_generated",
	"site_migration.preflight_complete",
	"site_migration.estimate_summary",
	"site_migration.direction_source",
	"site_migration.direction_target",
	"site_migration.sending_server",
	"site_migration.receiving_server",
	"site_migration.progress_waiting_source",
	"site_migration.progress_queued",
	"site_migration.progress_preparing_source",
	"site_migration.progress_receiving",
	"site_migration.progress_receiving_bytes",
	"site_migration.progress_receiving_speed",
	"site_migration.progress_sending",
	"site_migration.progress_sending_bytes",
	"site_migration.progress_sending_speed",
	"site_migration.progress_extracting",
	"site_migration.progress_remote_extracting",
	"site_migration.progress_building_target",
	"site_migration.progress_activating",
	"site_migration.progress_processing",
	"site_migration.progress_decision",
	"site_migration.progress_retryable_failure",
	"site_migration.progress_manual_failure",
	"site_migration.progress_interrupted",
	"site_migration.start_confirm",
	"site_migration.started",
	"site_migration.retry_queued",
	"site_migration.delete_task",
	"site_migration.delete_source_task_confirm",
	"site_migration.delete_target_task_confirm",
	"site_migration.task_deleted",
	"site_migration.completed",
	"site_migration.completed_delete_queued",
	"site_migration.restore_source_confirm",
	"site_migration.cancel_migration",
	"site_migration.cancel_migration_confirm",
	"site_migration.source_restored_dns_reminder",
	"site_migration.target_ready_dns_reminder",
	"site_migration.custom_command_warning",
	"alert.saving",
	"alert.send_failed",
	"alert.sending",
	"alert.smtp_copy",
	"alert.smtp_copy_failed",
	"alert.smtp_copy_success",
	"alert.smtp_config_saved",
	"alert.smtp_import",
	"alert.smtp_import_cancelled",
	"alert.smtp_import_prompt",
	"alert.smtp_import_success",
	"alert.smtp_overwrite_confirm",
	"alert.test_send",
	"alert.type_backup_failed",
	"alert.type_cpu_high_load",
	"alert.type_cron_failed",
	"alert.type_disk_pressure",
	"alert.type_memory_low",
	"alert.type_oom",
	"alert.type_panel_update",
	"alert.type_remote_backup_failed",
	"alert.type_service_abnormal",
	"alert.type_site_unavailable",
	"alert.type_ssl_expire",
	"alert.type_system_update",
	"alert.type_website_expiry",
	"alert.type_wp_fake_search_bot",
	"alert.type_wp_sqli_probe",
	"alert.webhook_config_saved",
	"alert.webhook_url_required",
	"alert.wp_security_config_saved",
	"cron.confirm_delete",
	"cron.confirm_run_paused",
	"cron.deleted_success",
	"cron.job_created",
	"cron.job_run_success",
	"cron.job_running",
	"cron.job_updated",
	"cron.load_failed",
	"cron.load_failed_with_error",
	"cron.run",
	"cron.full_backup",
	"cron.smart_incremental",
	"cron.select_target_site",
	"cron.status_disabled",
	"cron.status_enabled",
	"cron.status_migration_locked",
	"cron.status_site_paused",
	"cron.task_type_command",
	"cron.task_type_file_backup",
	"cron.task_type_wp_cron",
	"cron.wp_cron_target",
	"cron.every_5_minutes",
	"database.backup_count",
	"database.collapse_backups",
	"database.never_backed_up",
	"database.no_databases",
	"database.no_search_results",
	"database.php_password_help",
	"database.refresh",
	"database.refreshing",
	"database.search_placeholder",
	"database.show_more_backups",
	"database.size_unavailable",
	"database.wordpress_password_help",
	"website.status_paused",
	"website.status_running",
	"website.status_migrated",
	"website.status_deleting",
	"website.restore_migrated",
	"website.restore_migrated_detail",
	"website.restore_migrated_confirm",
	"website.restore_migrated_success",
	"website.generic_php_site",
	"website.auto_detect",
	"website.detecting",
	"website.backup_auto",
	"website.backup_manual",
	"website.backup_list",
	"website.save_other_cdn_settings",
	"website.save_settings",
	"website.wp_optimization",
	"website.fastcgi_cache_title",
	"website.restore",
	"website.processing",
	"website.sync_db_info",
	"website.clear_database",
	"website.clearing",
	"website.backing_up",
	"website.manual_backup",
	"website.restoring",
	"website.upload_restore",
	"website.unknown",
	"website.save_apply",
	"website.enable_lock",
	"website.unlock",
	"website.file_locked",
	"website.file_unlocked",
	"website.installing",
	"website.install_companion_plugin",
	"website.updating",
	"website.update_companion_plugin",
	"website.rebuild_plugin_config",
	"website.rebuilding_plugin_config",
	"website.rebuild_plugin_config_confirm",
	"website.ssl_enabled",
	"website.ssl_not_enabled",
	"website.ssl_pending",
	"website.online_monitoring",
	"website.anomaly_monitoring",
	"website.monitoring_enabled",
	"website.monitoring_disabled",
	"website.not_applicable",
	"website.access_log_full",
	"website.access_log_error_only",
	"website.access_log_off",
	"website.enabled",
	"website.disabled",
	"website.never_expires",
	"website.delete_confirm",
	"website.pause_confirm",
	"website.delete_success",
	"website.delete_has_backups_warning",
	"website.delete_backup_check_failed_warning",
	"website.backup_conflict_db_count",
	"website.backup_conflict_file_count",
	"website.backup_conflict_auto_enabled",
	"website.backup_conflict_cron_jobs",
	"website.site_pause",
	"website.site_enable",
	"website.reinstall",
	"website.reinstalling",
	"website.reinstalling_with_domain",
	"website.reinstall_completed",
	"wp_fleet.backup",
	"wp_fleet.cache",
	"wp_fleet.empty",
	"wp_fleet.expires_at",
	"wp_fleet.filter_php",
	"wp_fleet.filter_wordpress",
	"wp_fleet.health_critical",
	"wp_fleet.health_healthy",
	"wp_fleet.health_unknown",
	"wp_fleet.health_warning",
	"wp_fleet.inventory_complete",
	"wp_fleet.inventory_failed",
	"wp_fleet.inventory_queued",
	"wp_fleet.inventory_running",
	"wp_fleet.inventory_stale",
	"wp_fleet.inventory_unknown",
	"wp_fleet.issue_inventory_failed",
	"wp_fleet.issue_inventory_stale",
	"wp_fleet.issue_inventory_uncollected",
	"wp_fleet.issue_site_error",
	"wp_fleet.issue_ssl_expired",
	"wp_fleet.issue_ssl_expiring",
	"wp_fleet.issue_ssl_expiry_unknown",
	"wp_fleet.issue_ssl_setup_failed",
	"wp_fleet.issue_updates_available",
	"wp_fleet.more_issues",
	"wp_fleet.never",
	"wp_fleet.no_matches",
	"wp_fleet.overview_failed",
	"wp_fleet.plugin_updates",
	"wp_fleet.search_too_long",
	"wp_fleet.showing_sites",
	"wp_fleet.showing_stale_overview",
	"wp_fleet.ssl_disabled",
	"wp_fleet.ssl_expired",
	"wp_fleet.ssl_expiring",
	"wp_fleet.ssl_expiry_unknown",
	"wp_fleet.ssl_pending_error",
	"wp_fleet.ssl_valid",
	"wp_fleet.status_active",
	"wp_fleet.status_creating",
	"wp_fleet.status_deleting",
	"wp_fleet.status_error",
	"wp_fleet.status_paused",
	"wp_fleet.theme_updates",
	"wp_fleet.loading",
	"wp_fleet.retry_load",
	"wp_fleet.bulk_refresh",
	"wp_fleet.bulk_refresh_running",
	"wp_fleet.bulk_refresh_progress",
	"wp_fleet.bulk_refresh_complete",
	"wp_fleet.bulk_refresh_failed",
	"wp_fleet.update_checks_disabled_warning",
	"wp_fleet.update_checks_disabled",
	"wp_fleet.update_checks_enabled",
	"wp_fleet.update_checks_enabled_saved",
	"wp_fleet.update_checks_disabled_saved",
	"wp_fleet.update_checks_save_failed",
	"ai_diagnostics.analyzing",
	"ai_diagnostics.chat_role_ai",
	"ai_diagnostics.chat_role_user",
	"ai_diagnostics.collapse_details",
	"ai_diagnostics.confidence_prefix",
	"ai_diagnostics.diagnosing",
	"ai_diagnostics.diagnosis_completed",
	"ai_diagnostics.diagnosis_failed",
	"ai_diagnostics.diagnosis_running",
	"ai_diagnostics.expand_details",
	"ai_diagnostics.followup_failed",
	"ai_diagnostics.followup_running",
	"ai_diagnostics.history_summary",
	"ai_diagnostics.reply_ready",
	"ai_diagnostics.report_title",
	"ai_diagnostics.select_site_for_history",
	"ai_diagnostics.risk_high",
	"ai_diagnostics.risk_low",
	"ai_diagnostics.risk_medium",
	"ai_diagnostics.risk_text_high",
	"ai_diagnostics.risk_text_low",
	"ai_diagnostics.risk_text_medium",
	"ai_diagnostics.risk_unknown",
	"ai_diagnostics.select_site_to_start",
	"ai_diagnostics.send_to_ai",
	"ai_diagnostics.start_diagnosis_button",
	"ai_diagnostics.status_completed",
	"ai_diagnostics.status_failed",
	"ai_diagnostics.status_running",
	"ai_diagnostics.status_waiting",
	"ai_diagnostics.symptom_cache_issue",
	"ai_diagnostics.symptom_db_connection",
	"ai_diagnostics.symptom_performance",
	"ai_diagnostics.symptom_site_500",
	"ai_diagnostics.symptom_ssl_failure",
	"ai_diagnostics.symptom_wp_admin_down",
	"ai_diagnostics.symptom_log_analysis",
	"ai_diagnostics.log_context_description",
	"ai_diagnostics.data_read_count",
	"ai_diagnostics.tool_runtime_config",
	"ai_diagnostics.tool_security_config",
	"ai_diagnostics.tool_log_overview",
	"ai_diagnostics.tool_log_status",
	"ai_diagnostics.tool_log_path",
	"ai_diagnostics.tool_log_bot",
	"ai_diagnostics.tool_log_ip",
	"ai_diagnostics.tool_log_category",
	"ai_diagnostics.waiting",
	"ai_diagnostics.waiting_ai_request",
	"ai_diagnostics.waiting_analysis",
	"ai_diagnostics.waiting_collect",
	"ai_diagnostics.waiting_long",
	"log_analysis.analysis_failed",
	"log_analysis.analysis_finished",
	"log_analysis.analysis_running",
	"log_analysis.interrupted_by_restart",
	"log_analysis.load_failed",
	"log_analysis.risk_high",
	"log_analysis.risk_low",
	"log_analysis.risk_medium",
	"log_analysis.start_failed",
	"log_analysis.traffic_explanation",
	"log_analysis.category_security_rejected",
	"log_analysis.category_identified_automation",
	"log_analysis.category_http_error",
	"log_analysis.category_wordpress_endpoint",
	"log_analysis.category_static_asset",
	"log_analysis.category_page_like",
	"log_analysis.category_other",
	"log_analysis.detail_ai",
	"log_analysis.detail_ai_failed",
	"log_analysis.detail_ai_running",
	"log_analysis.ai_session_wait_help",
	"log_analysis.continue_diagnosis",
	"log_analysis.detail_load_failed",
	"log_analysis.currently_banned",
	"log_analysis.not_currently_banned",
	"log_analysis.banned_in_range",
	"log_analysis.security_event_sensitive_file_scan",
	"log_analysis.security_event_sqli_probe",
	"log_analysis.security_event_fake_search_bot",
	"log_analysis.security_event_suspicious_php",
	"log_analysis.security_event_client_ip_spoof",
	"security.cdn_mode_cloudflare_auto",
	"security.cdn_mode_compatible_missing_origin_ips",
	"security.cdn_mode_compatible_no_origin_ips",
	"security.confirm_delete_cdn_group",
	"security.fetch_cdn_groups_failed",
	"security.got_it",
	"security.help_title",
	"security.last_update_label",
	"security.refresh_triggered",
	"security.googlebot_last_success",
	"security.googlebot_last_error",
	"security.googlebot_source_official",
	"security.googlebot_source_relay",
	"security.googlebot_source_manual",
	"security.googlebot_source_unknown",
	"security.telemetry_disable_confirm",
	"security.telemetry_disabled",
	"security.telemetry_url_required",
	"security.sqli_protection",
	"security.sqli_protection_help",
	"security.sqli_block_enabled",
	"security.sqli_autoban_enabled",
	"security.sqli_proxy_boundary",
	"security.sqli_ban_threshold",
	"security.sqli_ban_window",
	"security.telemetry_enabled",
	"security.cdn_mode_strict_trusted_ips",
	"common.saved",
	"files.chunk_upload_failed",
	"files.clipboard_copy",
	"files.clipboard_cut",
	"files.clipboard_label",
	"files.compress_completed",
	"files.compress_directory_title",
	"files.compress_file_title",
	"files.compression_busy",
	"files.confirm_delete_items",
	"files.confirm_extract",
	"files.confirm_fix_permissions",
	"files.confirm_overwrite_existing",
	"files.copied_to_clipboard",
	"files.current_selection",
	"files.cut_to_clipboard",
	"files.decompression_busy",
	"files.delete_completed",
	"files.delete_failed",
	"files.deleted_item",
	"files.existing_items_conflict",
	"files.extract_completed",
	"files.extract_overwrite_conflict",
	"files.file_type_dir",
	"files.file_type_file",
	"files.item_count",
	"files.mkdir_success",
	"files.pagination_summary",
	"files.paste_target_required",
	"files.permissions_fixed",
	"files.prompt_archive_name",
	"files.remote_import_completed",
	"files.remote_import_failed",
	"files.remote_import_in_progress",
	"files.remote_import_preparing",
	"files.remote_import_start",
	"files.remote_import_url_placeholder",
	"files.rename_success",
	"files.search_failed",
	"files.search_result_path",
	"files.search_results_for",
	"files.search_truncated_match_limit",
	"files.search_truncated_scan_limit",
	"files.skip_existing_prompt",
	"files.unknown_size",
	"files.upload",
	"files.upload_completed",
	"files.upload_failed_with_error",
	"files.upload_init_failed",
	"files.upload_merge_failed",
	"files.uploading",
	"firewall.added_to_blacklist",
	"firewall.analyzing",
	"firewall.ban",
	"firewall.ban_level_10m",
	"firewall.ban_level_24h",
	"firewall.ban_level_30d",
	"firewall.ban_level_empty",
	"firewall.ban_level_permanent",
	"firewall.ban_level_rate_limit",
	"firewall.ban_success",
	"firewall.banning",
	"firewall.clear_rescan",
	"firewall.confirm_clear_history",
	"firewall.confirm_permanent_ban",
	"firewall.confirm_unban",
	"firewall.copy_failed_manual",
	"firewall.copy_line_count",
	"firewall.copy_line_ip",
	"firewall.copy_line_last_seen",
	"firewall.copy_line_note",
	"firewall.copy_line_path",
	"firewall.copy_line_risk",
	"firewall.copy_line_site",
	"firewall.copy_line_source",
	"firewall.copy_line_status",
	"firewall.copy_line_type",
	"firewall.enter_ip",
	"firewall.expected_unban_at",
	"firewall.event_runtime_php_access",
	"firewall.event_suspicious_php_file",
	"firewall.event_integrity_added",
	"firewall.event_integrity_modified",
	"firewall.event_integrity_deleted",
	"firewall.event_integrity_link",
	"firewall.event_integrity_unavailable",
	"firewall.source_integrity",
	"firewall.file_event_copied",
	"firewall.file_security_summary",
	"firewall.found",
	"firewall.incremental_refresh",
	"firewall.ip_filled_notice",
	"firewall.load_wp_report_failed",
	"firewall.load_more_history",
	"firewall.permanent",
	"firewall.fail2ban_managed",
	"firewall.fail2ban_managed_help",
	"firewall.current_bans_partial_read",
	"firewall.ban_sync_anomalies",
	"firewall.ban_sync_anomalies_help",
	"firewall.ban_rule_missing",
	"firewall.ban_status_unverified",
	"firewall.metadata_unknown",
	"firewall.refresh_analysis",
	"firewall.refresh_file_security_failed",
	"firewall.refreshing",
	"firewall.report_copied",
	"firewall.risk_high",
	"firewall.risk_low",
	"firewall.risk_medium",
	"firewall.scanning",
	"firewall.source_404",
	"firewall.source_login",
	"firewall.source_sqli",
	"firewall.source_manual",
	"firewall.source_nftables",
	"firewall.source_nginx",
	"firewall.source_panel",
	"firewall.source_scan",
	"firewall.source_scanner",
	"firewall.source_ssh",
	"firewall.source_web_protect",
	"firewall.status_current_risk",
	"firewall.status_handled",
	"firewall.total_records",
	"firewall.type_fake_search_bot",
	"firewall.type_sensitive_file_scan",
	"firewall.type_sqli_probe",
	"firewall.type_sqli_blocked",
	"firewall.rule_summary_sqli",
	"firewall.rule_sqli_title",
	"firewall.rule_sqli_detail",
	"firewall.rule_sqli_boundary",
	"firewall.type_suspicious_php",
	"firewall.unbanned",
	"firewall.unknown",
	"firewall.view_source_rule",
	"name",
	"overwrite",
	"site_id",
	"size",
	"skip",
	"symptom",
	"textarea",
	"time",
	"type",
	"website.at_least_one_url",
	"website.backup_completed",
	"website.backup_settings_saved",
	"website.cache_cleared",
	"website.cdn_compatible_no_origin_ips",
	"website.cdn_header_mismatch",
	"website.cdn_realip_saved",
	"website.cdn_select_non_cloudflare_help",
	"website.cdn_select_non_cloudflare_required",
	"website.cdn_strict_origin_ips",
	"website.confirm_change",
	"website.confirm_clear_cache",
	"website.confirm_clear_database",
	"website.confirm_clear_logs",
	"website.confirm_continue",
	"website.confirm_delete_backup",
	"website.confirm_delete_ssl",
	"website.confirm_restore_backup",
	"website.confirm_restore_upload",
	"website.confirm_update",
	"website.confirm_update_site_urls",
	"website.database_cleared",
	"website.deleted",
	"website.detect_failed",
	"website.detect_multiple_prefixes",
	"website.detect_prefix_found",
	"website.detect_prefix_missing",
	"website.document_root_saved",
	"website.domain_required",
	"website.domain_change_resources_warning",
	"website.sync_wp_site_urls",
	"website.sync_wp_site_urls_help",
	"website.sync_wp_site_urls_enter_domain",
	"website.wp_site_urls_domain_mismatch",
	"website.wp_site_urls_read_failed",
	"website.expiry_saved",
	"website.file_lock_disable_confirm",
	"website.file_lock_disabled",
	"website.file_lock_apply_confirm",
	"website.file_lock_apply_failed",
	"website.file_lock_apply_mode",
	"website.file_lock_check_impact",
	"website.file_lock_checking",
	"website.file_lock_enable_confirm",
	"website.file_lock_enabled",
	"website.file_lock_executable_warning",
	"website.file_editing_saved",
	"website.file_lock_legacy",
	"website.file_lock_mode_standard",
	"website.file_lock_mode_strict",
	"website.file_lock_preview_truncated",
	"website.file_lock_path_advanced_cache",
	"website.file_lock_path_cache",
	"website.file_lock_path_db_dropin",
	"website.file_lock_path_languages",
	"website.file_lock_path_mu_plugins",
	"website.file_lock_path_object_cache",
	"website.file_lock_path_php_ini",
	"website.file_lock_path_plugins",
	"website.file_lock_path_sunrise",
	"website.file_lock_path_themes",
	"website.file_lock_path_uploads",
	"website.file_lock_path_user_ini",
	"website.file_lock_path_wflogs",
	"website.file_lock_path_wordfence_waf",
	"website.file_lock_path_wp_config",
	"website.file_lock_readonly_dirs",
	"website.file_lock_sensitive_files",
	"website.file_lock_standard",
	"website.file_lock_standard_help",
	"website.file_lock_strict",
	"website.file_lock_strict_help",
	"website.file_lock_symlink_warning",
	"website.file_lock_writable_dirs",
	"website.save_file_editing",
	"website.save_password_reset",
	"website.password_reset_saved",
	"website.installed",
	"website.load_current_url_failed",
	"website.load_failed_with_error",
	"website.load_log_files_failed",
	"website.load_logs_failed",
	"website.log_empty_or_missing",
	"website.log_type_access",
	"website.log_type_error",
	"website.log_type_security",
	"website.logs_cleared",
	"website.monitoring_saved",
	"website.new_password_placeholder",
	"website.operation_failed_with_error",
	"website.optimization_saved",
	"website.password_updated",
	"website.processing_update",
	"website.reading",
	"website.save_failed",
	"website.save_failed_with_error",
	"website.site_urls_updated",
	"website.wp_admin_confirm",
	"website.wp_admin_load_failed",
	"website.wp_admin_password_generated",
	"website.wp_admin_password_mismatch",
	"website.wp_admin_password_short",
	"website.wp_admin_required",
	"website.wp_admin_update_failed",
	"website.wp_admin_updated",
	"website.ssl_deleted",
	"website.ssl_export_saved",
	"website.ssl_manual_required",
	"website.sync_wp_config_base",
	"website.sync_wp_config_prefix",
	"website.unchanged",
	"website.update_failed",
	"website.update_failed_with_error",
	"website.updated",

	"2006-01-02",
	"20060102",
	"20060102_150405",
	"2d",
	"Cache-Control",
	"Content-Disposition",
	"Content-Type",
	"ai_diagnostics.already_running",
	"ai_diagnostics.already_running_followup",
	"ai_diagnostics.api_key_required",
	"ai_diagnostics.build_context_failed",
	"ai_diagnostics.collect_context_failed",
	"ai_diagnostics.create_session_failed",
	"ai_diagnostics.followup_required",
	"ai_diagnostics.followup_too_long",
	"ai_diagnostics.followup_wait",
	"ai_diagnostics.invalid_session_id",
	"ai_diagnostics.invalid_symptom",
	"ai_diagnostics.load_context_failed",
	"ai_diagnostics.load_messages_failed",
	"ai_diagnostics.load_sessions_failed",
	"ai_diagnostics.not_enabled",
	"ai_diagnostics.result_ready",
	"ai_diagnostics.save_followup_failed",
	"ai_diagnostics.save_reply_failed",
	"ai_diagnostics.save_result_failed",
	"ai_diagnostics.session_interrupted",
	"ai_diagnostics.session_not_found",
	"ai_diagnostics.user_error_bad_response",
	"ai_diagnostics.user_error_empty_response",
	"ai_diagnostics.user_error_network",
	"ai_diagnostics.user_error_rate_limited",
	"ai_diagnostics.user_error_timeout",
	"ai_diagnostics.user_error_unauthorized",
	"ai_settings.invalid_provider",
	"ai_settings.load_failed",
	"ai_settings.save_failed",
	"auth.invalid_credentials",
	"auth.missing_csrf",
	"auth.not_logged_in",
	"auth.provide_credentials",
	"common.invalid_params",
	"files.path_out_of_bounds",
	"files.remote_import_chmod_failed",
	"files.remote_import_completed_fix_permissions",
	"files.remote_import_create_file_failed",
	"files.remote_import_disk_full",
	"files.remote_import_disk_space_low",
	"files.remote_import_downloading",
	"files.remote_import_failed_with_error",
	"files.remote_import_filename_required",
	"files.remote_import_host_invalid",
	"files.remote_import_host_local",
	"files.remote_import_host_private",
	"files.remote_import_host_resolve_failed",
	"files.remote_import_https_only",
	"files.remote_import_read_failed",
	"files.remote_import_rename_failed",
	"files.remote_import_request_failed",
	"files.remote_import_save_failed",
	"files.remote_import_status_code",
	"files.remote_import_task_missing",
	"files.remote_import_too_large",
	"files.remote_import_too_many_redirects",
	"files.remote_import_url_invalid",
	"files.remote_import_url_no_userinfo",
	"files.remote_import_url_required",
	"files.remote_import_waiting",
	"files.select_website_first",
	"files.target_directory_missing",
	"files.target_is_directory",
	"session_username",
	"ai_development.disable_confirm",
	"ai_development.disabled",
	"ai_development.disabled_status",
	"ai_development.enable_button",
	"ai_development.enable_confirm",
	"ai_development.enable_busy_force_confirm",
	"ai_development.enabled",
	"ai_development.enabled_downloaded",
	"ai_development.processing",
	"ai_development.stage_installing_wp_cli",
	"ai_development.stage_installing_nodejs",
	"ai_development.stage_configuring_access",
	"ai_development.stage_rotating_package",
	"ai_development.enabled_ready_download",
	"ai_development.first_prompt",
	"ai_development.first_prompt_copied",
	"ai_development.download_package",
	"ai_development.regenerate_package",
	"ai_development.package_downloaded",
	"ai_development.redownload_rotates_confirm",
	"ai_development.rotate_confirm",
	"ai_development.rotated_downloaded",
	"software.action_success",
	"software.clear_failed",
	"software.client_max_body_size_hint",
	"software.client_max_body_size_label",
	"software.config_not_found",
	"software.config_updated_reloaded",
	"software.create_php_config_failed",
	"software.innodb_buffer_pool_size_hint",
	"software.innodb_buffer_pool_size_label",
	"software.installed",
	"software.development_tool_install_confirm",
	"software.development_tool_installed",
	"software.install",
	"software.installing",
	"software.nodejs_development_help",
	"software.recommended",
	"software.wp_cli_development_help",
	"software.invalid_action",
	"software.log_cleared",
	"software.log_empty_or_unreadable",
	"software.max_execution_time_hint",
	"software.max_execution_time_label",
	"software.max_input_time_hint",
	"software.max_input_time_label",
	"software.max_input_vars_hint",
	"software.max_input_vars_label",
	"software.maxmemory_hint",
	"software.maxmemory_label",
	"software.memory_limit_hint",
	"software.memory_limit_label",
	"software.nginx_value_no_semicolon",
	"software.operation_failed_with_error",
	"software.php_installed_extensions",
	"software.php_int_invalid",
	"software.php_pool_rebuild_failed",
	"software.php_size_invalid",
	"software.post_max_size_hint",
	"software.post_max_size_label",
	"software.read_config_failed",
	"software.running",
	"software.stopped",
	"software.syntax_check_failed_with_rollback",
	"software.unknown_software",
	"software.unsupported_config_item",
	"software.upload_max_filesize_hint",
	"software.upload_max_filesize_label",
	"software.value_no_newline",
	"software.write_config_failed",
	"website.invalid_site_id",
	"website.not_found",
	"website.restore_failed",
	"website.restore_file_invalid",
	"website.restore_long_running_help",
	"website.restore_running_elapsed",
	"website.restore_started",
	"website.restore_still_running",
	"website.restore_success",
	"website.restore_success_elapsed",
	"website.restore_task_failed",
	"website.restore_uploading",
	"website.restore_waiting",
	"wp_inventory.active",
	"wp_inventory.active_plugins",
	"wp_inventory.core_update_available",
	"wp_inventory.core_update_none",
	"wp_inventory.current_theme_badge",
	"wp_inventory.error_bootstrap",
	"wp_inventory.error_code",
	"wp_inventory.error_generic",
	"wp_inventory.error_inventory_limit",
	"wp_inventory.error_memory_limit",
	"wp_inventory.error_output_limit",
	"wp_inventory.error_policy",
	"wp_inventory.error_policy_mismatch",
	"wp_inventory.error_protocol",
	"wp_inventory.error_site_changed",
	"wp_inventory.error_start_failed",
	"wp_inventory.error_terminated",
	"wp_inventory.error_timeout",
	"wp_inventory.error_unknown",
	"wp_inventory.error_worker",
	"wp_inventory.how_it_works_title",
	"wp_inventory.inactive",
	"wp_inventory.load_failed",
	"wp_inventory.loading",
	"wp_inventory.network_active",
	"wp_inventory.never",
	"wp_inventory.next",
	"wp_inventory.page_summary",
	"wp_inventory.previous",
	"wp_inventory.refresh",
	"wp_inventory.refresh_created",
	"wp_inventory.refresh_existing",
	"wp_inventory.refresh_failed",
	"wp_inventory.refreshing",
	"wp_inventory.retry",
	"wp_inventory.retry_hint",
	"wp_inventory.retry_useless",
	"wp_inventory.stale_notice",
	"wp_inventory.status_complete",
	"wp_inventory.status_failed_empty",
	"wp_inventory.status_failed_stale",
	"wp_inventory.status_queued",
	"wp_inventory.status_running",
	"wp_inventory.status_unexpected",
	"wp_inventory.status_unknown",
	"wp_inventory.status_unknown_updates",
	"wp_inventory.status_unknown_updates_label",
	"wp_inventory.status_up_to_date",
	"wp_inventory.status_updates_available",
	"wp_inventory.task_read_failed",
	"wp_inventory.tech_details",
	"wp_inventory.type_core",
	"wp_inventory.type_plugin",
	"wp_inventory.type_theme",
	"wp_inventory.wordpress_only",
	"wp_inventory.yes",
	"wp_inventory.no",
	"wp_core_update.preparing",
	"wp_core_update.recheck",
	"wp_core_update.confirm_message",
	"wp_core_update.preview_failed",
	"wp_core_update.preview_invalid",
	"wp_core_update.stage_backups_ready",
	"wp_core_update.stage_claimed",
	"wp_core_update.stage_complete",
	"wp_core_update.stage_health_check",
	"wp_core_update.stage_queued",
	"wp_core_update.stage_restoring_file_lock",
	"wp_core_update.stage_rollback",
	"wp_core_update.stage_unknown",
	"wp_core_update.stage_unlocking",
	"wp_core_update.stage_updating_core",
	"wp_core_update.stage_value",
	"wp_core_update.start_update",
	"wp_core_update.cancel_preparation",
	"wp_core_update.recent_backup_found",
	"wp_core_update.status_failed",
	"wp_core_update.status_interrupted_unknown",
	"wp_core_update.status_queued",
	"wp_core_update.status_running",
	"wp_core_update.status_success",
	"wp_core_update.status_unknown",
	"wp_core_update.submit_failed",
	"wp_core_update.submitted",
	"wp_core_update.submission_recovered",
	"wp_core_update.submitting",
	"wp_core_update.task_invalid",
	"wp_core_update.task_read_failed",
	"wp_core_update.up_to_date",
	"wp_plugin_update.action",
	"wp_plugin_update.checking",
	"wp_plugin_update.confirm",
	"wp_plugin_update.confirm_warning",
	"wp_plugin_update.cancel_preparation",
	"wp_plugin_update.description",
	"wp_plugin_update.not_found",
	"wp_plugin_update.not_in_repository",
	"wp_plugin_update.license_invalid",
	"wp_plugin_update.plugin",
	"wp_plugin_update.preview_failed",
	"wp_plugin_update.preview_invalid",
	"wp_plugin_update.recent_backup_found",
	"wp_plugin_update.backup_scope_value",
	"wp_plugin_update.rescan_after_success",
	"wp_plugin_update.stage_backups_ready",
	"wp_plugin_update.stage_claimed",
	"wp_plugin_update.stage_complete",
	"wp_plugin_update.stage_health_check",
	"wp_plugin_update.stage_queued",
	"wp_plugin_update.stage_reactivating",
	"wp_plugin_update.stage_restoring_file_lock",
	"wp_plugin_update.stage_rollback",
	"wp_plugin_update.stage_unknown",
	"wp_plugin_update.stage_unlocking",
	"wp_plugin_update.stage_updating_component",
	"wp_plugin_update.stage_value",
	"wp_plugin_update.status_failed",
	"wp_plugin_update.status_interrupted_unknown",
	"wp_plugin_update.status_preparing",
	"wp_plugin_update.status_queued",
	"wp_plugin_update.status_running",
	"wp_plugin_update.status_success",
	"wp_plugin_update.status_unknown",
	"wp_plugin_update.submit_failed",
	"wp_plugin_update.submitted",
	"wp_plugin_update.submission_recovered",
	"wp_plugin_update.submitting",
	"wp_plugin_update.task_invalid",
	"wp_plugin_update.task_read_failed",
	"wp_plugin_update.tracking",
	"wp_plugin_update.title",
	"wp_plugin_batch.button",
	"wp_plugin_batch.button_tracking",
	"wp_plugin_batch.theme_not_supported",
	"wp_plugin_batch.ignore_button",
	"wp_plugin_batch.ignore_confirm",
	"wp_plugin_batch.ignore_failed",
	"wp_plugin_batch.ignore_success",
	"wp_plugin_batch.item_dispatch_failed",
	"wp_plugin_batch.item_failed_awaiting_decision",
	"wp_plugin_batch.item_failed_generic",
	"wp_plugin_batch.item_failed_rollback_failed",
	"wp_plugin_batch.item_failed_rolled_back",
	"wp_plugin_batch.item_interrupted",
	"wp_plugin_batch.item_interrupted_acknowledged",
	"wp_plugin_batch.item_pending",
	"wp_plugin_batch.item_success",
	"wp_plugin_batch.item_updating",
	"wp_plugin_batch.load_failed",
	"wp_plugin_batch.rollback_button",
	"wp_plugin_batch.rollback_confirm",
	"wp_plugin_batch.rollback_reuse_confirm",
	"wp_plugin_batch.rollback_failed",
	"wp_plugin_batch.rollback_submitting",
	"wp_plugin_batch.rollback_success",
	"wp_plugin_batch.selected_count",
	"wp_plugin_batch.start",
	"wp_plugin_batch.start_failed",
	"wp_plugin_batch.start_invalid",
	"wp_plugin_batch.starting",
	"wp_plugin_batch.status_completed",
	"wp_plugin_batch.status_running",
	"wp_plugin_batch.task_invalid",
	"wp_theme_update.action",
	"wp_theme_update.backup_scope_value",
	"wp_theme_update.checking",
	"wp_theme_update.confirm",
	"wp_theme_update.confirm_warning",
	"wp_theme_update.current_theme_warning",
	"wp_theme_update.description",
	"wp_theme_update.not_found",
	"wp_theme_update.not_in_repository",
	"wp_theme_update.license_invalid",
	"wp_theme_update.preview_failed",
	"wp_theme_update.preview_invalid",
	"wp_theme_update.rescan_after_success",
	"wp_theme_update.stage_backups_ready",
	"wp_theme_update.stage_claimed",
	"wp_theme_update.stage_complete",
	"wp_theme_update.stage_health_check",
	"wp_theme_update.stage_queued",
	"wp_theme_update.stage_reactivating",
	"wp_theme_update.stage_restoring_file_lock",
	"wp_theme_update.stage_rollback",
	"wp_theme_update.stage_unknown",
	"wp_theme_update.stage_unlocking",
	"wp_theme_update.stage_updating_component",
	"wp_theme_update.status_failed",
	"wp_theme_update.status_interrupted_unknown",
	"wp_theme_update.status_preparing",
	"wp_theme_update.status_queued",
	"wp_theme_update.status_running",
	"wp_theme_update.status_success",
	"wp_theme_update.status_unknown",
	"wp_theme_update.submit_failed",
	"wp_theme_update.submitted",
	"wp_theme_update.submission_recovered",
	"wp_theme_update.submitting",
	"wp_theme_update.task_invalid",
	"wp_theme_update.task_read_failed",
	"wp_theme_update.theme",
	"wp_theme_update.title",
	"wp_theme_update.tracking",
	"wp_update_backup.kind_database",
	"wp_update_backup.kind_core_files",
	"wp_update_backup.kind_plugin_files",
	"wp_update_backup.kind_theme_files",
	"wp_update_backup.load_failed",
	"wp_update_backup.restore",
	"wp_update_backup.restore_failed",
	"wp_update_backup.restoring",
	"wp_update_backup.restore_confirm",
	"wp_update_backup.restore_batch_confirm",
	"wp_update_backup.batch_shared",
	"wp_update_backup.restore_started",
	"wp_update_backup.restore_success",
	"wp_update_backup.restore_task_failed",
	"wp_update_backup.restore_status_failed",
	"wp_update_log.load_failed",
	"wp_update_log.copy",
	"wp_update_log.copied",
	"wp_update_log.copy_failed",
	"wp_update_log.copy_title",
	"wp_update_log.copy_site",
	"wp_update_log.copy_task",
	"wp_update_log.copy_type",
	"wp_update_log.copy_component",
	"wp_update_log.copy_version",
	"wp_update_log.copy_status",
	"wp_update_log.copy_rollback",
	"wp_update_log.copy_started",
	"wp_update_log.copy_finished",
	"wp_update_log.copy_events_divider",
	"wp_update_log.kind_update",
	"wp_update_log.kind_rollback",
	"wp_update_log.requires_attention",
	"wp_update_log.rollback_failed",
	"wp_update_log.status_success",
	"wp_update_log.status_failed",
	"wp_update_log.status_running",
	"wp_update_log.status_queued",
	"wp_update_log.status_preparing",
	"wp_update_log.status_interrupted_unknown",
	"wp_update_log.no_events",
	"wp_update_log.event_result_info",
	"wp_update_log.event_result_success",
	"wp_update_log.event_result_failed",
	"wp_update_log.event_result_interrupted",
	"wp_update_log.event_result_manual",
}

func SetupRouter(cfg *config.Config, tmplFS embed.FS, staticFS embed.FS, version string, configPath string) *gin.Engine {
	panelVersion = version
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.SetTrustedProxies(nil)

	r.Use(middleware.CustomRecovery())
	r.Use(middleware.SecurityHeaders())

	// /healthz 必须在 ScanDefense 之前注册，否则本机健康检查会被扫描防御误封
	r.GET("/healthz", func(c *gin.Context) {
		ip := net.ParseIP(c.ClientIP())
		if ip == nil || !ip.IsLoopback() {
			c.Status(http.StatusNotFound)
			return
		}
		db := database.GetDB()
		if db == nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}
		var schemaVersion string
		if err := db.QueryRow("SELECT version FROM schema_version ORDER BY updated_at DESC, rowid DESC LIMIT 1").Scan(&schemaVersion); err != nil || schemaVersion == "" {
			c.Status(http.StatusServiceUnavailable)
			return
		}
		for _, table := range []string{"admin_users", "websites", "security_settings"} {
			var count int
			if err := db.QueryRow("SELECT COUNT(*) FROM " + table + " LIMIT 1").Scan(&count); err != nil {
				c.Status(http.StatusServiceUnavailable)
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "version": version})
	})

	db := database.GetDB()
	r.Use(middleware.ScanDefense(db, cfg.Panel.RandomSuffix))

	siteMigrationPairing, err := executor.NewSiteMigrationPairingService(db, version, cfg.Panel.TLSCertPath)
	if err != nil {
		panic(err)
	}
	siteMigrationSource, err := executor.NewSiteMigrationSourceService(db, siteMigrationPairing)
	if err != nil {
		panic(err)
	}
	siteMigrationRoot := cfg.Panel.DataDir
	if siteMigrationRoot == "" {
		siteMigrationRoot = os.TempDir()
	}
	siteMigrationPlanner, err := executor.NewSiteMigrationBatchPlanner(db, siteMigrationRoot)
	if err != nil {
		panic(err)
	}
	var siteMigrationWorkflow *executor.SiteMigrationWorkflowService
	var siteMigrationControl *executor.SiteMigrationControlService
	if siteMigrationPreparation, preparationErr := executor.NewSiteMigrationSourcePreparationService(db, cfg, siteMigrationPairing); preparationErr == nil {
		siteMigrationWorkflow, err = executor.NewSiteMigrationWorkflowService(db, cfg, siteMigrationPairing, siteMigrationPreparation)
		if err == nil {
			siteMigrationControl, err = executor.NewSiteMigrationControlService(db, cfg, siteMigrationPairing, siteMigrationWorkflow, filepath.Join(siteMigrationRoot, "site-migration", "target"))
		}
		if err != nil {
			log.Printf("网站搬家操作服务未启用: %v", err)
		}
	} else {
		log.Printf("网站搬家操作服务未启用: %v", preparationErr)
	}
	siteMigrationHandler := &handlers.SiteMigrationHandler{Service: siteMigrationPairing, Source: siteMigrationSource, Planner: siteMigrationPlanner, Workflow: siteMigrationWorkflow, Control: siteMigrationControl, DB: db, Version: version}
	// Install the body/authentication guard globally (it is a no-op outside the
	// machine API namespace) so Gin's 404/405 paths cannot bypass slow-body
	// deadlines by using an unknown endpoint or the wrong HTTP method.
	r.Use(middleware.SiteMigrationFailureLimit())
	r.Use(middleware.SiteMigrationRequestGuard(siteMigrationPairing))
	migrationMachine := r.Group("/api/site-migration/v1")
	migrationMachine.POST("/pair/redeem", siteMigrationHandler.Redeem)
	migrationMachine.POST("/pair/challenge", siteMigrationHandler.Challenge)
	migrationMachine.POST("/peer/revoke", siteMigrationHandler.MachineRevokePeer)
	migrationMachine.POST("/preflight", siteMigrationHandler.MachinePreflight)
	migrationMachine.POST("/target/batches", siteMigrationHandler.MachineCreateTargetBatch)
	migrationMachine.POST("/target/batches/queue", siteMigrationHandler.MachineQueueTargetBatch)
	migrationMachine.POST("/source/manifest", siteMigrationHandler.SourceManifest)
	migrationMachine.POST("/source/chunk", siteMigrationHandler.SourceChunk)
	migrationMachine.POST("/source/file-shard", siteMigrationHandler.SourceFileShard)
	migrationMachine.POST("/source/database", siteMigrationHandler.SourceDatabase)
	migrationMachine.POST("/source/database-chunk", siteMigrationHandler.SourceDatabaseChunk)
	migrationMachine.POST("/source/certificates", siteMigrationHandler.SourceCertificates)
	migrationMachine.POST("/source/certificate-chunk", siteMigrationHandler.SourceCertificateChunk)
	migrationMachine.POST("/source/settings", siteMigrationHandler.SourceSettings)
	migrationMachine.POST("/target/status", siteMigrationHandler.MachineTargetStatus)
	migrationMachine.POST("/target/retry", siteMigrationHandler.MachineTargetRetry)
	migrationMachine.POST("/target/delete-task", siteMigrationHandler.MachineDeleteTargetTask)
	migrationMachine.POST("/source/delete-task", siteMigrationHandler.MachineDeleteSourceTask)

	attemptTracker := middleware.NewLoginAttemptTracker(
		db,
		cfg.Security.MaxLoginAttempts,
		cfg.Security.AttemptWindowMinutes,
		cfg.Security.BanDurationHours,
	)

	basicAuthChecker := &middleware.BasicAuthChecker{
		RecordAttempt: attemptTracker.RecordAttempt,
		IsBanned:      attemptTracker.IsBanned,
	}

	staticPrefix := "/" + cfg.Panel.RandomSuffix + "/assets"
	staticFileSystem, _ := fs.Sub(staticFS, "static")
	r.StaticFS(staticPrefix, http.FS(staticFileSystem))

	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusNotFound, "Not Found")
	})
	r.GET("/favicon.ico", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	suffix := cfg.Panel.RandomSuffix
	prefix := "/" + suffix

	panelGroup := r.Group(prefix)
	panelGroup.Use(middleware.RandomPath(suffix))
	panelGroup.Use(middleware.BasicAuth(basicAuthChecker))

	// 面板根路径重定向到登录页（解决用户访问面板地址不带 /login 的问题）
	panelGroup.GET("", func(c *gin.Context) {
		if c.Request.URL.Path == "/"+suffix {
			c.Redirect(http.StatusFound, "/"+suffix+"/login")
			return
		}
		c.Next()
	})

	panelGroup.GET("/login", func(c *gin.Context) {
		i18n.MaybeSetLanguageCookie(c.Writer, c.Request)
		lang := i18n.LangFromRequest(c.Request)
		if !middleware.SetCSRFToken(c) {
			return
		}
		csrfToken := middleware.GetCSRFToken(c)
		c.HTML(http.StatusOK, "login.html", gin.H{
			"Title":        i18n.T(lang, "auth.login"),
			"PanelTitle":   handlers.GetPanelTitle(),
			"PanelVersion": version,
			"AssetVersion": version,
			"RandomSuffix": suffix,
			"Active":       "login",
			"AssetPrefix":  prefix + "/assets",
			"CSRFToken":    csrfToken,
			"Lang":         lang,
			"MessagesJSON": i18n.MessagesJSON(lang, i18nKeys),
		})
	})

	panelGroup.POST("/api/auth/login", middleware.CSRF(), func(c *gin.Context) {
		authHandler := &handlers.AuthHandler{DB: db, Prefix: suffix, Tracker: attemptTracker}
		authHandler.Login(c)
	})

	cacheHelper := &handlers.CacheHelperHandler{}
	pluginImageOptimizer := &handlers.ImageOptimizerHandler{}

	pluginGroup := r.Group(prefix)
	pluginGroup.Use(middleware.RandomPath(suffix))
	pluginGroup.GET("/api/sites/find", cacheHelper.FindByDomain)
	pluginGroup.POST("/api/sites/companion/update", cacheHelper.UpdateCompanionPlugin)
	maintenanceHandler := &handlers.MaintenanceHandler{}
	pluginGroup.GET("/api/sites/maintenance", maintenanceHandler.Plugin)
	pluginGroup.POST("/api/sites/maintenance/:action", maintenanceHandler.Plugin)
	pluginGroup.GET("/api/sites/ssl/export", cacheHelper.ExportSSLCertificate)
	pluginGroup.DELETE("/api/sites/clear-cache", cacheHelper.ClearByDomain)
	pluginGroup.PUT("/api/sites/cache-settings", cacheHelper.UpdateCacheSettings)
	pluginGroup.PUT("/api/sites/optimizer-settings", cacheHelper.UpdateOptimizerSettings)
	pluginGroup.POST("/api/sites/image-optimizer/start", pluginImageOptimizer.PluginStart)
	pluginGroup.GET("/api/sites/image-optimizer/status", pluginImageOptimizer.PluginStatus)
	pluginGroup.POST("/api/sites/image-optimizer/stop", pluginImageOptimizer.PluginStop)

	protected := panelGroup.Group("")
	protected.Use(middleware.SessionRequired())
	protected.Use(func(c *gin.Context) {
		if !middleware.SetCSRFToken(c) {
			return
		}
		c.Next()
	})
	protected.Use(middleware.CSRF())
	protected.GET("/api/websites/:id/maintenance", maintenanceHandler.Panel)
	protected.PUT("/api/websites/:id/maintenance", maintenanceHandler.Panel)
	protected.POST("/api/websites/:id/maintenance/relock", maintenanceHandler.Panel)
	protected.POST("/api/websites/:id/maintenance/password", maintenanceHandler.Password)
	anomalyHandler := &handlers.WPAnomalyHandler{Monitor: executor.DefaultWPAnomalyMonitor(cfg)}
	protected.GET("/api/websites/:id/anomaly-monitor", anomalyHandler.Handle)
	protected.PUT("/api/websites/:id/anomaly-monitor", anomalyHandler.Handle)
	protected.POST("/api/websites/:id/anomaly-monitor/check", anomalyHandler.Handle)

	// Adminer has its own CSRF tokens. Keep it behind both panel authentication
	// layers, but do not apply the panel API CSRF header requirement to its HTML forms.
	adminerTool := panelGroup.Group("")
	adminerTool.Use(middleware.SessionRequired())
	adminerHandler := &handlers.AdminerHandler{}
	adminerTool.Any("/tools/adminer/:id", adminerHandler.Proxy)
	adminerTool.Any("/tools/adminer/:id/*path", adminerHandler.Proxy)

	authHandler := &handlers.AuthHandler{DB: db, Prefix: suffix, Tracker: attemptTracker}
	protected.POST("/api/auth/logout", authHandler.Logout)
	protected.GET("/api/auth/check", authHandler.Check)
	protected.GET("/api/auth/csrf-token", authHandler.CSRFToken)
	protected.POST("/api/site-migration/pairing-package", siteMigrationHandler.GeneratePackage)
	protected.POST("/api/site-migration/peers/connect", siteMigrationHandler.Connect)
	protected.GET("/api/site-migration/peers", siteMigrationHandler.ListPeers)
	protected.GET("/api/site-migration/tasks", siteMigrationHandler.ListTasks)
	protected.DELETE("/api/site-migration/peers/:id", siteMigrationHandler.RevokePeer)
	protected.POST("/api/site-migration/peers/:id/preflight", siteMigrationHandler.RemotePreflight)
	protected.POST("/api/site-migration/start", siteMigrationHandler.Start)
	protected.POST("/api/site-migration/estimate", siteMigrationHandler.Estimate)
	protected.POST("/api/site-migration/tasks/:id/retry", siteMigrationHandler.Retry)
	protected.POST("/api/site-migration/tasks/:id/complete", siteMigrationHandler.CompleteSource)
	protected.POST("/api/site-migration/tasks/:id/restore", siteMigrationHandler.RestoreSource)
	protected.DELETE("/api/site-migration/tasks/:id", siteMigrationHandler.DeleteTask)

	websiteHandler := &handlers.WebsiteHandler{DB: db}
	wpInventoryHandler := &handlers.WPInventoryHandler{DB: db}
	wpFleetOverviewHandler := &handlers.WPFleetOverviewHandler{DB: db}
	wpCoreUpdateHandler := newWPCoreUpdateHandler(db, cfg.Panel.BackupDir)
	wpPluginUpdateHandler := newWPPluginUpdateHandler(db, cfg.Panel.BackupDir)
	wpPluginBatchHandler := newWPPluginBatchHandler(db, cfg.Panel.BackupDir, cfg.Paths.WWWRoot)
	wpThemeUpdateHandler := newWPThemeUpdateHandler(db, cfg.Panel.BackupDir)
	wpUpdateBackupHandler := &handlers.WPUpdateBackupHandler{BackupDir: cfg.Panel.BackupDir}
	wpUpdateLogHandler := &handlers.WPUpdateLogHandler{}
	protected.GET("/api/websites", websiteHandler.List)
	protected.GET("/api/wp-fleet/overview", wpFleetOverviewHandler.Overview)
	protected.POST("/api/wp-fleet/inventory-refresh", wpFleetOverviewHandler.RefreshAll)
	protected.POST("/api/websites", websiteHandler.Create)
	protected.POST("/api/websites/ssl-preflight", websiteHandler.SSLPreflight)
	protected.GET("/api/websites/:id", websiteHandler.Get)
	protected.GET("/api/websites/:id/wp-inventory", wpInventoryHandler.Summary)
	protected.POST("/api/websites/:id/wp-inventory/refresh", wpInventoryHandler.Refresh)
	protected.GET("/api/websites/:id/wp-inventory/tasks/:task_id", wpInventoryHandler.Task)
	protected.GET("/api/websites/:id/wp-inventory/components", wpInventoryHandler.Components)
	protected.GET("/api/websites/:id/wp-inventory/updates", wpInventoryHandler.Updates)
	protected.GET("/api/websites/:id/wp-core-update/preview", wpCoreUpdateHandler.Preview)
	protected.POST("/api/websites/:id/wp-core-update/confirm", wpCoreUpdateHandler.Confirm)
	protected.GET("/api/websites/:id/wp-core-update/tasks/latest", wpCoreUpdateHandler.LatestTask)
	protected.GET("/api/websites/:id/wp-core-update/tasks/:task_id", wpCoreUpdateHandler.Task)
	protected.GET("/api/websites/:id/wp-plugin-update/preview", wpPluginUpdateHandler.Preview)
	protected.POST("/api/websites/:id/wp-plugin-update/confirm", wpPluginUpdateHandler.Confirm)
	protected.GET("/api/websites/:id/wp-plugin-update/tasks/latest", wpPluginUpdateHandler.LatestTask)
	protected.GET("/api/websites/:id/wp-plugin-update/tasks/:task_id", wpPluginUpdateHandler.Task)
	protected.POST("/api/websites/:id/wp-plugin-batch", wpPluginBatchHandler.Create)
	protected.GET("/api/websites/:id/wp-plugin-batch", wpPluginBatchHandler.List)
	protected.GET("/api/websites/:id/wp-plugin-batch/:batch_id", wpPluginBatchHandler.Get)
	protected.POST("/api/websites/:id/wp-plugin-batch/tasks/:task_id/rollback", wpPluginBatchHandler.Rollback)
	protected.POST("/api/websites/:id/wp-plugin-batch/tasks/:task_id/ignore", wpPluginBatchHandler.Ignore)
	protected.GET("/api/websites/:id/wp-theme-update/preview", wpThemeUpdateHandler.Preview)
	protected.POST("/api/websites/:id/wp-theme-update/confirm", wpThemeUpdateHandler.Confirm)
	protected.GET("/api/websites/:id/wp-theme-update/tasks/latest", wpThemeUpdateHandler.LatestTask)
	protected.GET("/api/websites/:id/wp-theme-update/tasks/:task_id", wpThemeUpdateHandler.Task)
	protected.GET("/api/websites/:id/wp-update-backups", wpUpdateBackupHandler.List)
	protected.POST("/api/websites/:id/wp-update-backups/:backup_id/restore", wpUpdateBackupHandler.Restore)
	protected.GET("/api/websites/:id/wp-update-logs", wpUpdateLogHandler.List)
	protected.DELETE("/api/websites/:id", websiteHandler.Delete)
	protected.GET("/api/websites/:id/backup-usage", websiteHandler.BackupUsage)
	protected.PATCH("/api/websites/:id/status", websiteHandler.ToggleStatus)
	protected.GET("/api/websites/:id/ssl/renewal", websiteHandler.SSLRenewal)
	protected.PUT("/api/websites/:id/ssl/renewal", websiteHandler.SSLRenewal)
	protected.POST("/api/websites/:id/ssl", websiteHandler.EnableSSL)
	protected.GET("/api/websites/:id/ssl/download", websiteHandler.DownloadSSLPackage)
	protected.PUT("/api/websites/:id/ssl/export", websiteHandler.SetSSLExport)
	protected.DELETE("/api/websites/:id/ssl", websiteHandler.RemoveSSL)
	protected.PUT("/api/websites/:id/db-password", websiteHandler.ChangeDBPassword)
	protected.POST("/api/websites/:id/fix-wp-config", websiteHandler.FixWPConfig)
	protected.GET("/api/websites/:id/detect-table-prefix", websiteHandler.DetectDBTablePrefix)
	protected.GET("/api/websites/:id/wp-site-urls", websiteHandler.GetWPSiteURLs)
	protected.PUT("/api/websites/:id/wp-site-urls", websiteHandler.UpdateWPSiteURLs)
	protected.GET("/api/websites/:id/wp-administrators", websiteHandler.ListWPAdministrators)
	protected.PUT("/api/websites/:id/wp-administrators", websiteHandler.UpdateWPAdministrator)
	protected.GET("/api/websites/:id/logs", websiteHandler.ViewLogs)
	protected.GET("/api/websites/:id/log-files", websiteHandler.ListLogFiles)
	protected.GET("/api/websites/:id/logs/download", websiteHandler.DownloadLogFile)
	protected.DELETE("/api/websites/:id/logs", websiteHandler.ClearLogs)
	protected.PUT("/api/websites/:id/domains", websiteHandler.UpdateDomains)
	protected.PUT("/api/websites/:id/cache", websiteHandler.UpdateCache)
	protected.DELETE("/api/websites/:id/cache", websiteHandler.ClearCache)
	protected.PUT("/api/websites/:id/wp-optimizations", websiteHandler.SaveWPOptimizations)
	protected.PUT("/api/websites/:id/wp-update-checks", websiteHandler.SetWPUpdateChecks)
	protected.PUT("/api/websites/:id/file-editor", websiteHandler.SetFileEditingProtection)
	protected.PUT("/api/websites/:id/password-reset", websiteHandler.SetPasswordResetMode)
	protected.PUT("/api/websites/:id/file-lock", websiteHandler.SetFileLock)
	protected.GET("/api/websites/:id/file-lock/preview", websiteHandler.PreviewFileLock)
	protected.PUT("/api/websites/:id/monitoring", websiteHandler.SaveMonitoring)
	protected.POST("/api/websites/:id/install-plugin", websiteHandler.InstallPlugin)
	protected.GET("/api/websites/:id/install-plugin/status", websiteHandler.InstallPluginStatus)
	protected.POST("/api/websites/:id/reinstall-wp", websiteHandler.ReinstallWordPress)
	aiDevelopmentHandler := &handlers.AIDevelopmentAccessHandler{}
	protected.GET("/api/websites/:id/ai-development-access", aiDevelopmentHandler.Status)
	protected.POST("/api/websites/:id/ai-development-access", aiDevelopmentHandler.Enable)
	protected.POST("/api/websites/:id/ai-development-access/rotate", aiDevelopmentHandler.Rotate)
	protected.DELETE("/api/websites/:id/ai-development-access", aiDevelopmentHandler.Disable)
	protected.GET("/api/websites/:id/nginx-custom", websiteHandler.GetNginxCustom)
	protected.PUT("/api/websites/:id/nginx-custom", websiteHandler.SaveNginxCustom)
	protected.PUT("/api/websites/:id/access-log", websiteHandler.SetAccessLogMode)
	protected.PUT("/api/websites/:id/document-root", websiteHandler.SetDocumentRoot)
	protected.PUT("/api/websites/:id/cdn-realip", websiteHandler.SetCDNRealIP)
	protected.PUT("/api/websites/:id/log-retention", websiteHandler.SetLogRetention)
	protected.PUT("/api/websites/:id/expiry", websiteHandler.UpdateExpiry)
	backupHandler := &handlers.BackupHandler{}
	protected.GET("/api/websites/:id/backups", backupHandler.List)
	protected.POST("/api/websites/:id/backups", backupHandler.Create)
	protected.DELETE("/api/websites/:id/backups/:bid", backupHandler.Delete)
	protected.GET("/api/websites/:id/backups/:bid/download", backupHandler.Download)
	protected.POST("/api/websites/:id/backups/:bid/restore", backupHandler.Restore)
	protected.DELETE("/api/websites/:id/file-backups/:bid", backupHandler.DeleteFileBackup)
	protected.GET("/api/websites/:id/file-backups/:bid/download", backupHandler.DownloadFileBackup)
	protected.POST("/api/websites/:id/backups/upload-restore", backupHandler.UploadRestore)
	protected.GET("/api/websites/:id/backups/restore-tasks/:task_id", backupHandler.RestoreStatus)
	protected.GET("/api/websites/:id/backups/settings", backupHandler.GetSettings)
	protected.PUT("/api/websites/:id/backups/settings", backupHandler.UpdateSettings)
	protected.POST("/api/websites/:id/backups/clear-database", backupHandler.ClearDatabase)
	databaseManagerHandler := &handlers.DatabaseManagerHandler{}
	protected.GET("/api/databases", databaseManagerHandler.List)
	protected.GET("/api/websites/:id/adminer/status", adminerHandler.Status)
	protected.POST("/api/websites/:id/adminer/enable", adminerHandler.Enable)
	protected.POST("/api/websites/:id/adminer/disable", adminerHandler.Disable)
	protected.GET("/api/backups/overview", handlers.GetBackupOverview)
	protected.GET("/api/backups/policy", handlers.GetBackupPolicy)
	protected.PUT("/api/backups/policy", handlers.SaveBackupPolicy)
	protected.POST("/api/backups/reconcile-status", handlers.ReconcileBackupStatus)

	dashboardHandler := &handlers.DashboardHandler{}
	protected.GET("/api/dashboard/stats", dashboardHandler.GetStats)
	protected.GET("/api/dashboard/metrics", dashboardHandler.GetMetrics)
	protected.GET("/api/dashboard/site-resources", dashboardHandler.GetSiteResources)
	protected.GET("/api/announcement", handlers.GetAnnouncement)

	firewallHandler := &handlers.FirewallHandler{}
	protected.GET("/api/firewall/ports", firewallHandler.PortStatus)
	protected.POST("/api/firewall/ports", firewallHandler.AddPortRule)
	protected.POST("/api/firewall/ports/protection", firewallHandler.EnablePortProtection)
	protected.POST("/api/firewall/ports/access/preview", firewallHandler.PreviewAccess)
	protected.POST("/api/firewall/ports/access/apply", firewallHandler.ApplyAccess)
	protected.POST("/api/firewall/ports/access/confirm", firewallHandler.ConfirmAccess)
	protected.GET("/api/firewall/ssh-port", firewallHandler.SSHPortStatus)
	protected.POST("/api/firewall/ssh-port", firewallHandler.ChangeSSHPort)
	protected.POST("/api/firewall/ssh-port/confirm", firewallHandler.ConfirmSSHPort)
	protected.POST("/api/firewall/ports/protection/confirm", firewallHandler.ConfirmPortProtection)
	protected.DELETE("/api/firewall/ports/:id", firewallHandler.DeletePortRule)
	protected.GET("/api/firewall/bans", firewallHandler.ListBans)
	protected.GET("/api/firewall/wp-security-report", firewallHandler.WPSecurityReport)
	protected.GET("/api/firewall/file-security-events", firewallHandler.ListFileSecurityEvents)
	protected.POST("/api/firewall/file-security-events/refresh", firewallHandler.RefreshFileSecurityEvents)
	protected.POST("/api/firewall/bans", firewallHandler.ManualBan)
	protected.DELETE("/api/firewall/bans/:id", firewallHandler.Unban)
	protected.POST("/api/firewall/bans/:id/permanent", firewallHandler.PermanentBan)

	securityHandler := &handlers.SecurityHandler{}
	protected.GET("/api/security/status", securityHandler.GetStatus)
	protected.GET("/api/security/settings", securityHandler.GetSettings)
	protected.PUT("/api/security/settings", securityHandler.UpdateSettings)
	protected.POST("/api/security/whitelist/refresh", securityHandler.RefreshWhitelist)
	protected.PUT("/api/security/whitelist/googlebot", securityHandler.ImportGooglebotRanges)
	protected.GET("/api/security/cdn-realip-groups", securityHandler.ListCDNRealIPGroups)
	protected.POST("/api/security/cdn-realip-groups", securityHandler.CreateCDNRealIPGroup)
	protected.PUT("/api/security/cdn-realip-groups/:id", securityHandler.UpdateCDNRealIPGroup)
	protected.DELETE("/api/security/cdn-realip-groups/:id", securityHandler.DeleteCDNRealIPGroup)

	alertHandler := &handlers.AlertHandler{}
	protected.GET("/api/alert/settings", alertHandler.GetSettings)
	protected.PUT("/api/alert/settings", alertHandler.SaveSettings)
	protected.POST("/api/alert/smtp-config/export", alertHandler.ExportSMTPConfig)
	protected.POST("/api/alert/smtp-config/import", alertHandler.ImportSMTPConfig)
	protected.POST("/api/alert/test-smtp", alertHandler.TestSMTP)
	protected.POST("/api/alert/test-webhook", alertHandler.TestWebhook)
	protected.GET("/api/alert/log", alertHandler.GetLog)

	cronHandler := &handlers.CronHandler{}
	protected.GET("/api/cron", cronHandler.List)
	protected.POST("/api/cron", cronHandler.Create)
	protected.PUT("/api/cron/:id", cronHandler.Update)
	protected.DELETE("/api/cron/:id", cronHandler.Delete)
	protected.POST("/api/cron/:id/run", cronHandler.Run)
	protected.GET("/api/cron/system", cronHandler.SystemList)
	protected.GET("/api/cron/logs", cronHandler.ViewLogs)

	fileHandler := &handlers.FileHandler{}
	protected.GET("/api/files/list", fileHandler.List)
	protected.GET("/api/files/search", fileHandler.Search)
	protected.GET("/api/files/size", fileHandler.DirectorySize)
	protected.POST("/api/files/upload", fileHandler.Upload)
	protected.POST("/api/files/upload/init", fileHandler.UploadInit)
	protected.POST("/api/files/upload/chunk", fileHandler.UploadChunk)
	protected.POST("/api/files/upload/complete", fileHandler.UploadComplete)
	protected.POST("/api/files/remote-import", fileHandler.RemoteImport)
	protected.GET("/api/files/remote-import/:id", fileHandler.RemoteImportStatus)
	protected.GET("/api/files/download", fileHandler.Download)
	protected.DELETE("/api/files/delete", fileHandler.Delete)
	protected.PUT("/api/files/rename", fileHandler.Rename)
	protected.GET("/api/files/permissions", fileHandler.Permissions)
	protected.POST("/api/files/batch-zip", fileHandler.BatchCompress)
	protected.POST("/api/files/move", fileHandler.Move)
	protected.POST("/api/files/copy", fileHandler.Copy)
	protected.POST("/api/files/zip", fileHandler.Compress)
	protected.POST("/api/files/unzip", fileHandler.Decompress)
	protected.POST("/api/files/mkdir", fileHandler.CreateDir)
	protected.POST("/api/files/fix-permissions", fileHandler.FixPermissions)

	wpPackageService, err := executor.SharedWPPackageService(cfg)
	if err != nil {
		log.Printf("WordPress package service disabled: code=%s", executor.ArchiveErrorCode(err))
	}
	settingsHandler := &handlers.SettingsHandler{WPPackageService: wpPackageService, ConfigPath: configPath}
	aiHandler := &handlers.AIHandler{}
	logAnalysisHandler := &handlers.LogAnalysisHandler{}
	protected.GET("/api/settings/panel-certificate", settingsHandler.PanelCertificateStatus)
	protected.POST("/api/settings/panel-domain/check", settingsHandler.CheckPanelDomain)
	protected.POST("/api/settings/panel-certificate", settingsHandler.ApplyPanelCertificate)
	protected.GET("/api/settings", settingsHandler.GetSettings)
	protected.PUT("/api/settings", settingsHandler.UpdateSettings)
	protected.GET("/api/settings/logs", settingsHandler.GetOperationLogs)
	protected.GET("/api/settings/wp-package", settingsHandler.GetWPPackage)
	protected.POST("/api/settings/wp-package/upload", settingsHandler.UploadWPPackage)
	protected.POST("/api/settings/wp-package/download", settingsHandler.DownloadWPPackage)
	protected.DELETE("/api/settings/wp-package", settingsHandler.DeleteWPPackage)
	protected.GET("/api/settings/remote-backup", handlers.GetRemoteBackup)
	protected.PUT("/api/settings/remote-backup", handlers.SaveRemoteBackup)
	protected.POST("/api/settings/remote-backup/test", handlers.TestRemoteBackup)
	protected.GET("/api/settings/db-backup", settingsHandler.GetDBBackups)
	protected.POST("/api/settings/db-backup", settingsHandler.CreateDBBackup)
	protected.POST("/api/settings/db-backup/restore", settingsHandler.RestoreDBBackup)
	protected.GET("/api/settings/db-backup/restore-status", settingsHandler.GetDBRestoreStatus)
	protected.DELETE("/api/settings/db-backup", settingsHandler.DeleteDBBackup)
	protected.GET("/api/settings/db-backup/:filename/download", settingsHandler.DownloadDBBackup)
	protected.GET("/api/proxy/test", settingsHandler.TestProxy)
	protected.GET("/api/ai/settings", aiHandler.GetSettings)
	protected.PUT("/api/ai/settings", aiHandler.SaveSettings)
	protected.POST("/api/ai/test", aiHandler.Test)
	protected.POST("/api/websites/:id/ai/diagnose", aiHandler.Diagnose)
	protected.GET("/api/websites/:id/ai/sessions", aiHandler.ListSessions)
	protected.GET("/api/websites/:id/ai/sessions/:session_id", aiHandler.GetSession)
	protected.GET("/api/websites/:id/ai/sessions/:session_id/messages", aiHandler.ListMessages)
	protected.POST("/api/websites/:id/ai/sessions/:session_id/messages", aiHandler.SendMessage)
	protected.POST("/api/log-analysis", logAnalysisHandler.Start)
	protected.GET("/api/log-analysis", logAnalysisHandler.List)
	protected.GET("/api/log-analysis/:id", logAnalysisHandler.Get)
	protected.GET("/api/log-analysis/:id/details", logAnalysisHandler.Details)
	protected.POST("/api/log-analysis/:id/diagnostic-session", logAnalysisHandler.CreateDiagnosticSession)

	protected.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "dashboard.html", pageData(suffix, "dashboard", "dashboard_content", c))
	})
	protected.GET("/websites", func(c *gin.Context) {
		c.HTML(http.StatusOK, "websites.html", pageData(suffix, "websites", "websites_content", c))
	})
	protected.GET("/wordpress-overview", func(c *gin.Context) {
		c.HTML(http.StatusOK, "wordpress_overview.html", pageData(suffix, "wordpress_overview", "wordpress_overview_content", c))
	})
	protected.GET("/websites/new", func(c *gin.Context) {
		c.HTML(http.StatusOK, "website_new.html", pageData(suffix, "websites", "websites_new_content", c))
	})
	protected.GET("/websites/migration", func(c *gin.Context) {
		c.HTML(http.StatusOK, "site_migration.html", pageData(suffix, "websites", "site_migration_content", c))
	})
	protected.GET("/websites/:id", func(c *gin.Context) {
		c.HTML(http.StatusOK, "website_detail.html", pageData(suffix, "websites", "websites_detail_content", c))
	})
	protected.GET("/websites/:id/wordpress", func(c *gin.Context) {
		c.HTML(http.StatusOK, "wordpress_site_detail.html", pageData(suffix, "wordpress_overview", "wordpress_site_detail_content", c))
	})
	protected.GET("/databases", func(c *gin.Context) {
		c.HTML(http.StatusOK, "databases.html", pageData(suffix, "databases", "databases_content", c))
	})
	protected.GET("/databases/:id", func(c *gin.Context) {
		c.HTML(http.StatusOK, "database_detail.html", pageData(suffix, "databases", "database_detail_content", c))
	})
	protected.GET("/ai-diagnostics", func(c *gin.Context) {
		c.HTML(http.StatusOK, "ai_diagnostics.html", pageData(suffix, "ai-diagnostics", "ai_diagnostics_content", c))
	})
	protected.GET("/log-analysis", func(c *gin.Context) {
		c.HTML(http.StatusOK, "log_analysis.html", pageData(suffix, "log-analysis", "log_analysis_content", c))
	})
	protected.GET("/cron", func(c *gin.Context) {
		c.HTML(http.StatusOK, "cron.html", pageData(suffix, "cron", "cron_content", c))
	})
	protected.GET("/backups", func(c *gin.Context) {
		c.HTML(http.StatusOK, "backups.html", pageData(suffix, "backups", "backups_content", c))
	})
	protected.GET("/backups/remote-settings", func(c *gin.Context) {
		data := pageData(suffix, "backups", "remote_backup_settings_content", c)
		data["Title"] = i18n.T(i18n.LangFromRequest(c.Request), "backups.remote_settings")
		c.HTML(http.StatusOK, "remote_backup_settings.html", data)
	})
	protected.GET("/firewall", func(c *gin.Context) {
		c.HTML(http.StatusOK, "firewall.html", pageData(suffix, "firewall", "firewall_content", c))
	})
	protected.GET("/files", func(c *gin.Context) {
		c.HTML(http.StatusOK, "files.html", pageData(suffix, "files", "files_content", c))
	})
	protected.GET("/security", func(c *gin.Context) {
		c.HTML(http.StatusOK, "security.html", pageData(suffix, "security", "security_content", c))
	})
	protected.GET("/alert", func(c *gin.Context) {
		c.HTML(http.StatusOK, "alert.html", pageData(suffix, "alert", "alert_content", c))
	})
	protected.GET("/settings", func(c *gin.Context) {
		c.HTML(http.StatusOK, "settings.html", pageData(suffix, "settings", "settings_content", c))
	})
	protected.GET("/help", func(c *gin.Context) {
		c.HTML(http.StatusOK, "help.html", pageData(suffix, "help", "help_content", c))
	})

	vpsHandler := &handlers.VPSHandler{}
	protected.GET("/api/vps/overview", vpsHandler.Overview)
	protected.POST("/api/vps/tools", vpsHandler.SimpleTool)
	protected.GET("/api/vps/swap", vpsHandler.SwapStatus)
	protected.POST("/api/vps/swap/recommended", vpsHandler.ApplyRecommendedSwap)
	protected.PUT("/api/vps/swap", vpsHandler.ApplyCustomSwap)
	protected.PUT("/api/vps/swap/swappiness", vpsHandler.SetSwappiness)
	protected.DELETE("/api/vps/swap", vpsHandler.RemoveManagedSwap)
	protected.GET("/api/vps/dns", vpsHandler.DNSStatus)
	protected.POST("/api/vps/dns/test", vpsHandler.TestDNSPreset)
	protected.PUT("/api/vps/dns", vpsHandler.ApplyDNSPreset)
	protected.DELETE("/api/vps/dns", vpsHandler.RestoreAutomaticDNS)
	protected.POST("/api/vps/nftables/enable-boot", vpsHandler.EnableNftablesBoot)

	softwareHandler := &handlers.SoftwareHandler{}
	protected.GET("/vps", func(c *gin.Context) { c.HTML(http.StatusOK, "vps.html", pageData(suffix, "vps", "vps_content", c)) })
	protected.GET("/software", func(c *gin.Context) {
		c.HTML(http.StatusOK, "software.html", pageData(suffix, "software", "software_content", c))
	})
	protected.GET("/api/software", softwareHandler.List)
	protected.GET("/api/software/development-tools", softwareHandler.DevelopmentTools)
	protected.POST("/api/software/development-tools/install", softwareHandler.InstallDevelopmentTool)
	protected.GET("/api/software/recommend", softwareHandler.Recommend)
	protected.POST("/api/software/opcache/clear", softwareHandler.ClearOpcache)
	protected.GET("/api/software/guard", softwareHandler.GetGuardStatus)
	protected.POST("/api/software/guard/action", softwareHandler.GuardAction)
	protected.PUT("/api/software/php-config", softwareHandler.SavePHPBatchConfig)
	protected.PUT("/api/software/config", softwareHandler.SaveConfig)
	protected.GET("/api/software/log", softwareHandler.ViewLog)
	protected.DELETE("/api/software/log", softwareHandler.ClearLog)
	updateHandler := &handlers.UpdateHandler{CurrentVersion: version, ConfigPath: configPath, Config: cfg}
	protected.GET("/api/update/check", updateHandler.Check)
	protected.GET("/api/update/status", updateHandler.Status)
	protected.POST("/api/update/do", updateHandler.Update)
	sysUpdateHandler := &handlers.SystemUpdateHandler{Config: cfg}
	protected.GET("/api/system/updates", sysUpdateHandler.Check)
	protected.GET("/api/system/updates/status", sysUpdateHandler.Status)
	protected.POST("/api/system/updates/do", sysUpdateHandler.Update)

	tmpl := template.Must(template.New("").Funcs(i18n.FuncMap()).ParseFS(tmplFS, "templates/*.html"))
	r.SetHTMLTemplate(tmpl)

	return r
}

func newWPCoreUpdateHandler(db *sql.DB, backupDir string) *handlers.WPCoreUpdateHandler {
	handler := &handlers.WPCoreUpdateHandler{}
	service, err := executor.NewWPCoreUpdateService(db, backupDir)
	if err != nil {
		log.Println("WordPress 核心更新 API 未启用")
		return handler
	}
	handler.Service = service
	return handler
}

func newWPPluginUpdateHandler(db *sql.DB, backupDir string) *handlers.WPPluginUpdateHandler {
	handler := &handlers.WPPluginUpdateHandler{}
	service, err := executor.NewWPPluginUpdateService(db, backupDir)
	if err != nil {
		log.Println("WordPress 插件更新 API 未启用")
		return handler
	}
	handler.Service = service
	return handler
}

func newWPPluginBatchHandler(db *sql.DB, backupDir, wwwRoot string) *handlers.WPPluginBatchHandler {
	handler := &handlers.WPPluginBatchHandler{}
	service, err := executor.NewWPPluginBatchService(db, backupDir, wwwRoot)
	if err != nil {
		log.Println("WordPress 插件批量更新 API 未启用")
		return handler
	}
	handler.Service = service
	return handler
}

func newWPThemeUpdateHandler(db *sql.DB, backupDir string) *handlers.WPThemeUpdateHandler {
	handler := &handlers.WPThemeUpdateHandler{}
	service, err := executor.NewWPThemeUpdateService(db, backupDir)
	if err != nil {
		log.Println("WordPress 主题更新 API 未启用")
		return handler
	}
	handler.Service = service
	return handler
}

var pageTitleKeys = map[string]string{
	"vps":                "nav.vps",
	"dashboard":          "nav.dashboard",
	"websites":           "nav.websites",
	"wordpress_overview": "nav.wordpress_overview",
	"databases":          "nav.databases",
	"ai-diagnostics":     "nav.ai_diagnostics",
	"log-analysis":       "nav.log_analysis",
	"cron":               "nav.cron",
	"backups":            "nav.backups",
	"firewall":           "nav.firewall",
	"security":           "nav.security",
	"files":              "nav.files",
	"software":           "nav.software",
	"alert":              "nav.alert",
	"settings":           "nav.settings",
	"help":               "nav.help",
}

func pageData(suffix string, active string, contentTpl string, c *gin.Context) gin.H {
	i18n.MaybeSetLanguageCookie(c.Writer, c.Request)
	lang := i18n.LangFromRequest(c.Request)
	csrfToken := middleware.GetCSRFToken(c)
	title := i18n.T(lang, pageTitleKeys[active])
	return gin.H{
		"Title":           title,
		"PanelTitle":      handlers.GetPanelTitle(),
		"PanelVersion":    panelVersion,
		"AssetVersion":    panelVersion,
		"ContentTemplate": contentTpl,
		"RandomSuffix":    suffix,
		"Active":          active,
		"AssetPrefix":     "/" + suffix + "/assets",
		"CSRFToken":       csrfToken,
		"Lang":            lang,
		"MessagesJSON":    i18n.MessagesJSON(lang, i18nKeys),
	}
}
