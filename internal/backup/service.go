package backup

import (
	"context"
	"path/filepath"
	"strconv"
	"time"
)

const backupFilePrefix = "ddns-updater-backup-"

type Options struct {
	ConfigFilepath string
	IncludeConfig  bool
	Keep           int
}

type Service struct {
	// Injected fields
	backupPeriod time.Duration
	dataDir      string
	outputDir    string
	logger       Logger
	options      Options

	// Internal fields
	stopCh chan<- struct{}
	done   <-chan struct{}
}

func New(backupPeriod time.Duration,
	dataDir, outputDir string, logger Logger, options ...Options,
) *Service {
	backupOptions := Options{
		ConfigFilepath: filepath.Join(dataDir, "config.json"),
		IncludeConfig:  true,
	}
	if len(options) > 0 {
		backupOptions = options[0]
	}
	return &Service{
		logger:       logger,
		backupPeriod: backupPeriod,
		dataDir:      dataDir,
		outputDir:    outputDir,
		options:      backupOptions,
	}
}

func (s *Service) String() string {
	return "backup"
}

func makeZipFileName() string {
	return backupFilePrefix + strconv.FormatInt(time.Now().UnixNano(), 10) + ".zip"
}

func (s *Service) Start(ctx context.Context) (runError <-chan error, startErr error) {
	ready := make(chan struct{})
	runErrorCh := make(chan error)
	stopCh := make(chan struct{})
	s.stopCh = stopCh
	done := make(chan struct{})
	s.done = done
	go run(ready, runErrorCh, stopCh, done,
		s.outputDir, s.dataDir, s.backupPeriod, s.logger, s.options)
	select {
	case <-ready:
	case <-ctx.Done():
		return nil, s.Stop()
	}
	return runErrorCh, nil
}

func run(ready chan<- struct{}, runError chan<- error, stopCh <-chan struct{},
	done chan<- struct{}, outputDir, dataDir string, backupPeriod time.Duration,
	logger Logger, options Options,
) {
	defer close(done)

	if backupPeriod == 0 {
		close(ready)
		logger.Info("disabled")
		return
	}

	logger.Info("each " + backupPeriod.String() +
		"; writing zip files to directory " + outputDir)
	timer := time.NewTimer(backupPeriod)
	close(ready)

	for {
		select {
		case <-timer.C:
		case <-stopCh:
			_ = timer.Stop()
			return
		}
		files := []archiveFile{{
			filepath: filepath.Join(dataDir, "updates.json"),
			name:     "updates.json",
		}}
		if options.IncludeConfig {
			files = append(files, archiveFile{
				filepath: options.ConfigFilepath,
				name:     "config.json",
			})
		}
		err := zipArchive(filepath.Join(outputDir, makeZipFileName()), files)
		if err != nil {
			runError <- err
			return
		}
		err = pruneBackups(outputDir, options.Keep)
		if err != nil {
			runError <- err
			return
		}
		timer.Reset(backupPeriod)
	}
}

func (s *Service) Stop() (err error) {
	close(s.stopCh)
	<-s.done
	return nil
}
