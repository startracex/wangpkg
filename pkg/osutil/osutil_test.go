package osutil

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	return string(b)
}

func TestCaseInsensitive(t *testing.T) {
	if got := CaseInsensitive(); got != (runtime.GOOS == "windows") {
		t.Errorf("CaseInsensitive() = %v, want %v", got, runtime.GOOS == "windows")
	}
}

func TestRemoveIfExist(t *testing.T) {
	dir := t.TempDir()

	t.Run("removes an existing file", func(t *testing.T) {
		path := filepath.Join(dir, "a.txt")
		writeFile(t, path, "a")
		if err := RemoveIfExist(path); err != nil {
			t.Fatalf("RemoveIfExist: %v", err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("file should be gone, stat err = %v", err)
		}
	})

	t.Run("missing file is not an error", func(t *testing.T) {
		path := filepath.Join(dir, "missing.txt")
		if err := RemoveIfExist(path); err != nil {
			t.Errorf("RemoveIfExist on a missing file = %v, want nil", err)
		}
	})

	t.Run("empty directory is removed", func(t *testing.T) {
		path := filepath.Join(dir, "empty")
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := RemoveIfExist(path); err != nil {
			t.Fatalf("RemoveIfExist: %v", err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("directory should be gone, stat err = %v", err)
		}
	})

	t.Run("non empty directory reports an error", func(t *testing.T) {
		path := filepath.Join(dir, "full")
		writeFile(t, filepath.Join(path, "inner.txt"), "x")
		if err := RemoveIfExist(path); err == nil {
			t.Error("expected an error when removing a non-empty directory")
		}
	})
}

func TestRemoveDirIfEmpty(t *testing.T) {
	dir := t.TempDir()

	t.Run("removes an empty directory", func(t *testing.T) {
		path := filepath.Join(dir, "empty")
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := RemoveDirIfEmpty(path); err != nil {
			t.Fatalf("RemoveDirIfEmpty: %v", err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("directory should be gone, stat err = %v", err)
		}
	})

	t.Run("keeps a non empty directory", func(t *testing.T) {
		path := filepath.Join(dir, "full")
		writeFile(t, filepath.Join(path, "inner.txt"), "x")
		if err := RemoveDirIfEmpty(path); err != nil {
			t.Fatalf("RemoveDirIfEmpty: %v", err)
		}
		if _, err := os.Stat(path); err != nil {
			t.Errorf("directory should survive, stat err = %v", err)
		}
	})

	t.Run("missing directory is not an error", func(t *testing.T) {
		if err := RemoveDirIfEmpty(filepath.Join(dir, "missing")); err != nil {
			t.Errorf("RemoveDirIfEmpty on a missing path = %v, want nil", err)
		}
	})
}

func TestWriteFileIfNotEqual(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")

	t.Run("creates the file when it does not exist", func(t *testing.T) {
		if err := WriteFileIfNotEqual(path, []byte("hello"), 0o644); err != nil {
			t.Fatalf("WriteFileIfNotEqual: %v", err)
		}
		if got := readFile(t, path); got != "hello" {
			t.Errorf("content = %q", got)
		}
	})

	t.Run("different content is overwritten", func(t *testing.T) {
		if err := WriteFileIfNotEqual(path, []byte("world"), 0o644); err != nil {
			t.Fatalf("WriteFileIfNotEqual: %v", err)
		}
		if got := readFile(t, path); got != "world" {
			t.Errorf("content = %q, want world", got)
		}
	})

	t.Run("empty content is written", func(t *testing.T) {
		empty := filepath.Join(dir, "empty.txt")
		if err := WriteFileIfNotEqual(empty, nil, 0o644); err != nil {
			t.Fatalf("WriteFileIfNotEqual: %v", err)
		}
		if got := readFile(t, empty); got != "" {
			t.Errorf("content = %q, want empty", got)
		}
	})
}

func TestWriteFileIfNotExist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")

	t.Run("creates a missing file", func(t *testing.T) {
		if err := WriteFileIfNotExist(path, []byte("hello"), 0o600); err != nil {
			t.Fatalf("WriteFileIfNotExist: %v", err)
		}
		if got := readFile(t, path); got != "hello" {
			t.Errorf("content = %q", got)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat: %v", err)
		}
		if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
			t.Errorf("perm = %v, want 0600", info.Mode().Perm())
		}
	})
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()

	src := filepath.Join(dir, "src.txt")
	writeFile(t, src, "payload")

	t.Run("copies the content", func(t *testing.T) {
		dst := filepath.Join(dir, "dst.txt")
		if err := CopyFile(src, dst); err != nil {
			t.Fatalf("CopyFile: %v", err)
		}
		if got := readFile(t, dst); got != "payload" {
			t.Errorf("dst content = %q", got)
		}
		if got := readFile(t, src); got != "payload" {
			t.Errorf("src content changed: %q", got)
		}
	})

	t.Run("overwrites the destination", func(t *testing.T) {
		dst := filepath.Join(dir, "dst2.txt")
		writeFile(t, dst, "old")
		if err := CopyFile(src, dst); err != nil {
			t.Fatalf("CopyFile: %v", err)
		}
		if got := readFile(t, dst); got != "payload" {
			t.Errorf("dst content = %q", got)
		}
	})

	t.Run("copies an empty file", func(t *testing.T) {
		empty := filepath.Join(dir, "empty.txt")
		writeFile(t, empty, "")
		dst := filepath.Join(dir, "empty-copy.txt")
		if err := CopyFile(empty, dst); err != nil {
			t.Fatalf("CopyFile: %v", err)
		}
		info, err := os.Stat(dst)
		if err != nil {
			t.Fatalf("Stat: %v", err)
		}
		if info.Size() != 0 {
			t.Errorf("size = %d, want 0", info.Size())
		}
	})

	t.Run("missing source is an error", func(t *testing.T) {
		if err := CopyFile(filepath.Join(dir, "missing.txt"), filepath.Join(dir, "out.txt")); err == nil {
			t.Error("expected an error for a missing source")
		}
	})

	t.Run("missing destination directory is an error", func(t *testing.T) {
		if err := CopyFile(src, filepath.Join(dir, "no", "such", "dir", "out.txt")); err == nil {
			t.Error("expected an error when the destination directory does not exist")
		}
	})
}

func TestMoveFile(t *testing.T) {
	dir := t.TempDir()

	t.Run("moves within the same directory", func(t *testing.T) {
		src := filepath.Join(dir, "a.txt")
		dst := filepath.Join(dir, "b.txt")
		writeFile(t, src, "payload")

		if err := MoveFile(src, dst); err != nil {
			t.Fatalf("MoveFile: %v", err)
		}
		if _, err := os.Stat(src); !os.IsNotExist(err) {
			t.Errorf("source should be gone, stat err = %v", err)
		}
		if got := readFile(t, dst); got != "payload" {
			t.Errorf("dst content = %q", got)
		}
	})

	t.Run("overwrites the destination", func(t *testing.T) {
		src := filepath.Join(dir, "c.txt")
		dst := filepath.Join(dir, "d.txt")
		writeFile(t, src, "new")
		writeFile(t, dst, "old")

		if err := MoveFile(src, dst); err != nil {
			t.Fatalf("MoveFile: %v", err)
		}
		if got := readFile(t, dst); got != "new" {
			t.Errorf("dst content = %q, want new", got)
		}
	})

	t.Run("missing source is an error", func(t *testing.T) {
		if err := MoveFile(filepath.Join(dir, "missing.txt"), filepath.Join(dir, "out.txt")); err == nil {
			t.Error("expected an error for a missing source")
		}
	})
}

func TestSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeFile(t, filepath.Join(target, "inner.txt"), "x")

	link := filepath.Join(dir, "link")
	if err := Symlink(target, link); err != nil {
		t.Skipf("symlinks/junctions are not available: %v", err)
	}

	info, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("Lstat(%s): %v", link, err)
	}

	if info.Mode()&os.ModeSymlink == 0 && info.Mode()&fs.ModeIrregular == 0 {
		t.Errorf("mode = %v, want a reparse point (symlink or junction)", info.Mode())
	}
	if info.IsDir() {
		t.Errorf("mode = %v, want a link entry rather than a directory", info.Mode())
	}

	if _, err := os.Stat(filepath.Join(link, "inner.txt")); err != nil {
		t.Errorf("cannot read through the link: %v", err)
	}

	if err := Symlink(target, link); err != nil {
		t.Errorf("re-creating the same link should succeed, got %v", err)
	}
}
