package cache

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Nativu5/qweather-cli/internal/catalog"
	"github.com/Nativu5/qweather-cli/internal/qweather"
)

const (
	recordSchema   = "qweather.cache-record/v1"
	statusSchema   = "qweather.cache-status/v1"
	clearSchema    = "qweather.cache-clear/v1"
	maxRecordBytes = (qweather.DefaultMaxBodyBytes * 4 / 3) + (128 << 10)
	cleanupBudget  = 64
)

var safeSegment = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

type Record struct {
	Schema         string                 `json:"schema"`
	Capability     string                 `json:"capability"`
	Outcome        string                 `json:"outcome"`
	StoredAt       time.Time              `json:"storedAt"`
	ExpiresAt      time.Time              `json:"expiresAt"`
	TTLSeconds     int64                  `json:"ttlSeconds"`
	HTTPStatus     int                    `json:"httpStatus"`
	ResponseFamily catalog.ResponseFamily `json:"responseFamily"`
	ProviderBody   []byte                 `json:"providerBody,omitempty"`
}

type Status struct {
	Schema           string `json:"schema"`
	Profile          string `json:"profile"`
	Enabled          bool   `json:"enabled"`
	SensitiveEnabled bool   `json:"sensitiveEnabled"`
	Entries          int    `json:"entries"`
	Bytes            int64  `json:"bytes"`
	Expired          int    `json:"expired"`
	Corrupt          int    `json:"corrupt"`
	OldestStoredAt   string `json:"oldestStoredAt,omitempty"`
	NearestExpiresAt string `json:"nearestExpiresAt,omitempty"`
}

type ClearResult struct {
	Schema     string `json:"schema"`
	Scope      string `json:"scope"`
	Profile    string `json:"profile,omitempty"`
	Capability string `json:"capability,omitempty"`
	Entries    int    `json:"entriesRemoved"`
	Bytes      int64  `json:"bytesRemoved"`
}

type Store struct {
	root    string
	profile string
	now     func() time.Time
}

func NewStore(root, profile string, now func() time.Time) (*Store, error) {
	if strings.TrimSpace(root) == "" {
		return nil, errors.New("cache root is required")
	}
	if !safeSegment.MatchString(profile) {
		return nil, errors.New("cache profile contains unsupported characters")
	}
	if now == nil {
		now = time.Now
	}
	return &Store{root: filepath.Clean(root), profile: profile, now: now}, nil
}

func NewRecord(capability catalog.Capability, outcome string, response qweather.Response, storedAt, expiresAt time.Time) (Record, error) {
	hardTTL := maximumTTL(capability.Cache)
	record := Record{
		Schema: recordSchema, Capability: capability.ID, Outcome: outcome,
		StoredAt: storedAt.UTC(), ExpiresAt: expiresAt.UTC(), TTLSeconds: int64(hardTTL / time.Second),
		HTTPStatus: response.StatusCode, ResponseFamily: capability.Upstream.ResponseFamily,
		ProviderBody: append([]byte(nil), response.Body...),
	}
	if err := validateRecord(record, capability.ID); err != nil {
		return Record{}, err
	}
	if expiresAt.After(storedAt.Add(hardTTL)) {
		return Record{}, errors.New("cache record exceeds its hard TTL")
	}
	return record, nil
}

func (s *Store) Get(ctx context.Context, key Key) (Record, bool, error) {
	if err := ctx.Err(); err != nil {
		return Record{}, false, err
	}
	path, err := s.entryPath(key)
	if err != nil {
		return Record{}, false, err
	}
	record, err := readRecord(path, key.capabilityID)
	if errors.Is(err, fs.ErrNotExist) {
		return Record{}, false, nil
	}
	if err != nil {
		_ = os.Remove(path)
		return Record{}, false, nil
	}
	if err := validateKeyRecord(record, key); err != nil {
		_ = os.Remove(path)
		return Record{}, false, nil
	}
	if !s.now().UTC().Before(record.ExpiresAt) {
		_ = os.Remove(path)
		return Record{}, false, nil
	}
	return record, true, nil
}

