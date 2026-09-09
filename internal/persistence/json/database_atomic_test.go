package json

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteAtomicallyReplacesDatabase(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	databasePath := filepath.Join(directory, "updates.json")
	require.NoError(t, os.WriteFile(databasePath, []byte(`{"records":[]}`), 0o600))

	db := &Database{
		filepath: databasePath,
		data: dataModel{
			Records: []record{
				{
					Domain: "example.com",
					Owner:  "home",
				},
			},
		},
	}

	require.NoError(t, db.write())

	content, err := os.ReadFile(databasePath)
	require.NoError(t, err)

	var persisted dataModel
	require.NoError(t, json.Unmarshal(content, &persisted))
	require.Len(t, persisted.Records, 1)
	assert.Equal(t, "example.com", persisted.Records[0].Domain)
	assert.Equal(t, "home", persisted.Records[0].Owner)

	tempFiles, err := filepath.Glob(filepath.Join(directory, ".updates.json.tmp-*"))
	require.NoError(t, err)
	assert.Empty(t, tempFiles)
}
