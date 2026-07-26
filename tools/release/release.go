package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Version is the stable release version accepted by the release contract.
type Version struct {
	Major int
	Minor int
	Patch int
}

func ParseStableVersion(value string) (Version, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("version %q must be stable SemVer X.Y.Z", value)
	}
	values := [3]int{}
	for index, part := range parts {
		if part == "" || (len(part) > 1 && part[0] == '0') {
			return Version{}, fmt.Errorf("version %q contains an invalid numeric component", value)
		}
		for _, character := range part {
			if character < '0' || character > '9' {
				return Version{}, fmt.Errorf("version %q must contain numeric components only", value)
			}
		}
		parsed, err := strconv.Atoi(part)
		if err != nil {
			return Version{}, fmt.Errorf("version %q contains an out-of-range component: %w", value, err)
		}
		values[index] = parsed
	}
	return Version{Major: values[0], Minor: values[1], Patch: values[2]}, nil
}

func (version Version) String() string {
	return fmt.Sprintf("%d.%d.%d", version.Major, version.Minor, version.Patch)
}

type releaseTarget struct {
	GOOS   string
	GOARCH string
	Format string
}

var releaseTargets = []releaseTarget{
	{GOOS: "darwin", GOARCH: "arm64", Format: "tar.gz"},
	{GOOS: "darwin", GOARCH: "amd64", Format: "tar.gz"},
	{GOOS: "linux", GOARCH: "arm64", Format: "tar.gz"},
	{GOOS: "linux", GOARCH: "amd64", Format: "tar.gz"},
	{GOOS: "windows", GOARCH: "arm64", Format: "zip"},
	{GOOS: "windows", GOARCH: "amd64", Format: "zip"},
}

func ReleaseAssetNames(version Version) []string {
	names := make([]string, 0, len(releaseTargets)+1)
	for _, target := range releaseTargets {
		names = append(names, fmt.Sprintf("qweather-cli_%s_%s_%s.%s", version, target.GOOS, target.GOARCH, target.Format))
	}
	return append(names, "checksums.txt")
}

func BuildChecksumManifest(archives map[string][]byte) ([]byte, error) {
	if len(archives) == 0 {
		return nil, fmt.Errorf("at least one archive is required")
	}
	names := make([]string, 0, len(archives))
	for name := range archives {
		if name == "" || strings.ContainsAny(name, "/\\\r\n") || name == "checksums.txt" {
			return nil, fmt.Errorf("invalid archive filename %q", name)
		}
		names = append(names, name)
	}
	sort.Strings(names)
	var output bytes.Buffer
	for _, name := range names {
		digest := sha256.Sum256(archives[name])
		output.WriteString(hex.EncodeToString(digest[:]))
		output.WriteString("  ")
		output.WriteString(name)
		output.WriteByte('\n')
	}
	return output.Bytes(), nil
}

func PackageArtifacts(version Version, binaries map[string][]byte, license, readme []byte, timestamp time.Time) (map[string][]byte, error) {
	if len(binaries) != len(releaseTargets) {
		return nil, fmt.Errorf("expected %d target binaries, got %d", len(releaseTargets), len(binaries))
	}
	archives := make(map[string][]byte, len(releaseTargets))
	for _, target := range releaseTargets {
		key := target.GOOS + "/" + target.GOARCH
		binary, ok := binaries[key]
		if !ok || len(binary) == 0 {
			return nil, fmt.Errorf("missing target binary %s", key)
		}
		binaryName := "qweather"
		if target.GOOS == "windows" {
			binaryName += ".exe"
		}
		archive, err := BuildArchive(version, target, map[string][]byte{
			binaryName:  binary,
			"LICENSE":   license,
			"README.md": readme,
		}, timestamp)
		if err != nil {
			return nil, fmt.Errorf("package %s: %w", key, err)
		}
		name := fmt.Sprintf("qweather-cli_%s_%s_%s.%s", version, target.GOOS, target.GOARCH, target.Format)
		archives[name] = archive
	}
	manifest, err := BuildChecksumManifest(archives)
	if err != nil {
		return nil, err
	}
	artifacts := make(map[string][]byte, len(archives)+1)
	for name, archive := range archives {
		artifacts[name] = archive
	}
	artifacts["checksums.txt"] = manifest
	return artifacts, nil
}

