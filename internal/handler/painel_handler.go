package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/unicsul-finops/financial-operations-service/web"
)

// GET / - painel para acompanhar e operar as operações. usa a própria API, da mesma origem.
func Painel(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", web.Painel)
}
