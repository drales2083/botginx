package types

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	DefaultChunkSizeMB = 45
	MaxChunkSizeMB     = 50
)

type BackupFile struct {
	Name   string
	Path   string
	Size   int64
	SHA256 string
}

type ChunkInfo struct {
	Part   int
	Name   string
	Path   string
	Size   int64
	SHA256 string
}

func ChunkFile(filePath string, chunkSizeMB int) ([]ChunkInfo, error) {
	if chunkSizeMB <= 0 || chunkSizeMB > MaxChunkSizeMB {
		chunkSizeMB = DefaultChunkSizeMB
	}
	chunkSize := int64(chunkSizeMB) * 1024 * 1024

	info, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	if info.Size() <= chunkSize {
		hash, _ := HashFile(filePath)
		return []ChunkInfo{{
			Part:   0,
			Name:   filepath.Base(filePath),
			Path:   filePath,
			Size:   info.Size(),
			SHA256: hash,
		}}, nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	dir := filepath.Dir(filePath)
	baseName := filepath.Base(filePath)

	var chunks []ChunkInfo
	buf := make([]byte, chunkSize)
	part := 1

	for {
		n, err := file.Read(buf)
		if n == 0 {
			break
		}

		chunkName := fmt.Sprintf("%s.part%03d", baseName, part)
		chunkPath := filepath.Join(dir, chunkName)

		if err := os.WriteFile(chunkPath, buf[:n], 0600); err != nil {
			return nil, err
		}

		hash, _ := HashFile(chunkPath)
		chunks = append(chunks, ChunkInfo{
			Part:   part,
			Name:   chunkName,
			Path:   chunkPath,
			Size:   int64(n),
			SHA256: hash,
		})

		part++

		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}

	return chunks, nil
}

func HashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