func VerifyArtifactSet(version Version, artifacts map[string][]byte) error {
	if len(artifacts) != len(ReleaseAssetNames(version)) {
		return fmt.Errorf("artifact set must contain six archives and checksums.txt")
	}
	archives := make(map[string][]byte, len(artifacts)-1)
	for _, name := range ReleaseAssetNames(version) {
		data, ok := artifacts[name]
		if !ok || len(data) == 0 {
			return fmt.Errorf("artifact set is missing %s", name)
		}
		if name != "checksums.txt" {
			if len(data) > 64*1024*1024 {
				return fmt.Errorf("archive %s exceeds the 64 MiB download limit", name)
			}
			archives[name] = data
		}
	}
	manifest, err := BuildChecksumManifest(archives)
	if err != nil {
		return err
	}
	if !bytes.Equal(manifest, artifacts["checksums.txt"]) {
		return fmt.Errorf("checksums.txt does not match archive bytes")
	}
	for _, target := range releaseTargets {
		name := fmt.Sprintf("qweather-cli_%s_%s_%s.%s", version, target.GOOS, target.GOARCH, target.Format)
		binary := "qweather"
		if target.GOOS == "windows" {
			binary = "qweather.exe"
		}
		if err := verifyArchiveBytes(artifacts[name], target.Format, fmt.Sprintf("qweather-cli_%s_%s_%s", version, target.GOOS, target.GOARCH), binary); err != nil {
			return fmt.Errorf("verify %s: %w", name, err)
		}
	}
	return nil
}

func verifyArchiveBytes(data []byte, format, root, binary string) error {
	expected := map[string]bool{
		root + "/" + binary: false,
		root + "/LICENSE":   false,
		root + "/README.md": false,
	}
	if format == "tar.gz" {
		gzipReader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("open gzip: %w", err)
		}
		tarReader := tar.NewReader(gzipReader)
		for {
			header, err := tarReader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return fmt.Errorf("read tar: %w", err)
			}
			if header.Typeflag != tar.TypeReg {
				return fmt.Errorf("entry %s is not a regular file", header.Name)
			}
			if _, ok := expected[header.Name]; !ok || expected[header.Name] {
				return fmt.Errorf("unexpected or duplicate entry %s", header.Name)
			}
			if header.Size < 0 || header.Size > 128*1024*1024 {
				return fmt.Errorf("entry %s exceeds extracted size limit", header.Name)
			}
			if _, err := io.Copy(io.Discard, io.LimitReader(tarReader, header.Size)); err != nil {
				return fmt.Errorf("read entry %s: %w", header.Name, err)
			}
			expected[header.Name] = true
		}
		if err := gzipReader.Close(); err != nil {
			return fmt.Errorf("close gzip: %w", err)
		}
	} else if format == "zip" {
		archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return fmt.Errorf("open zip: %w", err)
		}
		for _, file := range archive.File {
			if !file.Mode().IsRegular() {
				return fmt.Errorf("entry %s is not a regular file", file.Name)
			}
			if _, ok := expected[file.Name]; !ok || expected[file.Name] {
				return fmt.Errorf("unexpected or duplicate entry %s", file.Name)
			}
			if file.UncompressedSize64 > 128*1024*1024 {
				return fmt.Errorf("entry %s exceeds extracted size limit", file.Name)
			}
			reader, err := file.Open()
			if err != nil {
				return fmt.Errorf("open entry %s: %w", file.Name, err)
			}
			if _, err := io.Copy(io.Discard, io.LimitReader(reader, int64(file.UncompressedSize64))); err != nil {
				reader.Close()
				return fmt.Errorf("read entry %s: %w", file.Name, err)
			}
			if err := reader.Close(); err != nil {
				return fmt.Errorf("close entry %s: %w", file.Name, err)
			}
			expected[file.Name] = true
		}
	} else {
		return fmt.Errorf("unsupported archive format %s", format)
	}
	for name, seen := range expected {
		if !seen {
			return fmt.Errorf("missing entry %s", name)
		}
	}
	return nil
}

