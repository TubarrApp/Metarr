package utils

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

// calculateFileHash computes SHA-256 hash of a file
func (fs *FSFileWriter) calculateFileHash(filepath string) ([]byte, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file for hashing: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	buf := make([]byte, 4*1024*1024) // 4MB buffer
	reader := bufio.NewReaderSize(file, 4*1024*1024)

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			if _, err := hash.Write(buf[:n]); err != nil {
				return nil, fmt.Errorf("error writing to hash: %w", err)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading file for hash: %w", err)
		}
	}

	return hash.Sum(nil), nil
}
