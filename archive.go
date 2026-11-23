package main

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
)

// Extract into the private install directory through an OS-enforced root.
func unpack(archive, destination string) error {
	root, err := os.OpenRoot(destination)
	if err != nil {
		return err
	}
	defer root.Close()
	var total int64
	files := 0
	write := func(name string, mode fs.FileMode, source io.Reader) error {
		name = strings.TrimPrefix(name, "./")
		if !fs.ValidPath(name) || strings.Contains(name, "\\") || !mode.IsRegular() {
			return fmt.Errorf("invalid archive entry %q", name)
		}
		if !strings.HasPrefix(name, "extension/") {
			return nil
		}
		files++
		if files > 16384 {
			return fmt.Errorf("archive has too many files")
		}
		if err := root.MkdirAll(path.Dir(name), 0o755); err != nil {
			return err
		}
		file, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode.Perm()|0o600)
		if err != nil {
			return err
		}
		n, err := io.Copy(file, io.LimitReader(source, (256<<20)-total+1))
		total += n
		closeErr := file.Close()
		if err != nil {
			return err
		}
		if total > 256<<20 {
			return fmt.Errorf("expanded archive exceeds size limit")
		}
		return closeErr
	}
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer reader.Close()
	if len(reader.File) > 32768 {
		return fmt.Errorf("archive has too many entries")
	}
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		source, err := file.Open()
		if err != nil {
			return err
		}
		err = write(file.Name, file.Mode(), source)
		source.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
