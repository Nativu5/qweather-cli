package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

const (
	defaultSkillPath = "skills/qweather"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "qweather Skill tooling: %v\n", err)
		os.Exit(1)
	}
}

func run(arguments []string) error {
	if len(arguments) == 0 {
		return errors.New("expected generate or check")
	}
	switch arguments[0] {
	case "generate":
		flags := flag.NewFlagSet("generate", flag.ContinueOnError)
		root := flags.String("root", ".", "repository root")
		if err := flags.Parse(arguments[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return errors.New("generate accepts no positional arguments")
		}
		return writeGeneratedReferences(filepath.Clean(*root))
	case "check":
		flags := flag.NewFlagSet("check", flag.ContinueOnError)
		root := flags.String("root", ".", "repository root")
		if err := flags.Parse(arguments[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return errors.New("check accepts no positional arguments")
		}
		return checkSkill(filepath.Clean(*root))
	default:
		return fmt.Errorf("unknown subcommand %q; expected generate or check", arguments[0])
	}
}
