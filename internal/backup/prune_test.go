package backup

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_pruneBackups(t *testing.T) {
	t.Parallel()

	const updatesContent = `{"records":[]}`
	backup := makeZip(t, zip.Deflate, map[string]string{
		"updates.json": updatesContent,
		"config.json":  `{"settings":[]}`,
	})
	withoutUpdates := makeZip(t, zip.Deflate, map[string]string{
		"config.json": `{"settings":[]}`,
	})
	// Flip a byte of the uncompressed content so its checksum does not match.
	corrupted := makeZip(t, zip.Store, map[string]string{"updates.json": updatesContent})
	contentIndex := bytes.Index(corrupted, []byte(updatesContent))
	require.NotEqual(t, -1, contentIndex)
	corrupted[contentIndex] ^= 0xff

	type file struct {
		content []byte
		age     time.Duration
	}

	testCases := map[string]struct {
		keep             uint
		files            map[string]file
		missingDirectory bool
		remaining        []string
		errMessage       string
	}{
		"keep_zero_keeps_all": {
			keep: 0,
			files: map[string]file{
				"ddns-updater-backup-1.zip": {content: backup, age: 2 * time.Hour},
				"ddns-updater-backup-2.zip": {content: backup, age: time.Hour},
			},
			remaining: []string{"ddns-updater-backup-1.zip", "ddns-updater-backup-2.zip"},
		},
		"fewer_backups_than_keep": {
			keep: 2,
			files: map[string]file{
				"ddns-updater-backup-1.zip": {content: backup, age: time.Hour},
			},
			remaining: []string{"ddns-updater-backup-1.zip"},
		},
		"oldest_backups_removed": {
			keep: 2,
			files: map[string]file{
				"ddns-updater-backup-1.zip": {content: backup, age: 3 * time.Hour},
				"ddns-updater-backup-2.zip": {content: backup, age: time.Hour},
				"ddns-updater-backup-3.zip": {content: backup, age: 2 * time.Hour},
			},
			remaining: []string{"ddns-updater-backup-2.zip", "ddns-updater-backup-3.zip"},
		},
		"other_files_not_counted_nor_removed": {
			keep: 1,
			files: map[string]file{
				"ddns-updater-backup-1.zip": {content: []byte("not a zip file"), age: 6 * time.Hour},
				"ddns-updater-backup-2.zip": {content: withoutUpdates, age: 5 * time.Hour},
				"ddns-updater-backup-3.zip": {content: corrupted, age: 4 * time.Hour},
				"other.zip":                 {content: backup, age: 3 * time.Hour},
				"ddns-updater-backup-4.zip": {content: backup, age: 2 * time.Hour},
				"ddns-updater-backup-5.zip": {content: backup, age: time.Hour},
			},
			remaining: []string{
				"ddns-updater-backup-1.zip", "ddns-updater-backup-2.zip",
				"ddns-updater-backup-3.zip", "other.zip", "ddns-updater-backup-5.zip",
			},
		},
		"missing_directory": {
			keep:             1,
			missingDirectory: true,
			errMessage:       "reading backup directory",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			directory := t.TempDir()
			if testCase.missingDirectory {
				directory = filepath.Join(directory, "missing")
			}
			now := time.Now()
			for fileName, file := range testCase.files {
				path := filepath.Join(directory, fileName)
				err := os.WriteFile(path, file.content, 0o600)
				require.NoError(t, err)
				modTime := now.Add(-file.age)
				err = os.Chtimes(path, modTime, modTime)
				require.NoError(t, err)
			}

			err := pruneBackups(directory, testCase.keep)

			if testCase.errMessage != "" {
				assert.ErrorContains(t, err, testCase.errMessage)
				return
			}
			require.NoError(t, err)
			entries, err := os.ReadDir(directory)
			require.NoError(t, err)
			remaining := make([]string, len(entries))
			for i, entry := range entries {
				remaining[i] = entry.Name()
			}
			assert.ElementsMatch(t, testCase.remaining, remaining)
		})
	}
}

func makeZip(t *testing.T, method uint16, files map[string]string) []byte {
	t.Helper()

	buffer := bytes.NewBuffer(nil)
	writer := zip.NewWriter(buffer)
	for name, content := range files {
		fileWriter, err := writer.CreateHeader(&zip.FileHeader{Name: name, Method: method})
		require.NoError(t, err)
		_, err = fileWriter.Write([]byte(content))
		require.NoError(t, err)
	}
	err := writer.Close()
	require.NoError(t, err)
	return buffer.Bytes()
}
