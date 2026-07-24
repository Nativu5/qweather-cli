package cache

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Nativu5/qweather-cli/internal/place"
	"github.com/Nativu5/qweather-cli/internal/qweather"
)

func cacheFixture(t *testing.T) (*Store, Key, Record, *time.Time) {
	t.Helper()
	now := time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)
	store, err := NewStore(filepath.Join(t.TempDir(), "cache"), "default", func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	capability := testCapability(t, "weather.city.current")
	key, err := BuildKey(capability, Material{
		APIHost: "example.qweatherapi.com", Profile: "default", EffectiveLang: "en",
		Resolved: place.Resolved{ID: "101010100"},
		Request:  qweather.Request{CapabilityID: capability.ID, Path: "/v7/weather/now"},
	})
	if err != nil {
		t.Fatal(err)
	}
	record, err := NewRecord(capability, "ok", qweather.Response{StatusCode: 200, Body: []byte("{\n  \"code\": \"200\",\n  \"now\": {\"temp\": \"20\"}\n}")}, now, now.Add(10*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	return store, key, record, &now
}

func TestStoreMissPutHitAndPrivatePermissions(t *testing.T) {
	store, key, record, _ := cacheFixture(t)
	if _, hit, err := store.Get(context.Background(), key); err != nil || hit {
		t.Fatalf("initial Get() hit=%v err=%v", hit, err)
	}
	if err := store.Put(context.Background(), key, record); err != nil {
		t.Fatal(err)
	}
	got, hit, err := store.Get(context.Background(), key)
	if err != nil || !hit || string(got.ProviderBody) != string(record.ProviderBody) {
		t.Fatalf("Get() record=%#v hit=%v err=%v", got, hit, err)
	}
	path, err := store.entryPath(key)
	if err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{store.root, store.profilePath(), filepath.Dir(path)} {
		info, err := os.Stat(directory)
		if err != nil {
			t.Fatalf("stat directory %s: %v", directory, err)
		}
		if info.Mode().Perm() != 0o700 {
			t.Fatalf("directory %s mode=%v", directory, info.Mode().Perm())
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat record: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("record mode=%v", info.Mode().Perm())
	}
}

func TestNewRecordNamesAndStoresPolicyMaximumTTL(t *testing.T) {
	now := time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)
	capability := testCapability(t, "storm.forecast")
	record, err := NewRecord(
		capability,
		"ok",
		qweather.Response{StatusCode: 200, Body: []byte(`{"code":"200","forecast":[{}]}`)},
		now,
		now.Add(20*time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}
	if record.Schema != "qweather.cache-record/v3" {
		t.Fatalf("Schema = %q, want qweather.cache-record/v3", record.Schema)
	}
	if record.PolicyMaxTTLSeconds != int64(time.Hour/time.Second) {
		t.Fatalf("PolicyMaxTTLSeconds = %d, want %d", record.PolicyMaxTTLSeconds, int64(time.Hour/time.Second))
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"policyMaxTtlSeconds":3600`) || strings.Contains(string(encoded), `"ttlSeconds"`) {
		t.Fatalf("record TTL field is ambiguous: %s", encoded)
	}
}

func TestStoreExpiresAndRemovesCorruptRecords(t *testing.T) {
	store, key, record, now := cacheFixture(t)
	if err := store.Put(context.Background(), key, record); err != nil {
		t.Fatal(err)
	}
	*now = record.ExpiresAt
	if _, hit, err := store.Get(context.Background(), key); err != nil || hit {
		t.Fatalf("expired Get() hit=%v err=%v", hit, err)
	}
	path, _ := store.entryPath(key)
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expired record still exists: %v", err)
	}
	*now = record.StoredAt
	if err := store.Put(context.Background(), key, record); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"schema":"broken"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, hit, err := store.Get(context.Background(), key); err != nil || hit {
		t.Fatalf("corrupt Get() hit=%v err=%v", hit, err)
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("corrupt record still exists: %v", err)
	}
}

func TestStoreInvalidatesPrivateDevelopmentV2Records(t *testing.T) {
	store, key, record, _ := cacheFixture(t)
	if err := store.Put(context.Background(), key, record); err != nil {
		t.Fatal(err)
	}
	path := mustEntryPath(t, store, key)
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	contents = []byte(strings.Replace(string(contents), "qweather.cache-record/v3", "qweather.cache-record/v2", 1))
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, hit, err := store.Get(context.Background(), key); err != nil || hit {
		t.Fatalf("v2 Get() hit=%v err=%v", hit, err)
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("v2 record still exists: %v", err)
	}
}

func TestStoreAtomicallyReplacesRecords(t *testing.T) {
	store, key, record, _ := cacheFixture(t)
	if err := store.Put(context.Background(), key, record); err != nil {
		t.Fatal(err)
	}
	replacement := record
	replacement.ProviderBody = json.RawMessage(`{"code":"200","now":{"temp":"21"}}`)
	if err := store.Put(context.Background(), key, replacement); err != nil {
		t.Fatal(err)
	}
	got, hit, err := store.Get(context.Background(), key)
	if err != nil || !hit || !strings.Contains(string(got.ProviderBody), `"21"`) {
		t.Fatalf("Get() record=%#v hit=%v err=%v", got, hit, err)
	}
	entries, err := os.ReadDir(filepath.Dir(mustEntryPath(t, store, key)))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".qweather-cache-") {
			t.Fatalf("temporary file remains: %s", entry.Name())
		}
	}
}

func TestStatusAndClearDoNotExposeKeysOrTargets(t *testing.T) {
	store, key, record, _ := cacheFixture(t)
	if err := store.Put(context.Background(), key, record); err != nil {
		t.Fatal(err)
	}
	status, err := store.Status(context.Background(), true, false)
	if err != nil || status.Entries != 1 || status.Bytes == 0 {
		t.Fatalf("status=%#v err=%v", status, err)
	}
	encoded, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), key.String()) || strings.Contains(string(encoded), "101010100") {
		t.Fatalf("status exposes target or key: %s", encoded)
	}
	cleared, err := store.Clear(context.Background(), key.CapabilityID(), false)
	if err != nil || cleared.Entries != 1 || cleared.Scope != "profile" {
		t.Fatalf("clear=%#v err=%v", cleared, err)
	}
	if _, hit, err := store.Get(context.Background(), key); err != nil || hit {
		t.Fatalf("Get() after clear hit=%v err=%v", hit, err)
	}
}

func TestClearCanExplicitlyScopeEveryProfile(t *testing.T) {
	store, key, record, now := cacheFixture(t)
	if err := store.Put(context.Background(), key, record); err != nil {
		t.Fatal(err)
	}
	other, err := NewStore(store.root, "other", func() time.Time { return *now })
	if err != nil {
		t.Fatal(err)
	}
	capability := testCapability(t, key.CapabilityID())
	otherKey, err := BuildKey(capability, Material{
		APIHost: "example.qweatherapi.com", Profile: "other", EffectiveLang: "en",
		Resolved: place.Resolved{ID: "101010100"},
		Request:  qweather.Request{CapabilityID: capability.ID, Path: "/v7/weather/now"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := other.Put(context.Background(), otherKey, record); err != nil {
		t.Fatal(err)
	}
	cleared, err := store.Clear(context.Background(), capability.ID, true)
	if err != nil || cleared.Scope != "all-profiles" || cleared.Entries != 2 {
		t.Fatalf("clear=%#v err=%v", cleared, err)
	}
	if _, hit, err := other.Get(context.Background(), otherKey); err != nil || hit {
		t.Fatalf("other profile hit=%v err=%v", hit, err)
	}
}

func mustEntryPath(t *testing.T, store *Store, key Key) string {
	t.Helper()
	path, err := store.entryPath(key)
	if err != nil {
		t.Fatal(err)
	}
	return path
}