// BuildArchive creates one deterministic release archive from its approved
// three-file payload. The caller owns the output bytes and may write them to a
// file or hash them without any further filesystem access.
func BuildArchive(version Version, target releaseTarget, files map[string][]byte, timestamp time.Time) ([]byte, error) {
	root := fmt.Sprintf("qweather-cli_%s_%s_%s", version, target.GOOS, target.GOARCH)
	binaryName := "qweather"
	if target.GOOS == "windows" {
		binaryName += ".exe"
	}
	ordered := []struct {
		name string
		mode int64
	}{
		{name: binaryName, mode: 0o755},
		{name: "LICENSE", mode: 0o644},
		{name: "README.md", mode: 0o644},
	}
	if target.Format != "tar.gz" && target.Format != "zip" {
		return nil, fmt.Errorf("unsupported archive format %q", target.Format)
	}
	if len(files) != len(ordered) {
		return nil, fmt.Errorf("archive payload must contain exactly %d files", len(ordered))
	}
	for _, entry := range ordered {
		if _, ok := files[entry.name]; !ok {
			return nil, fmt.Errorf("archive payload is missing %s", entry.name)
		}
	}
	for name := range files {
		found := false
		for _, entry := range ordered {
			if entry.name == name {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("archive payload contains unexpected file %s", name)
		}
	}

	if target.Format == "zip" {
		return buildZipArchive(root, ordered, files, timestamp)
	}
	return buildTarGzArchive(root, ordered, files, timestamp)
}

func buildTarGzArchive(root string, ordered []struct {
	name string
	mode int64
}, files map[string][]byte, timestamp time.Time) ([]byte, error) {
	var output bytes.Buffer
	gzipWriter := gzip.NewWriter(&output)
	gzipWriter.ModTime = timestamp.UTC()
	tarWriter := tar.NewWriter(gzipWriter)
	for _, entry := range ordered {
		data := files[entry.name]
		header := &tar.Header{
			Name:    root + "/" + entry.name,
			Mode:    entry.mode,
			Size:    int64(len(data)),
			ModTime: timestamp.UTC(),
			Uid:     0,
			Gid:     0,
			Uname:   "",
			Gname:   "",
			Format:  tar.FormatPAX,
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			return nil, fmt.Errorf("write tar header for %s: %w", entry.name, err)
		}
		if _, err := io.Copy(tarWriter, bytes.NewReader(data)); err != nil {
			return nil, fmt.Errorf("write tar file %s: %w", entry.name, err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		return nil, fmt.Errorf("close tar archive: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, fmt.Errorf("close gzip archive: %w", err)
	}
	return output.Bytes(), nil
}

func buildZipArchive(root string, ordered []struct {
	name string
	mode int64
}, files map[string][]byte, timestamp time.Time) ([]byte, error) {
	var output bytes.Buffer
	zipWriter := zip.NewWriter(&output)
	for _, entry := range ordered {
		header := &zip.FileHeader{Name: root + "/" + entry.name, Method: zip.Deflate}
		header.SetModTime(timestamp.UTC())
		header.SetMode(fs.FileMode(entry.mode))
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return nil, fmt.Errorf("create zip entry for %s: %w", entry.name, err)
		}
		if _, err := writer.Write(files[entry.name]); err != nil {
			return nil, fmt.Errorf("write zip file %s: %w", entry.name, err)
		}
	}
	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("close zip archive: %w", err)
	}
	return output.Bytes(), nil
}
