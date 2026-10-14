package backup

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const archiveCommentPrefix = "ddns-updater:v1:"

type archiveFile struct {
	filepath string
	name     string
}

func zipArchive(outputFilepath string, files []archiveFile) (err error) {
	outputDirectory := filepath.Dir(outputFilepath)
	f, err := os.CreateTemp(outputDirectory, ".ddns-updater-backup-*.tmp")
	if err != nil {
		return err
	}
	temporaryFilepath := f.Name()
	defer func() {
		if cleanupErr := os.Remove(temporaryFilepath); cleanupErr != nil &&
			!errors.Is(cleanupErr, os.ErrNotExist) && err == nil {
			err = cleanupErr
		}
	}()

	w := zip.NewWriter(f)
	archiveNames := make([]string, len(files))
	for i, file := range files {
		archiveNames[i] = file.name
	}
	sort.Strings(archiveNames)
	err = w.SetComment(archiveCommentPrefix + strings.Join(archiveNames, ","))
	if err != nil {
		return closeFailedArchive(w, f, fmt.Errorf("setting ZIP comment: %w", err))
	}
	for _, file := range files {
		err = addFile(w, file.filepath, file.name)
		if err != nil {
			return closeFailedArchive(w, f, err)
		}
	}
	err = w.Close()
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("closing ZIP writer: %w", err)
	}
	err = f.Sync()
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("syncing ZIP file: %w", err)
	}
	err = f.Close()
	if err != nil {
		return fmt.Errorf("closing ZIP file: %w", err)
	}
	err = os.Rename(temporaryFilepath, outputFilepath)
	if err != nil {
		return fmt.Errorf("renaming ZIP file: %w", err)
	}
	return nil
}

func closeFailedArchive(writer *zip.Writer, file *os.File, archiveErr error) error {
	zipCloseErr := writer.Close()
	if zipCloseErr != nil {
		zipCloseErr = fmt.Errorf("closing ZIP writer: %w", zipCloseErr)
	}
	fileCloseErr := file.Close()
	if fileCloseErr != nil {
		fileCloseErr = fmt.Errorf("closing ZIP file: %w", fileCloseErr)
	}
	return errors.Join(archiveErr, zipCloseErr, fileCloseErr)
}

func addFile(w *zip.Writer, filepath, archiveName string) (err error) {
	f, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer func() {
		closeErr := f.Close()
		if err == nil && closeErr != nil {
			err = fmt.Errorf("closing input file: %w", closeErr)
		}
	}()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = archiveName
	header.Method = zip.Deflate
	ioWriter, err := w.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(ioWriter, f)
	return err
}
