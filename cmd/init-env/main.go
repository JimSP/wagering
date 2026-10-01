// Command init-env generates local credentials from .env.example.
// Run it from the repository root; an optional argument selects the output file.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
)

var secretKeys = []string{
	"POSTGRES_PASSWORD", "POSTGRES_APP_PASSWORD", "KC_BOOTSTRAP_ADMIN_PASSWORD",
	"TEST_PROVIDER_A_CLIENT_SECRET", "TEST_PROVIDER_B_CLIENT_SECRET",
	"TEST_INTERNAL_CLIENT_SECRET", "TEST_EXPIRED_CLIENT_SECRET",
}

func generate(template, destination string) error {
	contents, err := os.ReadFile(template)
	if err != nil {
		return err
	}
	values := make(map[string]string)
	for _, key := range secretKeys {
		var random [32]byte
		if _, err := rand.Read(random[:]); err != nil {
			return err
		}
		values[key] = hex.EncodeToString(random[:])
	}
	values["DATABASE_URL"] = "postgres://wagering_app:" + values["POSTGRES_APP_PASSWORD"] + "@localhost:5432/wagering?sslmode=disable"
	lines := strings.Split(string(contents), "\n")
	for i, line := range lines {
		key, _, assignment := strings.Cut(line, "=")
		if value, ok := values[key]; ok && assignment {
			lines[i] = key + "=" + value
			delete(values, key)
		}
	}
	if len(values) != 0 {
		return errors.New("environment template is missing required credential fields")
	}
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := file.WriteString(strings.Join(lines, "\n"))
	closeErr := file.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return errors.Join(err, os.Remove(destination))
	}
	return nil
}

func main() {
	destination := ".env"
	if len(os.Args) > 2 {
		fmt.Fprintln(os.Stderr, "Usage: go run ./cmd/init-env [destination]")
		os.Exit(1)
	}
	if len(os.Args) == 2 {
		destination = os.Args[1]
	}
	if err := generate(".env.example", destination); err != nil {
		if errors.Is(err, os.ErrExist) {
			fmt.Fprintln(os.Stderr, "Environment file already exists; refusing to overwrite credentials.")
		} else {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
	fmt.Println("Private environment created (mode 0600). Values are not printed.")
}
