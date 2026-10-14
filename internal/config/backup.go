package config

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/qdm12/gosettings"
	"github.com/qdm12/gosettings/reader"
	"github.com/qdm12/gotree"
)

var errBackupKeepNegative = errors.New("backup keep count must be zero or positive")

type Backup struct {
	Period        *time.Duration
	Directory     *string
	IncludeConfig *bool
	Keep          *int
}

func (b *Backup) setDefaults(includeConfig bool) {
	b.Period = gosettings.DefaultPointer(b.Period, 0)
	b.Directory = gosettings.DefaultPointer(b.Directory, "./data")
	b.IncludeConfig = gosettings.DefaultPointer(b.IncludeConfig, includeConfig)
	b.Keep = gosettings.DefaultPointer(b.Keep, 0)
}

func (b Backup) Validate() (err error) {
	if *b.Keep < 0 {
		return fmt.Errorf("%w: %d", errBackupKeepNegative, *b.Keep)
	}
	return nil
}

func (b Backup) String() string {
	return b.toLinesNode().String()
}

func (b Backup) toLinesNode() *gotree.Node {
	if *b.Period == 0 {
		return gotree.New("Backup: disabled")
	}
	node := gotree.New("Backup")
	node.Appendf("Period: %s", b.Period)
	node.Appendf("Directory: %s", *b.Directory)
	node.Appendf("Include config: %t", *b.IncludeConfig)
	if *b.Keep == 0 {
		node.Appendf("Retention: unlimited")
	} else {
		node.Appendf("Retention: %d", *b.Keep)
	}
	return node
}

func (b *Backup) read(reader *reader.Reader) (err error) {
	b.Period, err = reader.DurationPtr("BACKUP_PERIOD")
	if err != nil {
		return err
	}

	b.Directory = reader.Get("BACKUP_DIRECTORY")
	b.IncludeConfig, err = reader.BoolPtr("BACKUP_INCLUDE_CONFIG")
	if err != nil {
		return fmt.Errorf("parse BACKUP_INCLUDE_CONFIG: %w", err)
	}
	keepString := reader.String("BACKUP_KEEP")
	if keepString != "" {
		keep, parseErr := strconv.Atoi(keepString)
		if parseErr != nil {
			return fmt.Errorf("parse BACKUP_KEEP: %w", parseErr)
		}
		b.Keep = &keep
	}
	return nil
}
