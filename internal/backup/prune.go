package backup

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// pruneBackups removes the oldest backups from the directory so that at most
// keep backups remain. Files which are not readable backup archives are
// neither counted nor removed. A keep value of zero keeps all backups.
func pruneBackups(directory string, keep uint) error {
	if keep == 0 {
		return nil
	}

	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("reading backup directory: %w", err)
	}

	type backup struct {
		path    string
		modTime time.Time
	}
	backups := make([]backup, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, zipFileNamePrefix) || !strings.HasSuffix(name, ".zip") {
			continue
		}
		path := filepath.Join(directory, name)
		if !isBackupArchive(path) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		backups = append(backups, backup{path: path, modTime: info.ModTime()})
	}

	if uint(len(backups)) <= keep {
		return nil
	}

	slices.SortStableFunc(backups, func(left, right backup) int {
		return left.modTime.Compare(right.modTime)
	})
	for _, backup := range backups[:uint(len(backups))-keep] {
		err = os.Remove(backup.path)
		if err != nil {
			return fmt.Errorf("removing backup: %w", err)
		}
	}
	return nil
}

// isBackupArchive reports whether the file is a ZIP archive containing the
// updates file, with every archived file passing its checksum verification.
func isBackupArchive(path string) bool {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return false
	}
	defer reader.Close()

	hasUpdates := false
	for _, file := range reader.File {
		if !isReadable(file) {
			return false
		}
		if file.Name == "updates.json" {
			hasUpdates = true
		}
	}
	return hasUpdates
}

func isReadable(file *zip.File) bool {
	reader, err := file.Open()
	if err != nil {
		return false
	}
	defer reader.Close()
	// The checksum is verified once the file is read to its end, and the data
	// files are far smaller than this limit on what gets decompressed.
	const maxSize = 64 << 20
	size, err := io.CopyN(io.Discard, reader, maxSize+1)
	return errors.Is(err, io.EOF) && size <= maxSize
}
