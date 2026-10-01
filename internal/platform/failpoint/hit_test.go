package failpoint

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCrashMarkerAndExitContract(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FAILPOINT", "wanted")
	t.Setenv("FAILPOINT_DIR", dir)
	var codes []int
	exit := func(code int) { codes = append(codes, code) }
	hit("other", exit)
	if len(codes) != 0 {
		t.Fatal(codes)
	}
	if _, err := os.Stat(filepath.Join(dir, "other")); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	hit("wanted", exit)
	if len(codes) != 1 || codes[0] != 86 {
		t.Fatal(codes)
	}
	info, err := os.Stat(filepath.Join(dir, "wanted"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 || info.Size() != 0 {
		t.Fatal(info)
	}
	hit("wanted", exit)
	if len(codes) != 1 {
		t.Fatal("crashed twice", codes)
	}
	t.Setenv("FAILPOINT_DIR", "")
	hit("wanted", exit)
	if len(codes) != 1 {
		t.Fatal(codes)
	}
	t.Setenv("FAILPOINT_DIR", filepath.Join(dir, "missing"))
	hit("wanted", exit)
	if len(codes) != 1 {
		t.Fatal(codes)
	}
}

func TestRealCrashExit(t *testing.T) {
	if os.Getenv("WAGERING_FAILPOINT_CHILD") == "1" {
		Hit("crash")
		return
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	cmd := exec.Command(exe, "-test.run=^TestRealCrashExit$")
	cmd.Env = append(os.Environ(), "WAGERING_FAILPOINT_CHILD=1", "FAILPOINT=crash", "FAILPOINT_DIR="+dir)
	err = cmd.Run()
	exit := &exec.ExitError{}
	ok := errors.As(err, &exit)
	if !ok || exit.ExitCode() != 86 {
		t.Fatalf("exit=%v", err)
	}
	if _, err = os.Stat(filepath.Join(dir, "crash")); err != nil {
		t.Fatal(err)
	}
}
