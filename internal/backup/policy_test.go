package backup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackupIncludeConfigCanBeDisabled(t *testing.T) {
	t.Setenv("BACKUP_INCLUDE_CONFIG", "false")
	assert.False(t, backupIncludesConfig())

	files := backupInputFiles("/data", backupIncludesConfig())
	assert.Equal(t, []string{filepath.Join("/data", "updates.json")}, files)
}

func TestBackupIncludeConfigDefaultsToEnabled(t *testing.T) {
	t.Setenv("BACKUP_INCLUDE_CONFIG", "")
	assert.True(t, backupIncludesConfig())
}

func TestBackupKeepCount(t *testing.T) {
	t.Setenv("BACKUP_KEEP", "7")
	assert.Equal(t, 7, backupKeepCount())

	t.Setenv("BACKUP_KEEP", "invalid")
	assert.Zero(t, backupKeepCount())
}

func TestPruneBackupsKeepsNewestFiles(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	for _, name := range []string{
		"ddns-updater-backup-0001.zip",
		"ddns-updater-backup-0002.zip",
		"ddns-updater-backup-0003.zip",
		"unrelated.zip",
	} {
		require.NoError(t, os.WriteFile(filepath.Join(directory, name), []byte("test"), 0o600))
	}

	require.NoError(t, pruneBackups(directory, 2))

	_, err := os.Stat(filepath.Join(directory, "ddns-updater-backup-0001.zip"))
	assert.ErrorIs(t, err, os.ErrNotExist)
	for _, name := range []string{
		"ddns-updater-backup-0002.zip",
		"ddns-updater-backup-0003.zip",
		"unrelated.zip",
	} {
		_, err := os.Stat(filepath.Join(directory, name))
		assert.NoError(t, err)
	}
}
