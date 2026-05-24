package server

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/peterjohnbishop/solid-locker/vault"
)

func handleStreamingUpload(c *gin.Context, db *vault.Storage, masterKey []byte) {
	reader, err := c.Request.MultipartReader()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Payload must be multipart/form-data"})
		return
	}

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate file uuid"})
			return
		}

		if part.FormName() == "file" {
			fileID, err := vault.GenerateUUIDv4()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create file record in DB"})
			}
			chunkSize := 2 * 1024 * 1024

			if err = db.CreateFileRecord(c.Request.Context(), fileID, part.FileName()); err != nil {
				part.Close()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create file record in DB"})
				return
			}

			err = vault.StreamEncryptAndStore(c.Request.Context(), part, chunkSize, masterKey, fileID, db)
			if err != nil {
				part.Close()
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
		part.Close()
	}

	c.JSON(http.StatusOK, gin.H{"status": "File streamed successfully"})
}

// handleStreamingDownload streams a requested file back to the client
func handleStreamingDownload(c *gin.Context, db *vault.Storage, masterKey []byte) {
	fileID := c.Param("id")

	filename, err := db.GetFilename(c.Request.Context(), fileID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File record not found"})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Header("Content-Type", "application/octet-stream")

	err = db.StreamRetrieveAndDecrypt(c.Request.Context(), fileID, c.Writer, masterKey)
	if err != nil {
		log.Printf("Streaming download failed for file %s: %v", fileID, err)
		return
	}
}
