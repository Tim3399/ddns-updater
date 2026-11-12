package params

import (
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// Test_Reader_getProvidersFromEnv cannot run in parallel since it sets
// the CONFIG environment variable.
func Test_Reader_getProvidersFromEnv(t *testing.T) {
	type writtenFile struct {
		path string
		data string
		perm fs.FileMode
	}

	testCases := map[string]struct {
		persistEnvironmentConfig bool
		writtenFiles             []writtenFile
	}{
		"persistence_disabled": {},
		"persistence_enabled": {
			persistEnvironmentConfig: true,
			writtenFiles: []writtenFile{{
				path: "config.json",
				data: "{\n  \"settings\": []\n}",
				perm: 0o666,
			}},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			t.Setenv("CONFIG", `{"settings":[]}`)

			logger := NewMockLogger(ctrl)
			logger.EXPECT().Info("reading JSON config from environment variable CONFIG")
			logger.EXPECT().Debug(`config read: {"settings":[]}`)

			var writtenFiles []writtenFile
			reader := &Reader{
				logger: logger,
				writeFile: func(filename string, data []byte, perm fs.FileMode) error {
					writtenFiles = append(writtenFiles, writtenFile{
						path: filename,
						data: string(data),
						perm: perm,
					})
					return nil
				},
				persistEnvironmentConfig: testCase.persistEnvironmentConfig,
			}

			providers, warnings, err := reader.getProvidersFromEnv("config.json")

			assert.NoError(t, err)
			assert.Empty(t, providers)
			assert.Empty(t, warnings)
			assert.Equal(t, testCase.writtenFiles, writtenFiles)
		})
	}
}
