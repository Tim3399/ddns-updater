package backup

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func Test_run(t *testing.T) {
	t.Parallel()

	const backupPeriod = time.Hour

	testCases := map[string]struct {
		keep uint
		// duration is how long the backups run for.
		duration time.Duration
		// remaining lists the backups expected to remain, each as the
		// duration after which it was written.
		remaining []time.Duration
	}{
		"all_backups_kept": {
			keep:      0,
			duration:  3 * backupPeriod,
			remaining: []time.Duration{backupPeriod, 2 * backupPeriod, 3 * backupPeriod},
		},
		"oldest_backups_removed": {
			keep:      2,
			duration:  3 * backupPeriod,
			remaining: []time.Duration{2 * backupPeriod, 3 * backupPeriod},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// The fake clock of synctest makes the backup periods instant
			// and the timestamps in the backup file names predictable.
			synctest.Test(t, func(t *testing.T) {
				ctrl := gomock.NewController(t)

				dataDir := t.TempDir()
				for _, fileName := range [...]string{"config.json", "updates.json"} {
					err := os.WriteFile(filepath.Join(dataDir, fileName), []byte("{}"), 0o600)
					require.NoError(t, err)
				}
				outputDir := t.TempDir()

				logger := NewMockLogger(ctrl)
				logger.EXPECT().Info("each 1h0m0s; writing zip files to directory " + outputDir)

				start := time.Now()
				ready := make(chan struct{})
				runError := make(chan error)
				stopCh := make(chan struct{})
				done := make(chan struct{})
				go run(ready, runError, stopCh, done, outputDir, dataDir, backupPeriod, logger, testCase.keep)
				<-ready

				timer := time.NewTimer(testCase.duration)
				<-timer.C
				synctest.Wait()
				close(stopCh)
				<-done

				remaining := make([]string, len(testCase.remaining))
				for i, duration := range testCase.remaining {
					timestamp := start.Add(duration).UnixNano()
					remaining[i] = "ddns-updater-backup-" + strconv.Itoa(int(timestamp)) + ".zip"
				}
				entries, err := os.ReadDir(outputDir)
				require.NoError(t, err)
				names := make([]string, len(entries))
				for i, entry := range entries {
					names[i] = entry.Name()
				}
				assert.ElementsMatch(t, remaining, names)
			})
		})
	}
}
