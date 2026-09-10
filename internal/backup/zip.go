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

func zipFiles(outputFilepath string, inputFilepaths ...string) error {
	files := make([]archiveFile, len(inputFilepaths))
	for i, inputFilepath := range inputFilepaths {
		files[i] = archiveFile{filepath: inputFilepath, name: filepath.Base(inputFilepath)}
	}
	return zipArchive(outputFilepath, files)
}

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
	if err = w.SetComment(archiveCommentPrefix + strings.Join(archiveNames, ",")); err != nil {
		return closeFailedArchive(w, f, fmt.Errorf("set ZIP comment: %w", err))
	}
	for _, file := range files {
		err = addFile(w, file.filepath, file.name)
		if err != nil {
			return closeFailedArchive(w, f, err)
		}
	}
	if err = w.Close(); err != nil {
		_ = f.Close()
		return fmt.Errorf("close ZIP writer: %w", err)
	}
	if err = f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("sync ZIP file: %w", err)
	}
	if err = f.Close(); err != nil {
		return fmt.Errorf("close ZIP file: %w", err)
	}
	if err = os.Rename(temporaryFilepath, outputFilepath); err != nil {
		return fmt.Errorf("publish ZIP file: %w", err)
	}
	return nil
}

func closeFailedArchive(writer *zip.Writer, file *os.File, archiveErr error) error {
	zipCloseErr := writer.Close()
	if zipCloseErr != nil {
		zipCloseErr = fmt.Errorf("close ZIP writer: %w", zipCloseErr)
	}
	fileCloseErr := file.Close()
	if fileCloseErr != nil {
		fileCloseErr = fmt.Errorf("close ZIP file: %w", fileCloseErr)
	}
	return errors.Join(archiveErr, zipCloseErr, fileCloseErr)
}

func addFile(w *zip.Writer, filepath, archiveName string) (err error) {
	f, err := os.Open(filepath)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		_ = f.Close()
		return err
	}
	header.Name = archiveName
	header.Method = zip.Deflate
	ioWriter, err := w.CreateHeader(header)
	if err != nil {
		_ = f.Close()
		return err
	}
	_, copyErr := io.Copy(ioWriter, f)
	closeErr := f.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return fmt.Errorf("close input file: %w", closeErr)
	}
	return nil
}
