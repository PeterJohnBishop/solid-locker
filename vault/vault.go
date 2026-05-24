package vault

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
	"github.com/peterjohnbishop/solid-locker/encryption"
)

func GenerateUUIDv4() (string, error) {
	b := make([]byte, 16)

	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	b[6] = (b[6] & 0x0f) | 0x40 // Set version to 4
	b[8] = (b[8] & 0x3f) | 0x80 // Set variant to RFC 4122

	// Format as a standard 36-character UUID string
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

type Storage struct {
	db *sql.DB
}

// interface for database storage
type ChunkSaver interface {
	StoreSingleChunk(ctx context.Context, fileID string, index int, payload []byte) error
}

// initializes the database and creates tables if they don't exist
func NewStorage(dbPath string) (*Storage, error) {
	// DSN parameters:
	// _fk=1 enforces foreign keys.
	// _journal_mode=WAL improves concurrent read/write performance.
	dsn := fmt.Sprintf("file:%s?_fk=1&_journal_mode=WAL", dbPath)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS files (id TEXT PRIMARY KEY, filename TEXT NOT NULL);
	CREATE TABLE IF NOT EXISTS file_chunks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		file_id TEXT NOT NULL,
		chunk_index INTEGER NOT NULL,
		encrypted_payload BLOB NOT NULL,
		FOREIGN KEY(file_id) REFERENCES files(id) ON DELETE CASCADE
	);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_file_chunk ON file_chunks(file_id, chunk_index);
	`
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	return &Storage{db: db}, nil
}

// CreateFileRecord initializes the parent row in the database so foreign keys don't fail.
func (s *Storage) CreateFileRecord(ctx context.Context, fileID string, filename string) error {
	query := "INSERT INTO files (id, filename) VALUES (?, ?)"
	_, err := s.db.ExecContext(ctx, query, fileID, filename)
	return err
}

// StoreSingleChunk satisfies the ChunkSaver interface.
func (s *Storage) StoreSingleChunk(ctx context.Context, fileID string, index int, payload []byte) error {
	query := "INSERT INTO file_chunks (file_id, chunk_index, encrypted_payload) VALUES (?, ?, ?)"
	_, err := s.db.ExecContext(ctx, query, fileID, index, payload)
	return err
}

// StreamEncryptAndStore reads from any stream, encrypts on the fly, and pushes straight to the DB.
func StreamEncryptAndStore(ctx context.Context, reader io.Reader, chunkSize int, masterKey []byte, fileID string, db ChunkSaver) error {
	buffer := make([]byte, chunkSize)
	chunkIndex := 0

	for {
		bytesRead, err := reader.Read(buffer)

		if bytesRead > 0 {
			encryptedChunk, encErr := encryption.EncryptData(buffer[:bytesRead], masterKey)
			if encErr != nil {
				return fmt.Errorf("failed to encrypt chunk %d: %w", chunkIndex, encErr)
			}

			if dbErr := db.StoreSingleChunk(ctx, fileID, chunkIndex, encryptedChunk); dbErr != nil {
				return fmt.Errorf("failed to store chunk %d: %w", chunkIndex, dbErr)
			}
			chunkIndex++
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed reading from stream: %w", err)
		}
	}

	return nil
}

// handleStreamingDownload streams a requested file back to the client
func (s *Storage) StreamRetrieveAndDecrypt(ctx context.Context, fileID string, w io.Writer, masterKey []byte) error {
	query := "SELECT encrypted_payload FROM file_chunks WHERE file_id = ? ORDER BY chunk_index ASC"
	rows, err := s.db.QueryContext(ctx, query, fileID)
	if err != nil {
		return fmt.Errorf("failed to query chunks: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var encryptedChunk []byte

		if err := rows.Scan(&encryptedChunk); err != nil {
			return fmt.Errorf("failed to scan chunk: %w", err)
		}

		plaintextChunk, err := encryption.DecryptData(encryptedChunk, masterKey)
		if err != nil {
			return fmt.Errorf("failed to decrypt chunk: %w", err)
		}

		_, err = w.Write(plaintextChunk)
		if err != nil {
			return fmt.Errorf("failed to write to output stream: %w", err)
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating over chunks: %w", err)
	}

	return nil
}

func (s *Storage) GetFilename(ctx context.Context, fileID string) (string, error) {
	var filename string
	err := s.db.QueryRowContext(ctx, "SELECT filename FROM files WHERE id = ?", fileID).Scan(&filename)
	if err != nil {
		return "", err
	}
	return filename, nil
}

// UploadLocalFile reads a file from disk, encrypts it, and stores it in SQLite
func UploadLocalFile(ctx context.Context, filePath string, db *Storage, masterKey []byte) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open local file: %w", err)
	}
	defer file.Close()

	fileID, err := GenerateUUIDv4()
	if err != nil {
		return "", fmt.Errorf("failed to generate UUID: %w", err)
	}
	filename := filepath.Base(filePath)
	chunkSize := 2 * 1024 * 1024

	if err := db.CreateFileRecord(ctx, fileID, filename); err != nil {
		return "", fmt.Errorf("failed to create db record: %w", err)
	}

	err = StreamEncryptAndStore(ctx, file, chunkSize, masterKey, fileID, db)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt and store: %w", err)
	}

	return fileID, nil
}

// DownloadLocalFile extracts a file from SQLite, decrypts it, and writes it to a local path
func DownloadLocalFile(ctx context.Context, fileID string, outputDir string, db *Storage, masterKey []byte) error {
	filename, err := db.GetFilename(ctx, fileID)
	if err != nil {
		return fmt.Errorf("file not found in DB: %w", err)
	}

	destinationPath := filepath.Join(outputDir, filename)
	file, err := os.Create(destinationPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer file.Close()

	err = db.StreamRetrieveAndDecrypt(ctx, fileID, file, masterKey)
	if err != nil {
		return fmt.Errorf("failed to decrypt and retrieve: %w", err)
	}

	if err := file.Sync(); err != nil {
		return fmt.Errorf("failed to sync file to disk: %w", err)
	}

	return nil
}
