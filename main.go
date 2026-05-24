package main

import (
	"log"

	"github.com/joho/godotenv"
	"github.com/peterjohnbishop/solid-locker/encryption"
)

func main() {
	// load environment variables
	if err := godotenv.Load(); err != nil {
		log.Print("No .env file found")
	}

	// generate a random 32-byte master key []byte
	encryption.InitMasterKey()

	log.Println("solid-locker")

}