func (s *Store) Put(ctx context.Context, key Key, record Record) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateRecord(record, key.capabilityID); err != nil {
		return err
	}
	if err := validateKeyRecord(record, key); err != nil {
		return err
	}
	path, err := s.entryPath(key)
	if err != nil {
		return err
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return fmt.Errorf("create cache root: %w", err)
	}
	for _, path := range []string{s.root, filepath.Join(s.root, "v1"), filepath.Join(s.root, "v1", "profiles"), s.profilePath(), directory} {
		if err := ensurePrivateDirectory(path); err != nil {
			return err
		}
	}
	contents, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode cache record: %w", err)
	}
	if int64(len(contents)) > maxRecordBytes {
		return errors.New("cache record exceeds the maximum size")
	}
	temporary, err := os.CreateTemp(directory, ".qweather-cache-*")
	if err != nil {
		return fmt.Errorf("create cache temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set cache temporary permissions: %w", err)
	}
	if _, err := temporary.Write(contents); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write cache temporary file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync cache temporary file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close cache temporary file: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace cache record atomically: %w", err)
	}
	if err := syncDirectory(directory); err != nil {
		return err
	}
	_ = s.cleanup(ctx, cleanupBudget)
	return nil
}

func (s *Store) Delete(ctx context.Context, key Key) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path, err := s.entryPath(key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove cache record: %w", err)
	}
	return nil
}

func (s *Store) Status(ctx context.Context, enabled, sensitiveEnabled bool) (Status, error) {
	result := Status{Schema: statusSchema, Profile: s.profile, Enabled: enabled, SensitiveEnabled: sensitiveEnabled}
	var oldest, nearest time.Time
	err := walkJSON(ctx, s.profilePath(), 0, func(path string, info fs.FileInfo) error {
		result.Entries++
		result.Bytes += info.Size()
		record, err := readRecord(path, "")
		if err != nil {
			result.Corrupt++
			return nil
		}
		now := s.now().UTC()
		if !now.Before(record.ExpiresAt) {
			result.Expired++
		}
		if oldest.IsZero() || record.StoredAt.Before(oldest) {
			oldest = record.StoredAt
		}
		if nearest.IsZero() || record.ExpiresAt.Before(nearest) {
			nearest = record.ExpiresAt
		}
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Status{}, err
	}
	if !oldest.IsZero() {
		result.OldestStoredAt = oldest.UTC().Format(time.RFC3339)
	}
	if !nearest.IsZero() {
		result.NearestExpiresAt = nearest.UTC().Format(time.RFC3339)
	}
	return result, nil
}

func (s *Store) Clear(ctx context.Context, capabilityID string, allProfiles bool) (ClearResult, error) {
	if capabilityID != "" && !safeSegment.MatchString(capabilityID) {
		return ClearResult{}, errors.New("cache capability contains unsupported characters")
	}
	result := ClearResult{Schema: clearSchema, Capability: capabilityID}
	if allProfiles {
		result.Scope = "all-profiles"
	} else {
		result.Scope = "profile"
		result.Profile = s.profile
	}
	paths := make([]string, 0)
	if allProfiles && capabilityID == "" {
		paths = append(paths, filepath.Join(s.root, "v1", "profiles"))
	} else if allProfiles {
		profiles, err := os.ReadDir(filepath.Join(s.root, "v1", "profiles"))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return ClearResult{}, fmt.Errorf("list cache profiles: %w", err)
		}
		for _, profile := range profiles {
			if profile.IsDir() && safeSegment.MatchString(profile.Name()) {
				paths = append(paths, filepath.Join(s.root, "v1", "profiles", profile.Name(), capabilityID))
			}
		}
	} else if capabilityID == "" {
		paths = append(paths, s.profilePath())
	} else {
		paths = append(paths, filepath.Join(s.profilePath(), capabilityID))
	}
	for _, path := range paths {
		entries, bytes, err := measureJSON(ctx, path)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return ClearResult{}, err
		}
		result.Entries += entries
		result.Bytes += bytes
		if err := os.RemoveAll(path); err != nil {
			return ClearResult{}, fmt.Errorf("clear cache scope: %w", err)
		}
	}
	return result, nil
}

func (s *Store) cleanup(ctx context.Context, budget int) error {
	visited := 0
	return walkJSON(ctx, s.profilePath(), budget, func(path string, _ fs.FileInfo) error {
		visited++
		record, err := readRecord(path, "")
		if err != nil || !s.now().UTC().Before(record.ExpiresAt) {
			_ = os.Remove(path)
		}
		if visited >= budget {
			return fs.SkipAll
		}
		return nil
	})
}

