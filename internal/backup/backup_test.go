package backup

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestZipFilesPublishesOnlyCompleteArchives(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	inputPath := filepath.Join(directory, "updates.json")
	require.NoError(t, os.WriteFile(inputPath, []byte(`{"records":[]}`), 0o600))
	outputPath := filepath.Join(directory, backupFilePrefix+"1.zip")

	err := zipFiles(outputPath, inputPath, filepath.Join(directory, "missing.json"))
	require.Error(t, err)
	_, statErr := os.Stat(outputPath)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
	temporaryFiles, globErr := filepath.Glob(filepath.Join(directory, ".ddns-updater-backup-*.tmp"))
	require.NoError(t, globErr)
	assert.Empty(t, temporaryFiles)
}

func TestZipArchiveUsesRecoveryNames(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	customConfigPath := filepath.Join(directory, "custom-name.json")
	updatesPath := filepath.Join(directory, "updates.json")
	require.NoError(t, os.WriteFile(customConfigPath, []byte(`{"settings":[]}`), 0o600))
	require.NoError(t, os.WriteFile(updatesPath, []byte(`{"records":[]}`), 0o600))
	outputPath := filepath.Join(directory, backupFilePrefix+"2.zip")

	require.NoError(t, zipArchive(outputPath, []archiveFile{
		{filepath: updatesPath, name: "updates.json"},
		{filepath: customConfigPath, name: "config.json"},
	}))
	reader, err := zip.OpenReader(outputPath)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, reader.Close()) })
	require.Len(t, reader.File, 2)
	assert.Equal(t, "updates.json", reader.File[0].Name)
	assert.Equal(t, "config.json", reader.File[1].Name)
}

func TestPruneBackupsKeepsNewestValidArchives(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	inputPath := filepath.Join(directory, "updates.json")
	require.NoError(t, os.WriteFile(inputPath, []byte(`{"records":[]}`), 0o600))
	oldPath := filepath.Join(directory, backupFilePrefix+"1000000000000000001.zip")
	newPath := filepath.Join(directory, backupFilePrefix+"1000000000000000002.zip")
	require.NoError(t, zipFiles(oldPath, inputPath))
	require.NoError(t, zipFiles(newPath, inputPath))
	corruptPath := filepath.Join(directory, backupFilePrefix+"1000000000000000003.zip")
	require.NoError(t, os.WriteFile(corruptPath, []byte("not a zip"), 0o600))
	unrelatedPath := filepath.Join(directory, "other.zip")
	require.NoError(t, os.WriteFile(unrelatedPath, []byte("unrelated"), 0o600))

	require.NoError(t, pruneBackups(directory, 1))
	_, err := os.Stat(oldPath)
	assert.ErrorIs(t, err, os.ErrNotExist)
	for _, path := range []string{newPath, corruptPath, unrelatedPath} {
		_, err := os.Stat(path)
		assert.NoError(t, err)
	}
}

func TestPruneBackupsOrdersLegacyInt32NamesByModificationTime(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	inputPath := filepath.Join(directory, "updates.json")
	require.NoError(t, os.WriteFile(inputPath, []byte(`{"records":[]}`), 0o600))
	legacyNames := []string{"2147483640", "-2147483646", "-2147483636"}
	paths := make([]string, len(legacyNames))
	baseTime := time.Unix(1_700_000_000, 0)
	for i, legacyName := range legacyNames {
		name := backupFilePrefix + legacyName + ".zip"
		paths[i] = filepath.Join(directory, name)
		require.NoError(t, zipFiles(paths[i], inputPath))
		fileTime := baseTime.Add(time.Duration(i) * time.Second)
		require.NoError(t, os.Chtimes(paths[i], fileTime, fileTime))
	}

	require.NoError(t, pruneBackups(directory, 2))
	_, err := os.Stat(paths[0])
	assert.ErrorIs(t, err, os.ErrNotExist)
	for _, path := range paths[1:] {
		_, err := os.Stat(path)
		assert.NoError(t, err)
	}
}

func TestValidZipRejectsOversizedArchive(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), backupFilePrefix+"1000000000000000001.zip")
	require.NoError(t, writeValidationZip(path, int64(maxRetentionArchiveSize)+1))
	assert.False(t, validZip(path))
	require.NoError(t, pruneBackups(filepath.Dir(path), 1))
	_, err := os.Stat(path)
	assert.NoError(t, err, "oversized archives are preserved but excluded from retention")
}

