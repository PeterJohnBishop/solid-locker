package server

import (
	"github.com/gin-gonic/gin"
	"github.com/peterjohnbishop/solid-locker/vault"
)

func addFileRoutes(r *gin.Engine, db *vault.Storage, masterKey []byte) {
	r.POST("/upload", func(c *gin.Context) {
		handleStreamingUpload(c, db, masterKey)
	})

	r.GET("/download/:id", func(c *gin.Context) {
		handleStreamingDownload(c, db, masterKey)
	})
}
