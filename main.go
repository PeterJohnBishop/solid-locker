package main

import (
	"log"

	"github.com/joho/godotenv"
	"github.com/peterjohnbishop/solid-locker/encryption"
	"github.com/peterjohnbishop/solid-locker/tui"
	"github.com/peterjohnbishop/solid-locker/vault"
)

func main() {
	// load environment variables
	if err := godotenv.Load(); err != nil {
		log.Print("No .env file found, relying on system environment variables")
	}

	// generate a random 32-byte master key []byte
	encryption.InitMasterKey()

	// initialize storage
	db, err := vault.NewStorage("./db/locker.db")
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	// serve gin
	// go func() {
	// 	server.ServeGin(db, encryption.SaltMaster)
	// }()

	// ssh
	tui.StartSSHServer(db)
}
