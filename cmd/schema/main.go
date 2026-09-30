// Command schema manages immutable SQL migrations and the pinned Docker migrator.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	filename = regexp.MustCompile(`^(\d{6})_([a-z][a-z0-9_]*)\.(up|down)\.sql$`)
	label    = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
)

type history struct {
	files  map[string][]byte
	hashes map[string]string
	latest int
}

func inspect(dir string, allowNew bool) (history, error) {
	h := history{files: map[string][]byte{}, hashes: map[string]string{}}
	data, err := os.ReadFile(filepath.Join(dir, "checksums.json"))
	if err != nil {
		return h, err
	}
	if err = json.Unmarshal(data, &h.hashes); err != nil {
		return h, err
	}
	if len(h.hashes) == 0 {
		return h, fmt.Errorf("empty locked history")
	}
	manifest, err := os.ReadFile(filepath.Join(dir, "checksums.sha256"))
	if err != nil {
		return h, err
	}
	if string(manifest) != checksumManifest(h.hashes) {
		return h, fmt.Errorf("checksums.json and checksums.sha256 disagree")
	}
	oldMax := 0
	for name, hash := range h.hashes {
		match := filename.FindStringSubmatch(name)
		if match == nil {
			return h, fmt.Errorf("invalid locked migration: %s", name)
		}
		version, _ := strconv.Atoi(match[1])
		if version > oldMax {
			oldMax = version
		}
		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || digest(content) != hash {
			return h, fmt.Errorf("immutable migration changed or removed: %s", name)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return h, err
	}
	pairs := map[int][]string{}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}
		match := filename.FindStringSubmatch(name)
		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return h, err
		}
		if match == nil || len(strings.TrimSpace(string(content))) == 0 {
			return h, fmt.Errorf("invalid/empty migration: %s", name)
		}
		version, _ := strconv.Atoi(match[1])
		if version > h.latest {
			h.latest = version
		}
		if _, ok := h.hashes[name]; !ok {
			if !allowNew {
				return h, fmt.Errorf("unregistered migration: %s; use schema-seal after review", name)
			}
			if version <= oldMax {
				return h, fmt.Errorf("new migrations must follow the locked history")
			}
		}
		pairs[version] = append(pairs[version], match[2]+"."+match[3])
		h.files[name] = content
	}
	for version, members := range pairs {
		sort.Strings(members)
		if len(members) != 2 || strings.TrimSuffix(members[0], ".down") != strings.TrimSuffix(members[1], ".up") || !strings.HasSuffix(members[0], ".down") || !strings.HasSuffix(members[1], ".up") {
			return h, fmt.Errorf("version %d requires one matching up/down pair", version)
		}
	}
	if len(pairs) != h.latest || pairs[0] != nil {
		return h, fmt.Errorf("migration versions must be contiguous from 1")
	}
	return h, nil
}
func digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func checksumManifest(hashes map[string]string) string {
	names := make([]string, 0, len(hashes))
	for name := range hashes {
		names = append(names, name)
	}
	sort.Strings(names)
	var out strings.Builder
	for _, name := range names {
		fmt.Fprintf(&out, "%s  %s\n", hashes[name], name)
	}
	return out.String()
}

type executor func(root string, args ...string) error

func run(root string, args []string, out io.Writer, execute executor) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: schema check|new NAME|seal|status|up|down STEPS|dump")
	}
	command := args[0]
	switch command {
	case "new", "down":
		if len(args) != 2 {
			return fmt.Errorf("%s requires one argument", command)
		}
	case "check", "seal", "status", "up", "dump":
		if len(args) != 1 {
			return fmt.Errorf("%s does not accept arguments", command)
		}
	default:
		return fmt.Errorf("unknown schema command: %s", command)
	}
	if command == "down" {
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 1 || !regexp.MustCompile(`^[0-9]+$`).MatchString(args[1]) {
			return fmt.Errorf("down requires an explicit positive step count")
		}
	}
	if command == "new" && !label.MatchString(args[1]) {
		return fmt.Errorf("supply a snake_case migration name")
	}
	dir := filepath.Join(root, "migrations")
	h, err := inspect(dir, command == "new" || command == "seal")
	if err != nil {
		return err
	}
	switch command {
	case "check":
		_, err = fmt.Fprintf(out, "%d versions; paired up/down files; SHA-256 verified\n", h.latest)
	case "new":
		if h.latest >= 999999 {
			return fmt.Errorf("migration version limit reached")
		}
		created := []string{}
		for _, direction := range []string{"up", "down"} {
			name := fmt.Sprintf("%06d_%s.%s.sql", h.latest+1, args[1], direction)
			path := filepath.Join(dir, name)
			f, openErr := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
			if openErr != nil {
				for _, p := range created {
					_ = os.Remove(p)
				}
				return openErr
			}
			created = append(created, path)
			_, writeErr := io.WriteString(f, "BEGIN;\n-- Implement and test before schema-seal.\nCOMMIT;\n")
			closeErr := f.Close()
			if writeErr != nil || closeErr != nil {
				for _, p := range created {
					_ = os.Remove(p)
				}
				if writeErr != nil {
					return writeErr
				}
				return closeErr
			}
			if _, err = fmt.Fprintln(out, filepath.Join("migrations", name)); err != nil {
				return err
			}
		}
	case "seal":
		hashes := map[string]string{}
		for name, content := range h.files {
			if strings.Contains(string(content), "Implement and test before schema-seal.") {
				return fmt.Errorf("unfinished migration: %s", name)
			}
			hashes[name] = digest(content)
		}
		data, marshalErr := json.MarshalIndent(hashes, "", "  ")
		if marshalErr != nil {
			return marshalErr
		}
		// If interrupted between replacements, check refuses the inconsistent manifests.
		if err = replace(filepath.Join(dir, "checksums.json"), append(data, '\n')); err != nil {
			return err
		}
		if err = replace(filepath.Join(dir, "checksums.sha256"), []byte(checksumManifest(hashes))); err != nil {
			return err
		}
		_, err = fmt.Fprintln(out, "New migration hashes registered; previous hashes preserved")
	case "dump":
		err = execute(root, "compose", "exec", "-T", "postgres", "pg_dump", "-U", "wagering", "-d", "wagering", "--schema-only", "--no-owner", "--no-privileges")
	default:
		if command == "status" {
			command = "version"
		}
		dockerArgs := []string{"compose", "run", "--rm", "migrate", command}
		if command == "down" {
			dockerArgs = append(dockerArgs, args[1])
		}
		err = execute(root, dockerArgs...)
	}
	return err
}

func replace(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".checksum-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(f.Name()) }()
	if err = f.Chmod(0o644); err != nil {
		_ = f.Close()
		return err
	}
	if _, err = f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func main() {
	root, err := os.Getwd()
	if err == nil {
		err = run(root, os.Args[1:], os.Stdout, func(root string, args ...string) error {
			cmd := exec.Command("docker", args...)
			cmd.Dir = root
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()
		})
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
