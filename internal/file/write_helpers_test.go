package file

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"metarr/internal/domain/logger"
)

// TestMain gives the global logger a console writer, since the zero value has none.
func TestMain(m *testing.M) {
	logger.Pl.Console = io.Discard
	os.Exit(m.Run())
}

// TestCopyFileCreatesDestDir tests that copyFile creates the destination's directory, not a directory at the file's path.
func TestCopyFileCreatesDestDir(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src.mp4")
	if err := os.WriteFile(src, []byte("video"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(tmp, "new", "dir", "dst.mp4")

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile: %v", err)
	}
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("destination is %v, want a regular file", info.Mode())
	}
}

// writeFile writes content to name in dir and returns its path.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestCopyFileKeepsExistingDest tests that a failed copy onto an existing file leaves that file untouched.
func TestCopyFileKeepsExistingDest(t *testing.T) {
	tmp := t.TempDir()
	src := writeFile(t, tmp, "src.mp4", "new")
	dst := writeFile(t, tmp, "dst.mp4", "existing")

	if err := copyFile(src, dst); err == nil {
		t.Fatal("copyFile onto an existing file succeeded, want an error")
	}
	if got, _ := os.ReadFile(dst); string(got) != "existing" {
		t.Fatalf("destination = %q, want it unchanged", got)
	}
}

// TestMoveOrCopyFileRefusesOverwrite tests that a move never replaces an existing destination.
func TestMoveOrCopyFileRefusesOverwrite(t *testing.T) {
	tmp := t.TempDir()
	src := writeFile(t, tmp, "src.mp4", "new")
	dst := writeFile(t, tmp, "dst.mp4", "existing")

	if err := moveOrCopyFile(src, dst); err == nil {
		t.Fatal("moveOrCopyFile onto an existing file succeeded, want an error")
	}
	if got, _ := os.ReadFile(dst); string(got) != "existing" {
		t.Fatalf("destination = %q, want it unchanged", got)
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("source removed: %v", err)
	}
}

// TestReplaceContents tests that the open handle reads the new contents after a replace.
func TestReplaceContents(t *testing.T) {
	p := writeFile(t, t.TempDir(), "meta.json", `{"title":"old"}`)
	f, err := os.OpenFile(p, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	if err := ReplaceContents(f, []byte(`{"title":"new"}`)); err != nil {
		t.Fatalf("ReplaceContents: %v", err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"title":"new"}` {
		t.Fatalf("handle reads %q, want the new contents", got)
	}
	if entries, _ := os.ReadDir(filepath.Dir(p)); len(entries) != 1 {
		t.Fatalf("directory holds %d entries, want only the replaced file", len(entries))
	}
}

// TestNextBackupPath tests that an identical backup is reused and a differing one gets a numbered sibling.
func TestNextBackupPath(t *testing.T) {
	tmp := t.TempDir()
	orig := writeFile(t, tmp, "video.mp4", "v1")

	first, err := nextBackupPath(orig)
	if err != nil || first == "" {
		t.Fatalf("nextBackupPath with no backups = %q, %v; want a path", first, err)
	}
	writeFile(t, tmp, filepath.Base(first), "v1")

	if p, err := nextBackupPath(orig); err != nil || p != "" {
		t.Fatalf("nextBackupPath with an identical backup = %q, %v; want \"\"", p, err)
	}

	if err := os.WriteFile(orig, []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := nextBackupPath(orig)
	if err != nil || second == "" || second == first {
		t.Fatalf("nextBackupPath with a differing backup = %q, %v; want a new numbered path", second, err)
	}
}
