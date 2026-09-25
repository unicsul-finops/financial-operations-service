package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/unicsul-finops/financial-operations-service/internal/config"
	"github.com/unicsul-finops/financial-operations-service/internal/database"
	"github.com/unicsul-finops/financial-operations-service/internal/handler"
	"github.com/unicsul-finops/financial-operations-service/internal/repository"
	"github.com/unicsul-finops/financial-operations-service/internal/service"
)

func main() {
	// por padrão o decimal vira string no json. número mantém o mesmo formato do payload da decisão.
	decimal.MarshalJSONWithoutQuotes = true

	cfg, err := config.Carregar()
	if err != nil {
		log.Fatal(err)
	}

	db, err := database.Conectar(cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}

	defer db.Close()

	repo := repository.NovoOperacaoRepository(db)
	operacaoService := service.NovoOperacaoService(repo)
	operacaoHandler := handler.NovoOperacaoHandler(operacaoService)

	router := novoRouter(operacaoHandler)

	log.Printf("financial-operations-service ouvindo na porta %s", cfg.Porta)
	log.Printf("swagger em http://localhost:%s/swagger", cfg.Porta)

	if err := router.Run(":" + cfg.Porta); err != nil {
		log.Fatal(err)
	}
}

// separado do main pra o teste conseguir conferir se toda rota está documentada no openapi.yaml.
func novoRouter(operacaoHandler *handler.OperacaoHandler) *gin.Engine {
	router := gin.Default()

	router.GET("/health", handler.Health)

	router.POST("/operacoes", operacaoHandler.Criar)
	router.GET("/operacoes", operacaoHandler.Listar)
	router.GET("/operacoes/:id", operacaoHandler.Buscar)
	router.GET("/operacoes/:id/parcelas", operacaoHandler.ListarParcelas)

	router.GET("/openapi.yaml", handler.OpenAPI)
	router.GET("/swagger", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	})
	router.GET("/swagger/*any", handler.SwaggerUI())

	return router
}
