package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/unicsul-finops/financial-operations-service/api"
)

// GET /openapi.yaml - spec usado pelo swagger ui e por quem quiser gerar client.
func OpenAPI(c *gin.Context) {
	c.Data(http.StatusOK, "application/yaml; charset=utf-8", api.OpenAPI)
}

// GET /swagger/*any - swagger ui. os arquivos da interface vêm embutidos, funciona sem internet no laboratório.
func SwaggerUI() gin.HandlerFunc {
	ui := ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("/openapi.yaml"))

	return func(c *gin.Context) {
		// o gin-swagger só responde em /swagger/index.html, /swagger/ puro daria 404.
		if c.Param("any") == "/" {
			c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
			return
		}

		ui(c)
	}
}
