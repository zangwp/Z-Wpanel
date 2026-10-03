package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/zangwp/Z-Wpanel/executor"
	"github.com/zangwp/Z-Wpanel/models"
	"net/http"
	"strconv"
	"strings"
)

func (h *FirewallHandler) SSHPortStatus(c *gin.Context) {
	c.JSON(200, models.SuccessResponse(executor.GetSSHPortStatus()))
}
func (h *FirewallHandler) ChangeSSHPort(c *gin.Context) {
	var req struct {
		Port    int  `json:"port"`
		Confirm bool `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || !req.Confirm {
		c.JSON(400, models.ErrorResponse("请确认新端口已在云安全组放行，并保留当前 SSH 会话"))
		return
	}
	result, err := executor.BeginSSHPortChange(req.Port, c.ClientIP())
	if err != nil {
		c.JSON(400, models.ErrorResponse(err.Error()))
		return
	}
	c.JSON(200, models.SuccessResponse(result))
}
func (h *FirewallHandler) ConfirmSSHPort(c *gin.Context) {
	var req struct {
		Token string `json:"token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, models.ErrorResponse("确认格式无效"))
		return
	}
	if err := executor.ConfirmSSHPortChange(req.Token); err != nil {
		c.JSON(400, models.ErrorResponse(err.Error()))
		return
	}
	c.JSON(200, models.SuccessResponse(gin.H{"confirmed": true}))
}

func (h *FirewallHandler) PreviewAccess(c *gin.Context) {
	var req executor.FirewallAccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, models.ErrorResponse("规则格式无效"))
		return
	}
	result, err := executor.PreviewFirewallAccess(req, c.ClientIP())
	if err != nil {
		c.JSON(400, models.ErrorResponse(err.Error()))
		return
	}
	c.JSON(200, models.SuccessResponse(result))
}

func (h *FirewallHandler) ApplyAccess(c *gin.Context) {
	var req executor.FirewallAccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, models.ErrorResponse("规则格式无效"))
		return
	}
	result, err := executor.ApplyFirewallAccess(req, c.ClientIP())
	if err != nil {
		c.JSON(400, models.ErrorResponse(err.Error()))
		return
	}
	c.JSON(200, models.SuccessResponse(result))
}

func (h *FirewallHandler) ConfirmAccess(c *gin.Context) {
	var req struct {
		Token string `json:"token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, models.ErrorResponse("确认格式无效"))
		return
	}
	if err := executor.ConfirmFirewallAccess(req.Token); err != nil {
		c.JSON(400, models.ErrorResponse(err.Error()))
		return
	}
	c.JSON(200, models.SuccessResponse(gin.H{"confirmed": true}))
}

func (h *FirewallHandler) PortStatus(c *gin.Context) {
	status, err := executor.GetFirewallPortStatus()
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse("读取端口与防火墙状态失败"))
		return
	}
	status.CurrentManagementIP = c.ClientIP()
	c.JSON(http.StatusOK, models.SuccessResponse(status))
}

func (h *FirewallHandler) AddPortRule(c *gin.Context) {
	var req executor.FirewallPortRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse("端口规则格式不正确"))
		return
	}
	if strings.EqualFold(strings.TrimSpace(req.SourceMode), "current") {
		req.Source = c.ClientIP()
	}
	rule, err := executor.AddFirewallPortRule(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse(rule))
}

func (h *FirewallHandler) EnablePortProtection(c *gin.Context) {
	var req struct {
		Confirm bool `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || !req.Confirm {
		c.JSON(http.StatusBadRequest, models.ErrorResponse("必须确认防锁死说明后才能启用保护模式"))
		return
	}
	result, err := executor.EnableFirewallProtection(c.ClientIP())
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse(result))
}

func (h *FirewallHandler) ConfirmPortProtection(c *gin.Context) {
	var req struct {
		Token string `json:"token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse("确认参数不正确"))
		return
	}
	if err := executor.ConfirmFirewallProtection(strings.TrimSpace(req.Token)); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse(gin.H{"confirmed": true}))
}

func (h *FirewallHandler) DeletePortRule(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse("无效的规则 ID"))
		return
	}
	if err := executor.DeleteFirewallPortRule(id); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse(gin.H{"deleted": true}))
}
