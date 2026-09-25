package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// GET /health - usado pelos outros grupos pra saber se o serviço está no ar.
func Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