func TestValidZipChecksCRC(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), backupFilePrefix+"1000000000000000001.zip")
	require.NoError(t, writeValidationZip(path, 32))
	reader, err := zip.OpenReader(path)
	require.NoError(t, err)
	require.Len(t, reader.File, 1)
	offset, err := reader.File[0].DataOffset()
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	content[offset] ^= 0xff
	// The path is created from t.TempDir above and is not externally controlled.
	//nolint:gosec
	require.NoError(t, os.WriteFile(path, content, 0o600))

	assert.False(t, validZip(path))
}

func TestPruneAfterFailedBackupPreservesExistingArchives(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	inputPath := filepath.Join(directory, "updates.json")
	require.NoError(t, os.WriteFile(inputPath, []byte(`{"records":[]}`), 0o600))
	oldPath := filepath.Join(directory, backupFilePrefix+"1000000000000000001.zip")
	newPath := filepath.Join(directory, backupFilePrefix+"1000000000000000003.zip")
	require.NoError(t, zipFiles(oldPath, inputPath))
	require.Error(t, zipFiles(filepath.Join(directory, backupFilePrefix+"1000000000000000002.zip"),
		inputPath, filepath.Join(directory, "missing.json")))
	require.NoError(t, zipFiles(newPath, inputPath))

	require.NoError(t, pruneBackups(directory, 2))
	for _, path := range []string{oldPath, newPath} {
		_, err := os.Stat(path)
		assert.NoError(t, err)
	}
}

func TestPruneIgnoresStructurallyValidLegacyPartialArchive(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	inputPath := filepath.Join(directory, "updates.json")
	require.NoError(t, os.WriteFile(inputPath, []byte(`{"records":[]}`), 0o600))
	oldPath := filepath.Join(directory, backupFilePrefix+"1000000000000000001.zip")
	partialPath := filepath.Join(directory, backupFilePrefix+"1000000000000000002.zip")
	newPath := filepath.Join(directory, backupFilePrefix+"1000000000000000003.zip")
	require.NoError(t, zipFiles(oldPath, inputPath))
	require.NoError(t, writeLegacyZip(partialPath, inputPath))
	require.NoError(t, zipFiles(newPath, inputPath))

	require.NoError(t, pruneBackups(directory, 2))
	for _, path := range []string{oldPath, partialPath, newPath} {
		_, err := os.Stat(path)
		assert.NoError(t, err)
	}
}

func TestPruneCountsCompleteLegacyArchives(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	updatesPath := filepath.Join(directory, "updates.json")
	configPath := filepath.Join(directory, "config.json")
	require.NoError(t, os.WriteFile(updatesPath, []byte(`{"records":[]}`), 0o600))
	require.NoError(t, os.WriteFile(configPath, []byte(`{"settings":[]}`), 0o600))
	legacyPath := filepath.Join(directory, backupFilePrefix+"1000000000000000001.zip")
	newPath := filepath.Join(directory, backupFilePrefix+"1000000000000000002.zip")
	require.NoError(t, writeLegacyZip(legacyPath, updatesPath, configPath))
	require.NoError(t, zipFiles(newPath, updatesPath))

	require.NoError(t, pruneBackups(directory, 1))
	_, err := os.Stat(legacyPath)
	assert.ErrorIs(t, err, os.ErrNotExist)
	_, err = os.Stat(newPath)
	assert.NoError(t, err)
}

func writeLegacyZip(outputPath string, inputPaths ...string) error {
	output, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	writer := zip.NewWriter(output)
	for _, inputPath := range inputPaths {
		if err := addFile(writer, inputPath, filepath.Base(inputPath)); err != nil {
			return closeFailedArchive(writer, output, err)
		}
	}
	if err := writer.Close(); err != nil {
		_ = output.Close()
		return err
	}
	return output.Close()
}

type zeroReader struct{}

func (zeroReader) Read(buffer []byte) (int, error) {
	clear(buffer)
	return len(buffer), nil
}

func writeValidationZip(outputPath string, size int64) error {
	output, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	writer := zip.NewWriter(output)
	if err := writer.SetComment(archiveCommentPrefix + "updates.json"); err != nil {
		return closeFailedArchive(writer, output, err)
	}
	method := zip.Store
	if size > maxRetentionArchiveSize {
		method = zip.Deflate
	}
	header := &zip.FileHeader{Name: "updates.json", Method: method}
	entry, err := writer.CreateHeader(header)
	if err != nil {
		return closeFailedArchive(writer, output, err)
	}
	if _, err := io.Copy(entry, io.LimitReader(zeroReader{}, size)); err != nil {
		return closeFailedArchive(writer, output, err)
	}
	if err := writer.Close(); err != nil {
		_ = output.Close()
		return err
	}
	return output.Close()
}
