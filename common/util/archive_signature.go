package util

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

func ArchiveRequestSignature(key, timestamp, method, path, query string, body []byte) string {
	checksum := sha256.Sum256(body)
	mac := hmac.New(sha256.New, []byte(key))
	fmt.Fprintf(mac, "%s\n%s\n%s\n%s\n%x", timestamp, method, path, query, checksum)
	return hex.EncodeToString(mac.Sum(nil))
}
