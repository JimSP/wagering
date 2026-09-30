package main

import (
	"bytes"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratePrivateCredentials(t *testing.T) {
	directory := t.TempDir()
	first, second := filepath.Join(directory, "first"), filepath.Join(directory, "second")
	for _, destination := range []string{first, second} {
		if err := generate("../../.env.example", destination); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	other, err := os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}
	values := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok {
			values[key] = value
		}
	}
	seen := make(map[string]bool)
	for _, key := range secretKeys {
		value := values[key]
		decoded, err := hex.DecodeString(value)
		if err != nil || len(decoded) != 32 || seen[value] || bytes.Contains(other, []byte(value)) {
			t.Fatalf("invalid or reused credential for %s", key)
		}
		seen[value] = true
	}
	if !strings.Contains(values["DATABASE_URL"], ":"+values["POSTGRES_APP_PASSWORD"]+"@") {
		t.Fatal("database URL does not use the generated application password")
	}
	info, err := os.Stat(first)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("unexpected permissions: %v", info.Mode().Perm())
	}
	if err := generate("../../.env.example", first); !errors.Is(err, os.ErrExist) {
		t.Fatal("existing file was not protected")
	}
	after, err := os.ReadFile(first)
	if err != nil || !bytes.Equal(data, after) {
		t.Fatal("existing environment file changed")
	}
}

func TestExampleContainsNoCredentials(t *testing.T) {
	data, err := os.ReadFile("../../.env.example")
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range append(append([]string{}, secretKeys...), "DATABASE_URL") {
		if !strings.Contains(string(data), "\n"+key+"=\n") {
			t.Fatalf("example must contain an empty %s", key)
		}
	}
}

func TestIncompleteTemplateDoesNotCreateFile(t *testing.T) {
	directory := t.TempDir()
	template, destination := filepath.Join(directory, "template"), filepath.Join(directory, "env")
	if err := os.WriteFile(template, []byte("POSTGRES_PASSWORD=\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := generate(template, destination); err == nil {
		t.Fatal("incomplete template accepted")
	}
	if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("invalid configuration created an output file")
	}
}
