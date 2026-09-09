package backup

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const backupFilePrefix = "ddns-updater-backup-"

func backupIncludesConfig() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("BACKUP_INCLUDE_CONFIG")))
	switch value {
	case "false", "no", "0", "off":
		return false
	default:
		// Preserve historical behavior unless explicitly disabled.
		return true
	}
}

func backupKeepCount() int {
	value := strings.TrimSpace(os.Getenv("BACKUP_KEEP"))
	if value == "" {
		return 0
	}
	keep, err := strconv.Atoi(value)
	if err != nil || keep < 1 {
		return 0
	}
	return keep
}

func backupInputFiles(dataDir string, includeConfig bool) []string {
	files := []string{filepath.Join(dataDir, "updates.json")}
	if includeConfig {
		files = append(files, filepath.Join(dataDir, "config.json"))
	}
	return files
}

func pruneBackups(outputDir string, keep int) error {
	if keep < 1 {
		return nil
	}

	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return err
	}

	backupNames := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, backupFilePrefix) && strings.HasSuffix(name, ".zip") {
			backupNames = append(backupNames, name)
		}
	}

	if len(backupNames) <= keep {
		return nil
	}

	// Filenames end in UnixNano, so lexical order matches creation order.
	sort.Strings(backupNames)
	for _, name := range backupNames[:len(backupNames)-keep] {
		if err := os.Remove(filepath.Join(outputDir, name)); err != nil {
			return err
		}
	}
	return nil
}
