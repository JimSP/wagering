package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvironmentMainHelper(t *testing.T) {
	if os.Getenv("ENV_MAIN_HELPER") != "1" {
		return
	}
	os.Args = append([]string{"init-env"}, strings.Split(os.Getenv("ENV_MAIN_ARGS"), "\x1f")...)
	if os.Getenv("ENV_MAIN_ARGS") == "" {
		os.Args = os.Args[:1]
	}
	main()
}
func TestEnvironmentCLIAndBareCredential(t *testing.T) {
	template, e := os.ReadFile("../../.env.example")
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		name, args            string
		missing, exists, fail bool
		want                  string
	}{
		{name: "default", want: "Private environment created"},
		{name: "explicit", args: "custom", want: "Private environment created"},
		{name: "usage", args: "a\x1fb", fail: true, want: "Usage:"},
		{name: "existing", exists: true, fail: true, want: "refusing to overwrite"},
		{name: "missing", missing: true, fail: true, want: ".env.example"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if !tc.missing {
				os.WriteFile(filepath.Join(dir, ".env.example"), append([]byte("POSTGRES_PASSWORD\nUNRELATED=value\n"), template...), 0600)
			}
			if tc.exists {
				os.WriteFile(filepath.Join(dir, ".env"), []byte("keep"), 0600)
			}
			cmd := exec.Command(os.Args[0], "-test.run=^TestEnvironmentMainHelper$")
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "ENV_MAIN_HELPER=1", "ENV_MAIN_ARGS="+tc.args)
			b, e := cmd.CombinedOutput()
			if (e != nil) != tc.fail || !strings.Contains(string(b), tc.want) {
				t.Fatalf("%v %s", e, b)
			}
			if !tc.fail {
				name := ".env"
				if tc.args != "" {
					name = tc.args
				}
				b, e = os.ReadFile(filepath.Join(dir, name))
				if e != nil || !strings.HasPrefix(string(b), "POSTGRES_PASSWORD\nUNRELATED=value\n") {
					t.Fatalf("bare key lost: %v %s", e, b)
				}
			}
			if tc.exists {
				b, _ = os.ReadFile(filepath.Join(dir, ".env"))
				if string(b) != "keep" {
					t.Fatal("overwrote existing")
				}
			}
		})
	}
}
