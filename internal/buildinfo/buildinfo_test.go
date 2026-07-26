package buildinfo

import "testing"

func TestCurrentNormalizesBuildTimeToUTC(t *testing.T) {
	original := BuildTime
	t.Cleanup(func() { BuildTime = original })
	BuildTime = "2026-07-26T13:21:22+08:00"

	got := Current("registry-hash")
	if got.BuildTime != "2026-07-26T05:21:22Z" {
		t.Fatalf("Current().BuildTime = %q, want UTC RFC3339", got.BuildTime)
	}
}

func TestCurrentPreservesUnknownBuildTime(t *testing.T) {
	original := BuildTime
	t.Cleanup(func() { BuildTime = original })
	BuildTime = "unknown"

	if got := Current("registry-hash").BuildTime; got != "unknown" {
		t.Fatalf("Current().BuildTime = %q, want unknown", got)
	}
}
