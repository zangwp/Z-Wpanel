package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/zangwp/Z-Wpanel/internal/executor"
	"github.com/zangwp/Z-Wpanel/internal/models"
	"net/http"
)

func (h *SecurityHandler) GetStatus(c *gin.Context) {
	status, err := executor.GetSecurityRuntimeStatus()
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse("读取安全运行状态失败"))
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse(status))
}
