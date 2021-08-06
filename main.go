package main

import (
	"log"
	"os"

	"github.com/DillonStreator/todos/storage"
	"github.com/eleanorhealth/milo"
	"github.com/go-pg/pg/v10"
)

var store *milo.Store

func main() {
	db := pg.Connect(&pg.Options{
		Addr:     getEnv("DB_ADDR", "localhost:8200"),
		User:     getEnv("DB_USER", "postgres"),
		Password: getEnv("DB_PASS", "password"),
		Database: getEnv("DB_NAME", "todos"),
	})
	defer db.Close()

	err := storage.CreateSchema(db)
	if err != nil {
		log.Fatal(err)
	}

	store = milo.NewStore(db, storage.MiloEntityModelMap)

	err = startServer()
	if err != nil {
		log.Fatal(err)
	}
}

func getEnv(key, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return defaultValue
}
