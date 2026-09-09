package params

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvironmentConfigPersistenceCanBeDisabled(t *testing.T) {
	t.Setenv("CONFIG", `{"settings":[]}`)
	t.Setenv("CONFIG_PERSIST", "false")

	path := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, writeConfigFile(path, []byte("secret"), 0o600))

	_, err := os.Stat(path)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestEnvironmentConfigPersistenceDefaultsToEnabled(t *testing.T) {
	t.Setenv("CONFIG", `{"settings":[]}`)
	t.Setenv("CONFIG_PERSIST", "")

	path := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, writeConfigFile(path, []byte("content"), 0o600))

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "content", string(content))
}

func TestConfigPersistDoesNotAffectFileBasedConfigWrites(t *testing.T) {
	t.Setenv("CONFIG", "")
	t.Setenv("CONFIG_PERSIST", "false")

	path := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, writeConfigFile(path, []byte("content"), 0o600))

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "content", string(content))
}
