package database

import (
	"database/sql"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func Conectar(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}

	// o sql.Open não conecta de verdade, o ping é o que garante que o banco está no ar antes de subir a api.
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}
