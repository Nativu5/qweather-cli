package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const requiredGoVersion = "go1.26.5"

type packOptions struct {
	SourceDir string
	OutputDir string
	Version   Version
	GoBinary  string
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(arguments []string, stdout, stderr io.Writer) int {
	if len(arguments) == 0 {
		_, _ = fmt.Fprintln(stderr, "usage: release <pack|verify|inspect> --version X.Y.Z [--source .] [--output dist]")
		return 2
	}
	command := arguments[0]
	if command != "pack" && command != "verify" && command != "inspect" {
		_, _ = fmt.Fprintf(stderr, "unknown release command %q\n", command)
		return 2
	}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(stderr)
	versionValue := flags.String("version", "", "stable SemVer without v prefix")
	sourceDir := flags.String("source", ".", "repository source directory")
	outputDir := flags.String("output", "dist", "release artifact output directory")
	goBinary := flags.String("go", "go", "Go toolchain binary")
	if err := flags.Parse(arguments[1:]); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "unexpected positional arguments")
		return 2
	}
	version, err := ParseStableVersion(*versionValue)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 2
	}
	options := packOptions{SourceDir: *sourceDir, OutputDir: *outputDir, Version: version, GoBinary: *goBinary}
	ctx := context.Background()
	if command == "inspect" {
		err = verifyArtifactDirectory(options.OutputDir, options.Version)
	} else if command == "verify" {
		err = verifyReproducible(ctx, options)
	} else {
		err = pack(ctx, options)
	}
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	if command == "inspect" {
		_, _ = fmt.Fprintf(stdout, "release artifacts for v%s verified in %s\n", version, options.OutputDir)
	} else {
		_, _ = fmt.Fprintf(stdout, "release artifacts for v%s written to %s\n", version, options.OutputDir)
	}
	return 0
}

func pack(ctx context.Context, options packOptions) error {
	artifacts, err := buildArtifacts(ctx, options)
	if err != nil {
		return err
	}
	return writeArtifacts(options.OutputDir, artifacts)
}

func verifyReproducible(ctx context.Context, options packOptions) error {
	first, err := buildArtifacts(ctx, options)
	if err != nil {
		return fmt.Errorf("first release build: %w", err)
	}
	second, err := buildArtifacts(ctx, options)
	if err != nil {
		return fmt.Errorf("second release build: %w", err)
	}
	if err := compareArtifacts(first, second); err != nil {
		return fmt.Errorf("release build is not reproducible: %w", err)
	}
	return writeArtifacts(options.OutputDir, first)
}

func buildArtifacts(ctx context.Context, options packOptions) (map[string][]byte, error) {
	sourceDir, err := filepath.Abs(options.SourceDir)
	if err != nil {
		return nil, fmt.Errorf("resolve source directory: %w", err)
	}
	versionFile, err := os.ReadFile(filepath.Join(sourceDir, "VERSION"))
	if err != nil {
		return nil, fmt.Errorf("read VERSION: %w", err)
	}
	if strings.TrimSpace(string(versionFile)) != options.Version.String() {
		return nil, fmt.Errorf("VERSION contains %q, want %s", strings.TrimSpace(string(versionFile)), options.Version)
	}
	status, err := gitOutput(ctx, sourceDir, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(status) != "" {
		return nil, fmt.Errorf("release source must be clean before packaging")
	}
	if err := requireGoVersion(ctx, options.GoBinary); err != nil {
		return nil, err
	}
	commit, timestamp, err := sourceMetadata(ctx, sourceDir)
	if err != nil {
		return nil, err
	}
	license, err := os.ReadFile(filepath.Join(sourceDir, "LICENSE"))
	if err != nil {
		return nil, fmt.Errorf("read LICENSE: %w", err)
	}
	readme, err := os.ReadFile(filepath.Join(sourceDir, "README.md"))
	if err != nil {
		return nil, fmt.Errorf("read README.md: %w", err)
	}
	temporaryDir, err := os.MkdirTemp("", "qweather-release-build-*")
	if err != nil {
		return nil, fmt.Errorf("create release build directory: %w", err)
	}
	defer os.RemoveAll(temporaryDir)

	binaries := make(map[string][]byte, len(releaseTargets))
	for _, target := range releaseTargets {
		binaryName := "qweather"
		if target.GOOS == "windows" {
			binaryName += ".exe"
		}
		path := filepath.Join(temporaryDir, target.GOOS+"-"+target.GOARCH+"-"+binaryName)
		if err := buildTarget(ctx, options.GoBinary, sourceDir, path, options.Version, commit, timestamp, target); err != nil {
			return nil, err
		}
		binary, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s/%s binary: %w", target.GOOS, target.GOARCH, err)
		}
		binaries[target.GOOS+"/"+target.GOARCH] = binary
	}
	return PackageArtifacts(options.Version, binaries, license, readme, timestamp)
}

func requireGoVersion(ctx context.Context, goBinary string) error {
	command := exec.CommandContext(ctx, goBinary, "env", "GOVERSION")
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("inspect Go version: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if got := strings.TrimSpace(string(output)); got != requiredGoVersion {
		return fmt.Errorf("release build requires %s, got %s", requiredGoVersion, got)
	}
	return nil
}

