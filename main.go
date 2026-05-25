package main

import (
	"log"

	"github.com/joho/godotenv"
	"github.com/peterjohnbishop/solid-locker/encryption"
	"github.com/peterjohnbishop/solid-locker/server"
	"github.com/peterjohnbishop/solid-locker/vault"
)

func main() {
	// load environment variables
	if err := godotenv.Load(); err != nil {
		log.Print("No .env file found, relying on system environment variables")
	}

	// generate a random 32-byte master key []byte
	encryption.InitMasterKey()

	db, err := vault.NewStorage("./db/locker.db")
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	// serve gin
	server.ServeGin(db, encryption.SaltMaster)
}