func (s *Store) profilePath() string {
	return filepath.Join(s.root, "v1", "profiles", s.profile)
}

func (s *Store) entryPath(key Key) (string, error) {
	if key.capabilityID == "" || !safeSegment.MatchString(key.capabilityID) {
		return "", errors.New("cache key capability is invalid")
	}
	if key.profile != s.profile {
		return "", errors.New("cache key belongs to a different profile")
	}
	digest := key.String()
	if len(digest) != sha256HexLength {
		return "", errors.New("cache key digest is invalid")
	}
	return filepath.Join(s.profilePath(), key.capabilityID, digest+".json"), nil
}

const sha256HexLength = 64

func validateRecord(record Record, capabilityID string) error {
	if record.Schema != recordSchema {
		return errors.New("cache record schema is not recognized")
	}
	if record.Capability == "" || (capabilityID != "" && record.Capability != capabilityID) {
		return errors.New("cache record capability does not match its key")
	}
	if record.Outcome != "ok" && record.Outcome != "no_data" {
		return errors.New("cache record outcome is invalid")
	}
	if record.StoredAt.IsZero() || !record.ExpiresAt.After(record.StoredAt) || record.TTLSeconds <= 0 {
		return errors.New("cache record timestamps are invalid")
	}
	if record.HTTPStatus < 200 || record.HTTPStatus >= 300 {
		return errors.New("cache record HTTP status is invalid")
	}
	if record.ResponseFamily != catalog.ResponseLegacyV1 && record.ResponseFamily != catalog.ResponseModernV1 && record.ResponseFamily != catalog.ResponseConsoleV1 {
		return errors.New("cache record response family is invalid")
	}
	if len(record.ProviderBody) > 0 && !json.Valid(record.ProviderBody) {
		return errors.New("cache record provider body is invalid JSON")
	}
	if len(record.ProviderBody) == 0 && record.HTTPStatus != 204 {
		return errors.New("cache record provider body is missing")
	}
	return nil
}

func validateKeyRecord(record Record, key Key) error {
	if key.ttl <= 0 || record.TTLSeconds != int64(key.ttl/time.Second) {
		return errors.New("cache record TTL does not match its capability policy")
	}
	if record.ExpiresAt.After(record.StoredAt.Add(key.ttl)) {
		return errors.New("cache record exceeds its hard TTL")
	}
	if record.ResponseFamily != key.family {
		return errors.New("cache record response family does not match its capability")
	}
	return nil
}

func readRecord(path, capabilityID string) (Record, error) {
	file, err := os.Open(path)
	if err != nil {
		return Record{}, err
	}
	defer file.Close()
	contents, err := io.ReadAll(io.LimitReader(file, maxRecordBytes+1))
	if err != nil {
		return Record{}, fmt.Errorf("read cache record: %w", err)
	}
	if int64(len(contents)) > maxRecordBytes {
		return Record{}, errors.New("cache record exceeds the maximum size")
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	record := Record{}
	if err := decoder.Decode(&record); err != nil {
		return Record{}, errors.New("cache record is malformed")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return Record{}, errors.New("cache record contains trailing data")
	}
	if err := validateRecord(record, capabilityID); err != nil {
		return Record{}, err
	}
	return record, nil
}

func ensurePrivateDirectory(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		if err := os.Mkdir(path, 0o700); err != nil {
			return fmt.Errorf("create private cache directory: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect cache directory: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("cache path component is not a real directory")
	}
	if info.Mode().Perm() != 0o700 {
		if err := os.Chmod(path, 0o700); err != nil {
			return fmt.Errorf("set private cache directory permissions: %w", err)
		}
	}
	return nil
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open cache directory for sync: %w", err)
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("sync cache directory: %w", err)
	}
	return nil
}

func walkJSON(ctx context.Context, root string, budget int, visit func(string, fs.FileInfo) error) error {
	count := 0
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || filepath.Ext(entry.Name()) != ".json" || strings.HasPrefix(entry.Name(), ".qweather-cache-") {
			return nil
		}
		if budget > 0 && count >= budget {
			return fs.SkipAll
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		count++
		return visit(path, info)
	})
}

func measureJSON(ctx context.Context, root string) (int, int64, error) {
	entries := 0
	var bytes int64
	err := walkJSON(ctx, root, 0, func(_ string, info fs.FileInfo) error {
		entries++
		bytes += info.Size()
		return nil
	})
	return entries, bytes, err
}
