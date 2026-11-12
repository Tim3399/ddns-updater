package backup

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_zipFiles(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		// files maps the name of each file to archive to its content.
		files map[string]string
		// archiveNames maps the name of each file to archive to its name in
		// the archive. Names without a file on disk are still requested.
		archiveNames map[string]string
		// archived maps the name of each archived file to its content.
		archived   map[string]string
		errMessage string
	}{
		"files_archived_under_their_archive_names": {
			files: map[string]string{
				"updates.json": `{"records":[]}`,
				"custom.json":  `{"settings":[]}`,
			},
			archiveNames: map[string]string{
				"updates.json": "updates.json",
				"custom.json":  "config.json",
			},
			archived: map[string]string{
				"updates.json": `{"records":[]}`,
				"config.json":  `{"settings":[]}`,
			},
		},
		"missing_file": {
			archiveNames: map[string]string{"updates.json": "updates.json"},
			errMessage:   "no such file or directory",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			directory := t.TempDir()
			for fileName, content := range testCase.files {
				err := os.WriteFile(filepath.Join(directory, fileName), []byte(content), 0o600)
				require.NoError(t, err)
			}
			files := make([]archiveFile, 0, len(testCase.archiveNames))
			for fileName, archiveName := range testCase.archiveNames {
				files = append(files, archiveFile{
					path: filepath.Join(directory, fileName),
					name: archiveName,
				})
			}
			outputFilepath := filepath.Join(directory, "backup.zip")

			err := zipFiles(outputFilepath, files...)

			if testCase.errMessage != "" {
				assert.ErrorContains(t, err, testCase.errMessage)
				return
			}
			require.NoError(t, err)
			reader, err := zip.OpenReader(outputFilepath)
			require.NoError(t, err)
			defer reader.Close()
			archived := make(map[string]string, len(reader.File))
			for _, file := range reader.File {
				fileReader, err := file.Open()
				require.NoError(t, err)
				content, err := io.ReadAll(fileReader)
				require.NoError(t, err)
				err = fileReader.Close()
				require.NoError(t, err)
				archived[file.Name] = string(content)
			}
			assert.Equal(t, testCase.archived, archived)
		})
	}
}
