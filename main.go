package main

import (
	"fmt"
	"log"
	"os"

	"github.com/DillonStreator/todos/storage"
	"github.com/eleanorhealth/milo"
	"github.com/go-pg/pg/v10"
	"github.com/joho/godotenv"
)

var store *milo.Store

func main() {
	_, jwtSecretEnvSet := os.LookupEnv("JWT_SECRET")
	if !jwtSecretEnvSet {
		log.Fatal("JWT_SECRET env must be set")
	}

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	cloudSQLConnectionName := os.Getenv("CLOUD_SQL_CONNECTION_NAME")
	socketDir := os.Getenv("SOCKET_DIR")
	var addr string

	var network string
	if cloudSQLConnectionName != "" && socketDir != "" {
		addr = fmt.Sprintf("%s/%s", socketDir, cloudSQLConnectionName)
		network = "unix"
	} else {
		addr = getEnv("DB_HOST", "localhost:8200")
		network = "tcp"
	}

	db := pg.Connect(&pg.Options{
		Network:  network,
		Addr:     addr,
		User:     getEnv("DB_USER", "postgres"),
		Password: getEnv("DB_PASS", "password"),
		Database: getEnv("DB_NAME", "todos"),
	})
	defer db.Close()

	err = storage.CreateSchema(db)
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
