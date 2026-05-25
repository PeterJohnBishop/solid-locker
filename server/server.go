package server

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/peterjohnbishop/solid-locker/vault"
)

func ServeGin(db *vault.Storage, masterKey []byte) {
	r := gin.Default()

	r.MaxMultipartMemory = 8 << 20 // 8 MiB

	addFileRoutes(r, db, masterKey)

	fmt.Println("Server listening on :8080...")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
