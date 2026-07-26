package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"testing"
	"time"
)

func TestParseStableVersionAcceptsOnlyStableSemVer(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
		ok    bool
	}{
		{name: "stable", input: "0.1.0", want: "0.1.0", ok: true},
		{name: "multi digit", input: "12.34.56", want: "12.34.56", ok: true},
		{name: "v prefix", input: "v0.1.0"},
		{name: "leading major zero", input: "01.2.3"},
		{name: "leading minor zero", input: "1.02.3"},
		{name: "leading patch zero", input: "1.2.03"},
		{name: "prerelease", input: "1.2.3-rc.1"},
		{name: "build suffix", input: "1.2.3+build"},
		{name: "missing component", input: "1.2"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseStableVersion(test.input)
			if !test.ok {
				if err == nil {
					t.Fatalf("ParseStableVersion(%q) error = nil, want error", test.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseStableVersion(%q) error = %v", test.input, err)
			}
			if got.String() != test.want {
				t.Fatalf("ParseStableVersion(%q) = %q, want %q", test.input, got.String(), test.want)
			}
		})
	}
}

func TestReleaseAssetNamesMatchAcceptedContract(t *testing.T) {
	version, err := ParseStableVersion("1.2.3")
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		"qweather-cli_1.2.3_darwin_arm64.tar.gz",
		"qweather-cli_1.2.3_darwin_amd64.tar.gz",
		"qweather-cli_1.2.3_linux_arm64.tar.gz",
		"qweather-cli_1.2.3_linux_amd64.tar.gz",
		"qweather-cli_1.2.3_windows_arm64.zip",
		"qweather-cli_1.2.3_windows_amd64.zip",
		"checksums.txt",
	}
	got := ReleaseAssetNames(version)
	if len(got) != len(want) {
		t.Fatalf("ReleaseAssetNames length = %d, want %d", len(got), len(want))
	}
	for index := range want {
		if got[index] != want[index] {
			t.Errorf("ReleaseAssetNames[%d] = %q, want %q", index, got[index], want[index])
		}
	}
}

func TestBuildArchiveIsDeterministicAndContainsOnlyApprovedFiles(t *testing.T) {
	version, err := ParseStableVersion("1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		"qweather":  []byte("binary\n"),
		"LICENSE":   []byte("license\n"),
		"README.md": []byte("readme\n"),
	}
	timestamp := time.Unix(1_750_000_000, 0).UTC()

	first, err := BuildArchive(version, releaseTarget{GOOS: "linux", GOARCH: "amd64", Format: "tar.gz"}, files, timestamp)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildArchive(version, releaseTarget{GOOS: "linux", GOARCH: "amd64", Format: "tar.gz"}, files, timestamp)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("identical archive inputs produced different bytes")
	}

	reader, err := gzip.NewReader(bytes.NewReader(first))
	if err != nil {
		t.Fatal(err)
	}
	tarReader := tar.NewReader(reader)
	var names []string
	for {
		header, nextErr := tarReader.Next()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			t.Fatal(nextErr)
		}
		names = append(names, header.Name)
	}
	want := []string{"qweather-cli_1.2.3_linux_amd64/qweather", "qweather-cli_1.2.3_linux_amd64/LICENSE", "qweather-cli_1.2.3_linux_amd64/README.md"}
	if !equalStrings(names, want) {
		t.Fatalf("tar entries = %#v, want %#v", names, want)
	}
}

func TestBuildZipArchiveContainsOnlyApprovedFiles(t *testing.T) {
	version, err := ParseStableVersion("1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	archiveBytes, err := BuildArchive(version, releaseTarget{GOOS: "windows", GOARCH: "amd64", Format: "zip"}, map[string][]byte{
		"qweather.exe": []byte("binary\n"),
		"LICENSE":      []byte("license\n"),
		"README.md":    []byte("readme\n"),
	}, time.Unix(1_750_000_000, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(archiveBytes), int64(len(archiveBytes)))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, file := range reader.File {
		names = append(names, file.Name)
	}
	want := []string{"qweather-cli_1.2.3_windows_amd64/qweather.exe", "qweather-cli_1.2.3_windows_amd64/LICENSE", "qweather-cli_1.2.3_windows_amd64/README.md"}
	if !equalStrings(names, want) {
		t.Fatalf("zip entries = %#v, want %#v", names, want)
	}
}

func TestBuildChecksumManifestIsSortedAndUsesAcceptedFormat(t *testing.T) {
	got, err := BuildChecksumManifest(map[string][]byte{
		"qweather-cli_1.2.3_windows_amd64.zip":  []byte("windows"),
		"qweather-cli_1.2.3_linux_amd64.tar.gz": []byte("linux"),
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "caf90169eefa5f807d577486b9f795ab86ae2983c5c20806cff959117e90af18  qweather-cli_1.2.3_linux_amd64.tar.gz\n" +
		"340d600392818df2413382dc7d8325c360d83ea49a262d31760348484bbc10b5  qweather-cli_1.2.3_windows_amd64.zip\n"
	if string(got) != want {
		t.Fatalf("manifest = %q, want %q", got, want)
	}
}

func TestPackageArtifactsProducesSixArchivesAndOneManifest(t *testing.T) {
	version, err := ParseStableVersion("1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	binaries := make(map[string][]byte)
	for _, target := range releaseTargets {
		binaries[target.GOOS+"/"+target.GOARCH] = []byte(target.GOOS + "/" + target.GOARCH)
	}
	artifacts, err := PackageArtifacts(version, binaries, []byte("license\n"), []byte("readme\n"), time.Unix(1_750_000_000, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	wantNames := ReleaseAssetNames(version)
	if len(artifacts) != len(wantNames) {
		t.Fatalf("artifacts length = %d, want %d", len(artifacts), len(wantNames))
	}
	for _, name := range wantNames {
		if len(artifacts[name]) == 0 {
			t.Errorf("artifact %s is missing or empty", name)
		}
	}
	if err := VerifyArtifactSet(version, artifacts); err != nil {
		t.Fatalf("VerifyArtifactSet() error = %v", err)
	}

	artifacts["qweather-cli_1.2.3_linux_amd64.tar.gz"] = []byte("corrupted")
	if err := VerifyArtifactSet(version, artifacts); err == nil {
		t.Fatal("VerifyArtifactSet() accepted a corrupted archive")
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range want {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}
