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
	pinnedCommit     = "02bb257a032c503c65924005da6ebca48d94b390"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "qweather Skill tooling: %v\n", err)
		os.Exit(1)
	}
}

func run(arguments []string) error {
	if len(arguments) == 0 {
		return errors.New("expected generate, check, or sync-openapi")
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
	case "sync-openapi":
		flags := flag.NewFlagSet("sync-openapi", flag.ContinueOnError)
		root := flags.String("root", ".", "repository root")
		source := flags.String("source", ".cache/qweather-dev-site-source", "official qwd/dev-site checkout")
		commit := flags.String("commit", pinnedCommit, "full reviewed upstream commit")
		if err := flags.Parse(arguments[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return errors.New("sync-openapi accepts no positional arguments")
		}
		return syncOpenAPI(filepath.Clean(*root), filepath.Clean(*source), *commit)
	default:
		return fmt.Errorf("unknown subcommand %q; expected generate, check, or sync-openapi", arguments[0])
	}
}
