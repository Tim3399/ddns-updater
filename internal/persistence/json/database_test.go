package json

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/qdm12/ddns-updater/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Database_write(t *testing.T) {
	t.Parallel()

	const existingContent = `{"records":[]}`
	data := dataModel{Records: []record{{Domain: "example.com", Owner: "home"}}}
	const dataContent = `{
  "records": [
    {
      "domain": "example.com",
      "owner": "home",
      "ips": null
    }
  ]
}
`

	testCases := map[string]struct {
		// existingPerm is the permission of the existing database file,
		// which does not exist if it is zero.
		existingPerm fs.FileMode
		// directoryName is the name of a directory to create in the directory
		// of the database, to have a path which cannot be written as a file.
		directoryName string
		data          dataModel
		content       string
		errMessage    string
	}{
		"database_created": {
			data:    data,
			content: dataContent,
		},
		"database_replaced_with_its_permissions": {
			existingPerm: 0o640,
			data:         data,
			content:      dataContent,
		},
		"database_written_in_place_without_temporary_file": {
			existingPerm:  0o600,
			directoryName: "updates.json.tmp",
			data:          data,
			content:       dataContent,
		},
		"database_kept_on_encoding_error": {
			existingPerm: 0o600,
			data: dataModel{Records: []record{{
				Events: []models.HistoryEvent{{Time: time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)}},
			}}},
			content:    existingContent,
			errMessage: "encoding data",
		},
		"database_path_is_a_directory": {
			directoryName: "updates.json",
			data:          data,
			errMessage:    "writing database file",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			directory := t.TempDir()
			databaseFilepath := filepath.Join(directory, "updates.json")
			if testCase.existingPerm != 0 {
				err := os.WriteFile(databaseFilepath, []byte(existingContent), testCase.existingPerm)
				require.NoError(t, err)
				// The permission given when creating the file is subject to the umask.
				err = os.Chmod(databaseFilepath, testCase.existingPerm)
				require.NoError(t, err)
			}
			if testCase.directoryName != "" {
				err := os.Mkdir(filepath.Join(directory, testCase.directoryName), 0o700)
				require.NoError(t, err)
			}
			db := &Database{
				filepath: databaseFilepath,
				data:     testCase.data,
			}

			err := db.write()

			if testCase.errMessage != "" {
				assert.ErrorContains(t, err, testCase.errMessage)
			} else {
				assert.NoError(t, err)
			}

			entries, err := os.ReadDir(directory)
			require.NoError(t, err)
			for _, entry := range entries {
				if entry.Name() != testCase.directoryName {
					assert.Equal(t, "updates.json", entry.Name())
				}
			}

			if testCase.content == "" {
				return
			}
			content, err := os.ReadFile(databaseFilepath)
			require.NoError(t, err)
			assert.Equal(t, testCase.content, string(content))

			// Windows does not support POSIX permission bits.
			if testCase.existingPerm != 0 && runtime.GOOS != "windows" {
				fileInfo, err := os.Stat(databaseFilepath)
				require.NoError(t, err)
				assert.Equal(t, testCase.existingPerm, fileInfo.Mode().Perm())
			}
		})
	}
}
