package backups

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maxBackupBytes int64 = 200 << 20

// The archive carries the existing HMAC alongside the database, never the key.
func writeBackupArchive(w io.Writer, path, signature string) error {
	in, err := os.Open(path)
	if err != nil {
		return err
	}
	defer in.Close()
	zw := zip.NewWriter(w)
	defer zw.Close()
	db, err := zw.Create("backup.db")
	if err != nil {
		return err
	}
	if _, err := io.Copy(db, in); err != nil {
		return err
	}
	sig, err := zw.Create("signature.txt")
	if err != nil {
		return err
	}
	if _, err := io.WriteString(sig, signature); err != nil {
		return err
	}
	return zw.Close()
}

// prepareBackupUpload accepts a signed archive or the existing raw DB+signature
// contract. It extracts only the two fixed entries, with bounded expansion.
func prepareBackupUpload(path, signature string) (string, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", err
	}
	var header [2]byte
	_, err = io.ReadFull(f, header[:])
	f.Close()
	if err != nil {
		return "", "", err
	}
	if string(header[:]) != "PK" {
		return path, signature, nil
	}
	zr, err := zip.OpenReader(path)
	if err != nil {
		return "", "", fmt.Errorf("invalid backup archive: %w", err)
	}
	defer zr.Close()
	if len(zr.File) != 2 {
		return "", "", fmt.Errorf("backup archive must contain backup.db and signature.txt")
	}
	var dbEntry, sigEntry *zip.File
	for _, entry := range zr.File {
		switch entry.Name {
		case "backup.db":
			dbEntry = entry
		case "signature.txt":
			sigEntry = entry
		default:
			return "", "", fmt.Errorf("unexpected backup archive entry")
		}
	}
	if dbEntry == nil || sigEntry == nil || dbEntry.UncompressedSize64 > uint64(maxBackupBytes) || sigEntry.UncompressedSize64 > 512 {
		return "", "", fmt.Errorf("invalid or oversized backup archive")
	}
	sig, err := sigEntry.Open()
	if err != nil {
		return "", "", err
	}
	sigBytes, err := io.ReadAll(io.LimitReader(sig, 513))
	sig.Close()
	if err != nil || len(sigBytes) > 512 {
		return "", "", fmt.Errorf("invalid backup signature entry")
	}
	db, err := dbEntry.Open()
	if err != nil {
		return "", "", err
	}
	defer db.Close()
	out, err := os.CreateTemp(filepath.Dir(path), "restore-*.db")
	if err != nil {
		return "", "", err
	}
	n, copyErr := io.Copy(out, io.LimitReader(db, maxBackupBytes+1))
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil || n > maxBackupBytes {
		os.Remove(out.Name())
		return "", "", fmt.Errorf("invalid or oversized backup database")
	}
	return out.Name(), strings.TrimSpace(string(sigBytes)), nil
}
