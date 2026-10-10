package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// o padrão do gin responde em text/plain. aqui sai no mesmo formato json de todos os outros erros.
func RotaNaoEncontrada(c *gin.Context) {
	c.JSON(http.StatusNotFound, gin.H{"erro": "rota não encontrada"})
}

// o gin já preenche o header Allow com os métodos aceitos antes de chamar este handler.
func MetodoNaoPermitido(c *gin.Context) {
	c.JSON(http.StatusMethodNotAllowed, gin.H{"erro": "método não permitido nesta rota"})
}
