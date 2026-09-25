package config

import (
	"errors"
	"os"

	"github.com/joho/godotenv"
)

// porta combinada com os outros grupos: score 8080, decisão 8081, operações 8083.
const portaPadrao = "8083"

type Config struct {
	Porta       string
	DatabaseURL string
}

func Carregar() (Config, error) {
	// se não tiver .env segue o baile, as variáveis podem vir do próprio ambiente.
	godotenv.Load()

	config := Config{
		Porta:       os.Getenv("PORT"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
	}

	if config.Porta == "" {
		config.Porta = portaPadrao
	}

	if config.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL não configurada")
	}

	return config, nil
}