func sourceMetadata(ctx context.Context, sourceDir string) (string, time.Time, error) {
	commitOutput, err := gitOutput(ctx, sourceDir, "rev-parse", "HEAD")
	if err != nil {
		return "", time.Time{}, err
	}
	commit := strings.TrimSpace(commitOutput)
	if len(commit) != 40 {
		return "", time.Time{}, fmt.Errorf("source commit must be a full 40-character SHA, got %q", commit)
	}
	timestampOutput, err := gitOutput(ctx, sourceDir, "show", "-s", "--format=%ct", "HEAD")
	if err != nil {
		return "", time.Time{}, err
	}
	seconds, err := strconv.ParseInt(strings.TrimSpace(timestampOutput), 10, 64)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("parse source commit time: %w", err)
	}
	return commit, time.Unix(seconds, 0).UTC(), nil
}

func gitOutput(ctx context.Context, sourceDir string, arguments ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", append([]string{"-C", sourceDir}, arguments...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(arguments, " "), err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func buildTarget(ctx context.Context, goBinary, sourceDir, outputPath string, version Version, commit string, timestamp time.Time, target releaseTarget) error {
	linkerFlags := strings.Join([]string{
		"-s",
		"-w",
		"-buildid=",
		"-X", "github.com/Nativu5/qweather-cli/internal/buildinfo.Version=" + version.String(),
		"-X", "github.com/Nativu5/qweather-cli/internal/buildinfo.Commit=" + commit,
		"-X", "github.com/Nativu5/qweather-cli/internal/buildinfo.BuildTime=" + timestamp.UTC().Format(time.RFC3339),
	}, " ")
	command := exec.CommandContext(ctx, goBinary,
		"build",
		"-trimpath",
		"-buildvcs=false",
		"-ldflags", linkerFlags,
		"-o", outputPath,
		"./cmd/qweather",
	)
	command.Dir = sourceDir
	command.Env = environmentWith(os.Environ(), map[string]string{
		"CGO_ENABLED":       "0",
		"GOOS":              target.GOOS,
		"GOARCH":            target.GOARCH,
		"SOURCE_DATE_EPOCH": strconv.FormatInt(timestamp.Unix(), 10),
	})
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("build %s/%s: %w: %s", target.GOOS, target.GOARCH, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func environmentWith(current []string, overrides map[string]string) []string {
	result := make([]string, 0, len(current)+len(overrides))
	for _, entry := range current {
		name, _, found := strings.Cut(entry, "=")
		if found {
			if _, overridden := overrides[name]; overridden {
				continue
			}
		}
		result = append(result, entry)
	}
	keys := make([]string, 0, len(overrides))
	for key := range overrides {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		result = append(result, key+"="+overrides[key])
	}
	return result
}

func compareArtifacts(first, second map[string][]byte) error {
	if len(first) != len(second) {
		return fmt.Errorf("artifact counts differ: %d and %d", len(first), len(second))
	}
	for name, firstBytes := range first {
		secondBytes, ok := second[name]
		if !ok {
			return fmt.Errorf("second build is missing %s", name)
		}
		if !bytes.Equal(firstBytes, secondBytes) {
			return fmt.Errorf("artifact bytes differ for %s", name)
		}
	}
	return nil
}

func writeArtifacts(outputDir string, artifacts map[string][]byte) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create artifact output directory: %w", err)
	}
	written := make(map[string]bool, len(artifacts))
	for _, name := range sortedArtifactNames(artifacts) {
		data := artifacts[name]
		temporary, err := os.CreateTemp(outputDir, "."+name+"-*")
		if err != nil {
			return fmt.Errorf("create temporary artifact %s: %w", name, err)
		}
		temporaryName := temporary.Name()
		if _, err := temporary.Write(data); err != nil {
			temporary.Close()
			os.Remove(temporaryName)
			return fmt.Errorf("write artifact %s: %w", name, err)
		}
		if err := temporary.Sync(); err != nil {
			temporary.Close()
			os.Remove(temporaryName)
			return fmt.Errorf("sync artifact %s: %w", name, err)
		}
		if err := temporary.Close(); err != nil {
			os.Remove(temporaryName)
			return fmt.Errorf("close artifact %s: %w", name, err)
		}
		if err := os.Rename(temporaryName, filepath.Join(outputDir, name)); err != nil {
			os.Remove(temporaryName)
			return fmt.Errorf("install artifact %s: %w", name, err)
		}
		written[name] = true
	}
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return fmt.Errorf("inspect artifact output directory: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() && !written[entry.Name()] {
			return fmt.Errorf("output directory contains unexpected file %s", entry.Name())
		}
	}
	return nil
}

func verifyArtifactDirectory(outputDir string, version Version) error {
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return fmt.Errorf("read artifact directory: %w", err)
	}
	artifacts := make(map[string][]byte, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			return fmt.Errorf("artifact directory contains unexpected directory %s", entry.Name())
		}
		data, err := os.ReadFile(filepath.Join(outputDir, entry.Name()))
		if err != nil {
			return fmt.Errorf("read artifact %s: %w", entry.Name(), err)
		}
		artifacts[entry.Name()] = data
	}
	return VerifyArtifactSet(version, artifacts)
}

func sortedArtifactNames(artifacts map[string][]byte) []string {
	names := make([]string, 0, len(artifacts))
	for name := range artifacts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
