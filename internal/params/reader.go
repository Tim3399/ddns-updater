package params

import (
	"io/fs"
	"os"
	"strings"
)

type Reader struct {
	logger    Logger
	readFile  func(filename string) ([]byte, error)
	writeFile func(filename string, data []byte, perm fs.FileMode) (err error)
}

type Logger interface {
	Info(s string)
	Debug(s string)
}

func NewReader(logger Logger) *Reader {
	return &Reader{
		logger:    logger,
		readFile:  os.ReadFile,
		writeFile: writeConfigFile,
	}
}

// writeConfigFile preserves the historical behavior of persisting CONFIG to
// disk unless CONFIG_PERSIST is explicitly disabled. File-based configuration
// is unaffected by this switch.
func writeConfigFile(filename string, data []byte, perm fs.FileMode) error {
	if os.Getenv("CONFIG") != "" && !environmentConfigPersistenceEnabled() {
		return nil
	}
	return os.WriteFile(filename, data, perm)
}

func environmentConfigPersistenceEnabled() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("CONFIG_PERSIST")))
	switch value {
	case "false", "no", "0", "off":
		return false
	default:
		return true
	}
}
