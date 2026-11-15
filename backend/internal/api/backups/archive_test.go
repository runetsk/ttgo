package backups

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBackupArchiveRejectsUnsafeEntries(t *testing.T) {
	for _, kind := range []string{"path traversal", "duplicate database", "oversized database", "oversized signature"} {
		t.Run(kind, func(t *testing.T) {
			var data bytes.Buffer
			zw := zip.NewWriter(&data)
			if kind == "oversized database" {
				_, err := zw.CreateRaw(&zip.FileHeader{Name: "backup.db", UncompressedSize64: uint64(maxBackupBytes + 1), Method: zip.Store})
				require.NoError(t, err)
			} else {
				name := "backup.db"
				if kind == "path traversal" {
					name = "../backup.db"
				}
				out, err := zw.Create(name)
				require.NoError(t, err)
				_, err = out.Write([]byte("database"))
				require.NoError(t, err)
			}
			name := "signature.txt"
			if kind == "duplicate database" {
				name = "backup.db"
			}
			out, err := zw.Create(name)
			require.NoError(t, err)
			sig := []byte("signature")
			if kind == "oversized signature" {
				sig = bytes.Repeat([]byte("x"), 513)
			}
			_, err = out.Write(sig)
			require.NoError(t, err)
			require.NoError(t, zw.Close())
			path := filepath.Join(t.TempDir(), "upload.zip")
			require.NoError(t, os.WriteFile(path, data.Bytes(), 0600))
			_, _, err = prepareBackupUpload(path, "")
			require.Error(t, err)
		})
	}
}
