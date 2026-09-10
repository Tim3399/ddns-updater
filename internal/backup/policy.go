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
		if err := os.Remove(filepath.Join(outputDir, candidate.name)); err != nil {
			return err
		}
	}
	return nil
}

func readBackupCandidate(outputDir string, entry os.DirEntry) (backupCandidate, bool) {
	if entry.IsDir() {
		return backupCandidate{}, false
	}
	name := entry.Name()
	if !strings.HasPrefix(name, backupFilePrefix) || !strings.HasSuffix(name, ".zip") {
		return backupCandidate{}, false
	}
	timestampString := strings.TrimSuffix(strings.TrimPrefix(name, backupFilePrefix), ".zip")
	timestamp, err := strconv.ParseInt(timestampString, 10, 64)
	if err != nil || !validZip(filepath.Join(outputDir, name)) {
		return backupCandidate{}, false
	}
	info, err := entry.Info()
	if err != nil {
		return backupCandidate{}, false
	}
	candidateTime := info.ModTime()
	if timestamp > math.MaxInt32 || timestamp < math.MinInt32 {
		candidateTime = time.Unix(0, timestamp)
	}
	return backupCandidate{name: name, time: candidateTime}, true
}

func validZip(path string) bool {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return false
	}
	defer reader.Close()
	entries := make(map[string]struct{}, len(reader.File))
	remainingBytes := int64(maxRetentionArchiveSize)
	for _, file := range reader.File {
		entries[file.Name] = struct{}{}
		fileReader, err := file.Open()
		if err != nil {
			return false
		}
		bytesRead, copyErr := io.CopyN(io.Discard, fileReader, remainingBytes+1)
		closeErr := fileReader.Close()
		if bytesRead > remainingBytes ||
			(copyErr != nil && !errors.Is(copyErr, io.EOF)) || closeErr != nil {
			return false
		}
		remainingBytes -= bytesRead
	}
	if reader.Comment == "" {
		_, hasUpdates := entries["updates.json"]
		_, hasConfig := entries["config.json"]
		return hasUpdates && hasConfig
	}
	if !strings.HasPrefix(reader.Comment, archiveCommentPrefix) {
		return false
	}
	expectedNames := strings.Split(strings.TrimPrefix(reader.Comment, archiveCommentPrefix), ",")
	if len(expectedNames) == 0 {
		return false
	}
	for _, name := range expectedNames {
		if name == "" {
			return false
		}
		if _, ok := entries[name]; !ok {
			return false
		}
	}
	_, hasUpdates := entries["updates.json"]
	return hasUpdates
}
