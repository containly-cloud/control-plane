package storage

import (
	"crypto/sha256"
	"encoding/hex"
)

func checksum(contents []byte) string {
	sum := sha256.Sum256(contents)
	return hex.EncodeToString(sum[:])
}
