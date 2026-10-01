// Command reports generates test evidence and enforces coverage and mutation gates.
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func readJSON(path string, v any) error {
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	return json.Unmarshal(b, v)
}

func writeJSON(path string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: reports mutations FILE | coverage FILE | acceptance [--check-only] | requirements [--verbose] | command-status NORMAL RACE VET | targeted-mutations SOURCE GO OUTPUT")
	}
	switch args[0] {
	case "mutations":
		if len(args) != 2 {
			break
		}
		return mutationGate(args[1])
	case "coverage":
		if len(args) != 2 {
			break
		}
		return coverage(args[1])
	case "acceptance":
		if len(args) == 1 {
			return acceptance(false)
		}
		if len(args) == 2 && args[1] == "--check-only" {
			return acceptance(true)
		}
	case "requirements":
		if len(args) == 1 {
			return requirements(false)
		}
		if len(args) == 2 && args[1] == "--verbose" {
			return requirements(true)
		}
	case "command-status":
		if len(args) != 4 {
			break
		}
		return commandStatus(args[1:])
	case "targeted-mutations":
		if len(args) != 4 {
			break
		}
		return targetedMutations(args[1], args[2], args[3])
	}
	return fmt.Errorf("invalid report arguments: %v", args)
}

func main() {
	if e := run(os.Args[1:]); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
