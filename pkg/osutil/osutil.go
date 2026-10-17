package osutil

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"runtime"
	"syscall"

	junction "github.com/nyaosorg/go-windows-junction"
)

func CaseInsensitive() bool {
	return runtime.GOOS == "windows"
}

func RemoveIfExist(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func RemoveDirIfEmpty(path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if len(entries) == 0 {
		return os.Remove(path)
	}
	return nil
}

func WriteFileIfNotEqual(path string, content []byte, perm os.FileMode) error {
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(content, old) {
		return nil
	}
	return os.WriteFile(path, content, perm)
}

func WriteFileIfNotExist(path string, content []byte, perm os.FileMode) error {
	_, err := os.Stat(path)
	if err == nil {
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.WriteFile(path, content, perm)
}

func CopyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = srcFile.Close() }()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = dstFile.Close() }()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func MoveFile(src, dst string) error {
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}
	if !errors.Is(err, syscall.EXDEV) {
		return err
	}
	if err := CopyFile(src, dst); err != nil {
		return err
	}
	return os.Remove(src)
}

func Symlink(oldpath, newpath string) error {
	oldStat, err := os.Stat(oldpath)
	if err != nil {
		return err
	}

	if newInfo, err := os.Lstat(newpath); err == nil {
		same := false
		isLink := newInfo.Mode()&os.ModeSymlink != 0
		if isLink {
			same = os.SameFile(newInfo, oldStat)
		} else if statInfo, err := os.Stat(newpath); err == nil {
			same = os.SameFile(statInfo, oldStat)
		}
		if same {
			return nil
		}
		if isLink {
			if err := os.Remove(newpath); err != nil {
				return err
			}
		} else {
			return fs.ErrExist
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	return junction.Create(oldpath, newpath)
}
