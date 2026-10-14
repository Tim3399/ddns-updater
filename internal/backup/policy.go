package backup

import (
	"archive/zip"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const maxRetentionArchiveSize = 64 << 20

type backupCandidate struct {
	name string
	time time.Time
}

// pruneBackups deletes the oldest backup archives in outputDir so that at most
// keep archives remain. A keep value of zero or less keeps all archives.
func pruneBackups(outputDir string, keep int) error {
	if keep <= 0 {
		return nil
	}
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return err
	}
	candidates := make([]backupCandidate, 0, len(entries))
	for _, entry := range entries {
		candidate, ok := readBackupCandidate(outputDir, entry)
		if ok {
			candidates = append(candidates, candidate)
		}
	}
	if len(candidates) <= keep {
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].time.Equal(candidates[j].time) {
			return candidates[i].name < candidates[j].name
		}
		return candidates[i].time.Before(candidates[j].time)
	})
	for _, candidate := range candidates[:len(candidates)-keep] {
		err = os.Remove(filepath.Join(outputDir, candidate.name))
		if err != nil {
			return err
		}
	}
	return nil
}

// readBackupCandidate returns the directory entry as a pruning candidate
// only if it is a complete backup archive written by this program,
// so that no other file in the backup directory is ever deleted.
func readBackupCandidate(outputDir string, entry os.DirEntry) (backupCandidate, bool) {
	if entry.IsDir() {
		return backupCandidate{}, false
	}
	name := entry.Name()
	timestampString, ok := strings.CutPrefix(name, backupFilePrefix)
	if !ok {
		return backupCandidate{}, false
	}
	timestampString, ok = strings.CutSuffix(timestampString, ".zip")
	if !ok {
		return backupCandidate{}, false
	}
	timestamp, err := strconv.ParseInt(timestampString, 10, 64)
	if err != nil || !isCompleteBackup(filepath.Join(outputDir, name)) {
		return backupCandidate{}, false
	}
	info, err := entry.Info()
	if err != nil {
		return backupCandidate{}, false
	}
	// Older versions built the file name from int(time.Now().UnixNano()),
	// which wraps around on 32-bit platforms. Only names outside the int32
	// range are trusted as timestamps, otherwise the file modification time
	// is used to order backups.
	candidateTime := info.ModTime()
	if timestamp > math.MaxInt32 || timestamp < math.MinInt32 {
		candidateTime = time.Unix(0, timestamp)
	}
	return backupCandidate{name: name, time: candidateTime}, true
}

// isCompleteBackup reports whether the ZIP archive at path is intact and
// contains every file a backup needs.
func isCompleteBackup(path string) bool {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return false
	}
	defer reader.Close()
	entries, ok := readArchiveEntries(reader.File)
	if !ok {
		return false
	}
	return hasRequiredEntries(reader.Comment, entries)
}

// readArchiveEntries decompresses every file of the archive, reading at most
// maxRetentionArchiveSize bytes in total, and returns the set of file names.
// It returns false if any file cannot be read in full.
func readArchiveEntries(files []*zip.File) (entries map[string]struct{}, ok bool) {
	entries = make(map[string]struct{}, len(files))
	remainingBytes := int64(maxRetentionArchiveSize)
	for _, file := range files {
		entries[file.Name] = struct{}{}
		fileReader, err := file.Open()
		if err != nil {
			return nil, false
		}
		bytesRead, copyErr := io.CopyN(io.Discard, fileReader, remainingBytes+1)
		closeErr := fileReader.Close()
		if bytesRead > remainingBytes ||
			(copyErr != nil && !errors.Is(copyErr, io.EOF)) || closeErr != nil {
			return nil, false
		}
		remainingBytes -= bytesRead
	}
	return entries, true
}

// hasRequiredEntries checks the archive entries against the file names listed
// in the archive comment. Archives written before the comment was introduced
// have no comment and always contain both the updates and config files.
func hasRequiredEntries(comment string, entries map[string]struct{}) bool {
	if comment == "" {
		return hasEntry(entries, updatesArchiveName) && hasEntry(entries, configArchiveName)
	}
	listedNames, ok := strings.CutPrefix(comment, archiveCommentPrefix)
	if !ok {
		return false
	}
	for name := range strings.SplitSeq(listedNames, ",") {
		if name == "" || !hasEntry(entries, name) {
			return false
		}
	}
	return hasEntry(entries, updatesArchiveName)
}

func hasEntry(entries map[string]struct{}, name string) bool {
	_, ok := entries[name]
	return ok
}
