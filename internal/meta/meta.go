package meta

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"time"
)

// FileMeta carries file properties for upload.
type FileMeta struct {
	Path     string
	OrigName string
	SHA256   string
	MIME     string
	FlowID   string
	SrcIP    string
	DstIP    string
	Sensor   string
	TS       time.Time
	Size     int64
}

// HashSHA256 returns SHA-256 and file size.
func HashSHA256(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil { return "", 0, err }
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil { return "", 0, err }
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// GuessMIME guesses MIME from file extension.
func GuessMIME(filename string) string {
	ext := filepath.Ext(filename)
	switch ext {
	case ".txt":
		return "text/plain"
	case ".json":
		return "application/json"
	case ".pcap":
		return "application/vnd.tcpdump.pcap"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	default:
		return "application/octet-stream"
	}
}
