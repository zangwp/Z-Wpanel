package router

import (
	"database/sql"
	"embed"
	"github.com/zangwp/Z-Wpanel/internal/accountsecurity"
	"html/template"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/zangwp/Z-Wpanel/internal/config"
	"github.com/zangwp/Z-Wpanel/internal/database"
	"github.com/zangwp/Z-Wpanel/internal/executor"
	"github.com/zangwp/Z-Wpanel/internal/handlers"
	"github.com/zangwp/Z-Wpanel/internal/i18n"
	"github.com/zangwp/Z-Wpanel/internal/middleware"

	"github.com/gin-gonic/gin"
)

var panelVersion string
var panelMessages *i18n.TemplateMessages

func SetupRouter(cfg *config.Config, tmplFS embed.FS, staticFS embed.FS, version string, configPath string) *gin.Engine {
	r, _ := SetupRouterWithCleanup(cfg, tmplFS, staticFS, version, configPath)
	return r
}
func SetupRouterWithCleanup(cfg *config.Config, tmplFS embed.FS, staticFS embed.FS, version string, configPath string) (*gin.Engine, func()) {
	panelVersion = version
	catalog, catalogErr := i18n.NewTemplateMessages(tmplFS, i18n.AllMessageKeys())
	if catalogErr != nil {
		log.Panicf("cannot initialize translations: %v", catalogErr)
	}
	panelMessages = catalog
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
	dbPath := cfg.SQLite.Path
	if dbPath == "" {
		if err := db.QueryRow("SELECT file FROM pragma_database_list WHERE name = 'main'").Scan(&dbPath); err != nil || dbPath == "" {
			panic("account security requires a persistent database path")
		}
	}
	mfaService, err := accountsecurity.NewMFAService(db, filepath.Join(filepath.Dir(dbPath), "account-mfa.key"))
	if err != nil {
		log.Panicf("cannot initialize account security: %v", err)
	}
	auditService := accountsecurity.NewAuditService(db)
	auditService.SetNotifier(executor.SendAccountSecurityNotification)
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

	attemptTracker.Audit = auditService
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
	authHandler := &handlers.AuthHandler{DB: db, Prefix: suffix, Tracker: attemptTracker, MFA: mfaService, Audit: auditService}

	// 面板根路径重定向到登录页（解决用户访问面板地址不带 /login 的问题）
	panelGroup.GET("", func(c *gin.Context) {
		if c.Request.URL.Path == "/"+suffix {
			c.Redirect(http.StatusFound, "/"+suffix+"/login")
			return
		}
		c.Next()
	})

	panelGroup.GET("/login", func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
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
			"MessagesJSON": panelMessages.JSON(lang, "login"),
		})
	})

	panelGroup.POST("/api/auth/login", middleware.CSRF(), func(c *gin.Context) {
		authHandler.Login(c)
	})
	// Login needs a token before a panel session exists. Keep this read-only
	// endpoint behind the random path and browser authentication gateway.
	panelGroup.GET("/api/auth/csrf-token", func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		if !middleware.SetCSRFToken(c) {
			return
		}
		authHandler.CSRFToken(c)
	})

	cacheHelper := &handlers.CacheHelperHandler{}
	pluginImageOptimizer := &handlers.ImageOptimizerHandler{}

	pluginGroup := r.Group(prefix)
	pluginGroup.Use(middleware.RandomPath(suffix))
	pluginGroup.Use(handlers.PluginRequestGuard())
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
	protected.Use(handlers.WebsiteBackupMaintenanceGuard())
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

	authHandler = &handlers.AuthHandler{DB: db, Prefix: suffix, Tracker: attemptTracker, MFA: mfaService, Audit: auditService}
	protected.POST("/api/auth/logout", authHandler.Logout)
	protected.GET("/api/auth/check", authHandler.Check)
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
	protected.GET("/api/security/whitelist/tasks/:task_id", securityHandler.WhitelistRefreshStatus)
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
	settingsHandler := &handlers.SettingsHandler{Auth: authHandler, WPPackageService: wpPackageService, ConfigPath: configPath}
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

	wpPanelAccessHandler := &handlers.WPPanelAccessHandler{Auth: authHandler}
	protected.GET("/api/auth/mfa", authHandler.MFAStatus)
	protected.POST("/api/auth/mfa/setup", authHandler.MFASetup)
	protected.POST("/api/auth/mfa/confirm", authHandler.MFAConfirm)
	protected.POST("/api/auth/mfa/disable", authHandler.MFADisable)
	protected.POST("/api/auth/mfa/recovery-codes", authHandler.MFARecoveryCodes)
	protected.GET("/api/auth/sessions", authHandler.ListSessions)
	protected.DELETE("/api/auth/sessions/:id", authHandler.RevokeSession)
	protected.POST("/api/auth/sessions/revoke-others", authHandler.RevokeOtherSessions)
	protected.GET("/api/websites/:id/cloudflare-security", websiteHandler.GetCloudflareSecurity)
	protected.PUT("/api/websites/:id/cloudflare-security", websiteHandler.SaveCloudflareSecurity)
	protected.POST("/api/websites/:id/cloudflare-security/test", websiteHandler.TestCloudflareSecurity)
	protected.POST("/api/websites/:id/cloudflare-security/sync", websiteHandler.SyncCloudflareSecurity)
	protected.GET("/api/websites/:id/wordpress-access", wpPanelAccessHandler.Status)
	protected.PUT("/api/websites/:id/wordpress-access", wpPanelAccessHandler.Save)
	protected.POST("/api/websites/:id/wordpress-access/login", wpPanelAccessHandler.Login)
	protected.POST("/api/websites/:id/wordpress-access/revoke", wpPanelAccessHandler.Revoke)
	protected.POST("/api/websites/:id/site-backups", backupHandler.CreateSiteBackup)
	protected.POST("/api/websites/:id/site-backups/recover-maintenance", backupHandler.RecoverSiteBackupMaintenance)
	protected.POST("/api/websites/:id/site-backups/:bid/restore", backupHandler.RestoreSiteBackup)
	protected.DELETE("/api/websites/:id/site-backups/:bid", backupHandler.DeleteSiteBackup)
	protected.GET("/api/websites/:id/site-backups/:bid/download", backupHandler.DownloadSiteBackup)
	protected.POST("/api/websites/:id/file-backups/:bid/restore", backupHandler.RestoreFileBackup)
	protected.GET("/api/software/health", softwareHandler.Health)
	protected.POST("/api/software/backup-dependencies/install", softwareHandler.InstallBackupDependencies)
	protected.GET("/api/websites/:id/security-status", websiteHandler.SecurityStatus)

	protected.GET("/api/auth/audit", authHandler.AuditLog)
	protected.GET("/api/auth/security-notifications", authHandler.GetSecurityNotifications)
	protected.PUT("/api/auth/security-notifications", authHandler.SetSecurityNotifications)
	protected.POST("/api/websites/:id/security-verify", websiteHandler.VerifySecurityStatus)
	protected.POST("/api/websites/:id/file-backups", backupHandler.CreateFileBackup)
	protected.GET("/api/websites/:id/backups/tasks/:task_id", backupHandler.BackupOperationStatus)
	protected.GET("/api/websites/:id/backups/active-operation", backupHandler.ActiveBackupOperation)
	protected.GET("/api/websites/:id/file-backups/restore-tasks/:task_id", backupHandler.FileRestoreStatus)
	protected.GET("/api/websites/:id/backups/active-restore", backupHandler.ActiveRestore)
	return r, auditService.Close
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
		"MessagesJSON":    panelMessages.JSON(lang, contentTpl),
	}
}
