package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORS deixa uma página de outra origem, como o front de outro grupo, chamar a API pelo navegador.
// "*" libera qualquer origem; uma lista libera só as origens citadas. sem origens, não faz nada.
//
// roda antes do roteamento, então também responde o preflight (OPTIONS) que o router, sozinho, recusaria com 405.
func CORS(origens []string) gin.HandlerFunc {
	liberaTudo := false
	permitidas := map[string]bool{}
	for _, origem := range origens {
		origem = strings.TrimSpace(origem)
		switch {
		case origem == "*":
			liberaTudo = true
		case origem != "":
			permitidas[origem] = true
		}
	}

	return func(c *gin.Context) {
		origem := c.GetHeader("Origin")
		if origem == "" || (!liberaTudo && !permitidas[origem]) {
			c.Next()
			return
		}

		if liberaTudo {
			c.Header("Access-Control-Allow-Origin", "*")
		} else {
			c.Header("Access-Control-Allow-Origin", origem)
			c.Header("Vary", "Origin")
		}
		// o Location do 201 só chega ao JavaScript se for exposto.
		c.Header("Access-Control-Expose-Headers", "Location")

		if c.Request.Method == http.MethodOptions && c.GetHeader("Access-Control-Request-Method") != "" {
			c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			cabecalhos := c.GetHeader("Access-Control-Request-Headers")
			if cabecalhos == "" {
				cabecalhos = "Content-Type, Accept"
			}
			c.Header("Access-Control-Allow-Headers", cabecalhos)
			c.Header("Access-Control-Max-Age", "600")
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
