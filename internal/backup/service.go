package backup

import (
	"context"
	"path/filepath"
	"strconv"
	"time"
)

type Service struct {
	// Injected fields
	backupPeriod time.Duration
	dataDir      string
	outputDir    string
	logger       Logger
	keep         uint
	// configFilepath is the path to the configuration file, which is
	// backed up only if includeConfig is true.
	configFilepath string
	includeConfig  bool

	// Internal fields
	stopCh chan<- struct{}
	done   <-chan struct{}
}

func New(backupPeriod time.Duration,
	dataDir, outputDir string, logger Logger, keep uint,
	configFilepath string, includeConfig bool,
) *Service {
	return &Service{
		logger:         logger,
		backupPeriod:   backupPeriod,
		dataDir:        dataDir,
		outputDir:      outputDir,
		keep:           keep,
		configFilepath: configFilepath,
		includeConfig:  includeConfig,
	}
}

func (s *Service) String() string {
	return "backup"
}

const zipFileNamePrefix = "ddns-updater-backup-"

func makeZipFileName() string {
	return zipFileNamePrefix + strconv.Itoa(int(time.Now().UnixNano())) + ".zip"
}

func (s *Service) Start(ctx context.Context) (runError <-chan error, startErr error) {
	ready := make(chan struct{})
	runErrorCh := make(chan error)
	stopCh := make(chan struct{})
	s.stopCh = stopCh
	done := make(chan struct{})
	s.done = done
	go run(ready, runErrorCh, stopCh, done,
		s.outputDir, s.dataDir, s.backupPeriod, s.logger, s.keep,
		s.configFilepath, s.includeConfig)
	select {
	case <-ready:
	case <-ctx.Done():
		return nil, s.Stop()
	}
	return runErrorCh, nil
}

func run(ready chan<- struct{}, runError chan<- error, stopCh <-chan struct{},
	done chan<- struct{}, outputDir, dataDir string, backupPeriod time.Duration,
	logger Logger, keep uint, configFilepath string, includeConfig bool,
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
		files := []archiveFile{{path: filepath.Join(dataDir, "updates.json"), name: "updates.json"}}
		if includeConfig {
			files = append(files, archiveFile{path: configFilepath, name: "config.json"})
		}
		err := zipFiles(filepath.Join(outputDir, makeZipFileName()), files...)
		if err != nil {
			runError <- err
			return
		}
		err = pruneBackups(outputDir, keep)
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
