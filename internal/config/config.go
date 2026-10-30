package config

import (
	"errors"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// porta combinada com os outros grupos: score 8080, decisão 8081, operações 8083.
const portaPadrao = "8083"

// o laboratório sobe cada serviço no próprio PC e as telas se falam pela rede, então o CORS vem liberado.
const corsPadrao = "*"

type Config struct {
	Porta       string
	DatabaseURL string
	// origens aceitas pelo CORS; "*" libera qualquer uma.
	CorsOrigens []string
}

func Carregar() (Config, error) {
	// se não tiver .env segue o baile, as variáveis podem vir do próprio ambiente.
	godotenv.Load()

	config := Config{
		Porta:       os.Getenv("PORT"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
	}

	config.CorsOrigens = origensDoCORS(os.Getenv("CORS_ORIGINS"))

	if config.Porta == "" {
		config.Porta = portaPadrao
	}

	if config.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL não configurada")
	}

	return config, nil
}

// lista separada por vírgula. vazio vale o padrão; "off" desliga o CORS.
func origensDoCORS(valor string) []string {
	valor = strings.TrimSpace(valor)
	if valor == "" {
		valor = corsPadrao
	}
	if strings.EqualFold(valor, "off") {
		return nil
	}

	origens := []string{}
	for _, origem := range strings.Split(valor, ",") {
		if origem = strings.TrimSpace(origem); origem != "" {
			origens = append(origens, origem)
		}
	}

	return origens
}
