package cache

import (
	"errors"
	"net/url"
	"strings"
	"testing"

	"github.com/Nativu5/qweather-cli/internal/catalog"
	"github.com/Nativu5/qweather-cli/internal/place"
	"github.com/Nativu5/qweather-cli/internal/qweather"
)

func testCapability(t *testing.T, id string) catalog.Capability {
	t.Helper()
	registry, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	capability, ok := registry.Find(id)
	if !ok {
		t.Fatalf("missing capability %s", id)
	}
	return capability
}

func TestBuildKeyIsCanonicalAndScoped(t *testing.T) {
	capability := testCapability(t, "weather.city.current")
	material := Material{
		APIHost: "EXAMPLE.QWEATHERAPI.COM.", Profile: "default", EffectiveLang: "en", EffectiveUnit: "metric",
		Resolved: place.Resolved{ID: "101010100"},
		Request: qweather.Request{
			CapabilityID: capability.ID, Path: "/v7/weather/now",
			Query: url.Values{"lang": {"en"}, "location": {"101010100"}},
		},
	}
	first, err := BuildKey(capability, material)
	if err != nil {
		t.Fatal(err)
	}
	material.Request.Query = url.Values{"location": {"101010100"}, "lang": {"en"}}
	second, err := BuildKey(capability, material)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || len(first.String()) != 64 {
		t.Fatalf("first=%s second=%s", first.String(), second.String())
	}
	material.Resolved = place.Resolved{ID: "101010100", Lat: "39.9", Lon: "116.4", TZ: "Asia/Shanghai"}
	richResolution, err := BuildKey(capability, material)
	if err != nil {
		t.Fatal(err)
	}
	if richResolution != first {
		t.Fatal("invocation-local Geo details changed an ID-target cache key")
	}
	if strings.Contains(first.String(), "101010100") || strings.Contains(first.String(), "example") {
		t.Fatalf("opaque key exposes source material: %s", first.String())
	}
	material.Profile = "other"
	third, err := BuildKey(capability, material)
	if err != nil {
		t.Fatal(err)
	}
	if third == first {
		t.Fatal("profile scope did not affect cache key")
	}
	capability.RequestRevision++
	fourth, err := BuildKey(capability, material)
	if err != nil {
		t.Fatal(err)
	}
	if fourth == third {
		t.Fatal("request revision did not affect cache key")
	}
}

func TestBuildKeyEnforcesGeoAndSensitivePolicies(t *testing.T) {
	geo := testCapability(t, "geo.city.top")
	_, err := BuildKey(geo, Material{APIHost: "example.com", Profile: "default", Request: qweather.Request{CapabilityID: geo.ID, Path: geo.Upstream.PathTemplate}})
	if !errors.Is(err, ErrPolicyDisabled) {
		t.Fatalf("Geo BuildKey() error = %v", err)
	}
	account := testCapability(t, "account.finance.summary")
	material := Material{APIHost: "example.com", Profile: "default", Request: qweather.Request{CapabilityID: account.ID, Path: account.Upstream.PathTemplate}}
	if _, err := BuildKey(account, material); !errors.Is(err, ErrPolicyDisabled) {
		t.Fatalf("Account BuildKey() error = %v", err)
	}
	material.AllowSensitive = true
	if _, err := BuildKey(account, material); err != nil {
		t.Fatalf("opted-in Account BuildKey() error = %v", err)
	}
}

func TestEnabledUsesOnlyExplicitCapabilityPolicy(t *testing.T) {
	if !Enabled(testCapability(t, "weather.city.current"), true, false) {
		t.Fatal("ordinary data capability should be enabled")
	}
	if Enabled(testCapability(t, "geo.city.lookup"), true, true) {
		t.Fatal("Geo capability must never be enabled")
	}
	account := testCapability(t, "account.requests.stats")
	if Enabled(account, true, false) || !Enabled(account, true, true) {
		t.Fatal("Account capability did not require sensitive opt-in")
	}
	if Enabled(testCapability(t, "weather.city.current"), false, true) {
		t.Fatal("global cache disable was ignored")
	}
}
