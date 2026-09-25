package main

import (
	"log"

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

	router := gin.Default()

	router.GET("/health", handler.Health)

	router.POST("/operacoes", operacaoHandler.Criar)
	router.GET("/operacoes", operacaoHandler.Listar)
	router.GET("/operacoes/:id", operacaoHandler.Buscar)
	router.GET("/operacoes/:id/parcelas", operacaoHandler.ListarParcelas)

	log.Printf("financial-operations-service ouvindo na porta %s", cfg.Porta)

	if err := router.Run(":" + cfg.Porta); err != nil {
		log.Fatal(err)
	}
}
