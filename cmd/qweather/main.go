package main

import (
	"context"
	"os"

	"github.com/Nativu5/qweather-cli/internal/app"
	"github.com/Nativu5/qweather-cli/internal/buildinfo"
	"github.com/Nativu5/qweather-cli/internal/catalog"
	"github.com/Nativu5/qweather-cli/internal/cli"
	"github.com/Nativu5/qweather-cli/internal/output"
)

func main() {
	os.Exit(run())
}

func run() int {
	registry, err := catalog.Default()
	if err != nil {
		problem := output.NewProblem(10, output.CodeRegistryInvalid, "compiled capability registry is invalid")
		problem.Cause = err
		_ = output.RenderProblem(os.Stderr, problem, output.ModeText)
		return problem.ExitCode
	}
	hash, err := registry.Hash()
	if err != nil {
		problem := output.NewProblem(10, output.CodeRegistryInvalid, "compiled capability registry cannot be hashed")
		problem.Cause = err
		_ = output.RenderProblem(os.Stderr, problem, output.ModeText)
		return problem.ExitCode
	}
	root, err := cli.NewRoot(registry, app.NewDefault(), buildinfo.Current(hash))
	if err != nil {
		problem := output.NewProblem(10, output.CodeCommandTreeInvalid, "command tree cannot be constructed")
		problem.Cause = err
		_ = output.RenderProblem(os.Stderr, problem, output.ModeText)
		return problem.ExitCode
	}
	return cli.Execute(context.Background(), root, os.Args[1:], os.Stdout, os.Stderr)
}
