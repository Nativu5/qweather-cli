package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

const (
	upstreamRepository    = "https://github.com/qwd/dev-site"
	manifestSchema        = "qweather.openapi-snapshot/v1"
	expectedExampleCount  = 53
	englishSpecification  = "qweather-apis-en.yml"
	chineseSpecification  = "qweather-apis-zh.yml"
	upstreamOpenAPIRoot   = "assets/openapi"
	openAPISnapshotSuffix = "references/upstream/openapi"
)

type manifestFile struct {
	Path   string `json:"path"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type localeComparison struct {
	OperationCount    int      `json:"operationCount"`
	SchemaDifferences []string `json:"schemaDifferences"`
}

type snapshotManifest struct {
	Schema           string           `json:"schema"`
	Repository       string           `json:"repository"`
	Commit           string           `json:"commit"`
	Files            []manifestFile   `json:"files"`
	LocaleComparison localeComparison `json:"localeComparison"`
}

type operationContract struct {
	OperationID string
	Deprecated  bool
	Parameters  []string
	Responses   []string
	Structure   string
}

type openAPIDocument struct {
	root           map[string]any
	operations     map[string]operationContract
	externalValues map[string]bool
}

func syncOpenAPI(root, source, commit string) error {
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(commit) {
		return fmt.Errorf("upstream commit must be a full lowercase 40-character SHA: %q", commit)
	}
	if commit != pinnedCommit {
		return fmt.Errorf("commit %s is not the reviewed pin %s; update the accepted design and Issue before advancing it", commit, pinnedCommit)
	}
	remoteBytes, err := gitOutput(source, "remote", "get-url", "origin")
	if err != nil {
		return err
	}
	if normalizeGitURL(strings.TrimSpace(string(remoteBytes))) != upstreamRepository {
		return fmt.Errorf("source origin is %q, want %s", strings.TrimSpace(string(remoteBytes)), upstreamRepository)
	}
	resolvedBytes, err := gitOutput(source, "rev-parse", commit+"^{commit}")
	if err != nil {
		return err
	}
	if resolved := strings.TrimSpace(string(resolvedBytes)); resolved != commit {
		return fmt.Errorf("resolved upstream commit %s, want %s", resolved, commit)
	}

	english, err := gitFile(source, commit, path.Join(upstreamOpenAPIRoot, englishSpecification))
	if err != nil {
		return err
	}
	chinese, err := gitFile(source, commit, path.Join(upstreamOpenAPIRoot, chineseSpecification))
	if err != nil {
		return err
	}
	englishDocument, err := parseOpenAPI("English", english)
	if err != nil {
		return err
	}
	chineseDocument, err := parseOpenAPI("Chinese", chinese)
	if err != nil {
		return err
	}
	exampleNames := unionExternalValues(englishDocument.externalValues, chineseDocument.externalValues)
	if len(exampleNames) != expectedExampleCount {
		return fmt.Errorf("OpenAPI locales reference %d unique examples, want %d", len(exampleNames), expectedExampleCount)
	}
	comparison, err := compareLocales(englishDocument, chineseDocument)
	if err != nil {
		return err
	}

	target := filepath.Join(root, defaultSkillPath, filepath.FromSlash(openAPISnapshotSuffix))
	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create snapshot parent: %w", err)
	}
	staging, err := os.MkdirTemp(parent, ".openapi-sync-*")
	if err != nil {
		return fmt.Errorf("create snapshot staging directory: %w", err)
	}
	defer os.RemoveAll(staging)
	if err := os.MkdirAll(filepath.Join(staging, "examples"), 0o755); err != nil {
		return fmt.Errorf("create staged examples directory: %w", err)
	}
	if err := os.WriteFile(filepath.Join(staging, englishSpecification), english, 0o644); err != nil {
		return fmt.Errorf("stage English OpenAPI: %w", err)
	}
	if err := os.WriteFile(filepath.Join(staging, chineseSpecification), chinese, 0o644); err != nil {
		return fmt.Errorf("stage Chinese OpenAPI: %w", err)
	}
	for _, name := range exampleNames {
		data, err := gitFile(source, commit, path.Join(upstreamOpenAPIRoot, "examples", name))
		if err != nil {
			return err
		}
		if !json.Valid(data) {
			return fmt.Errorf("upstream example %s is not valid JSON", name)
		}
		if err := os.WriteFile(filepath.Join(staging, "examples", name), data, 0o644); err != nil {
			return fmt.Errorf("stage example %s: %w", name, err)
		}
	}
	if err := os.WriteFile(filepath.Join(staging, "NOTICE.md"), buildNotice(commit), 0o644); err != nil {
		return fmt.Errorf("stage NOTICE.md: %w", err)
	}
	manifest, err := buildManifest(staging, commit, comparison)
	if err != nil {
		return err
	}
	manifestBytes, err := marshalManifest(manifest)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(staging, "manifest.json"), manifestBytes, 0o644); err != nil {
		return fmt.Errorf("stage manifest.json: %w", err)
	}
	if err := validateSnapshot(staging); err != nil {
		return fmt.Errorf("validate staged snapshot: %w", err)
	}
	if err := replaceDirectory(staging, target); err != nil {
		return err
	}
	return nil
}

func checkSnapshot(root string) error {
	return validateSnapshot(filepath.Join(root, defaultSkillPath, filepath.FromSlash(openAPISnapshotSuffix)))
}

func validateSnapshot(directory string) error {
	manifestBytes, err := os.ReadFile(filepath.Join(directory, "manifest.json"))
	if err != nil {
		return fmt.Errorf("read snapshot manifest: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(manifestBytes))
	decoder.DisallowUnknownFields()
	var manifest snapshotManifest
	if err := decoder.Decode(&manifest); err != nil {
		return fmt.Errorf("decode snapshot manifest: %w", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return fmt.Errorf("decode snapshot manifest: %w", err)
	}
	if manifest.Schema != manifestSchema || manifest.Repository != upstreamRepository || manifest.Commit != pinnedCommit {
		return fmt.Errorf("snapshot manifest source identity does not match the accepted pin")
	}
	canonicalManifest, err := marshalManifest(manifest)
	if err != nil {
		return err
	}
	if !bytes.Equal(manifestBytes, canonicalManifest) {
		return fmt.Errorf("manifest.json is not in canonical deterministic form")
	}

	actualFiles, err := snapshotFiles(directory)
	if err != nil {
		return err
	}
	manifestPaths := make([]string, 0, len(manifest.Files))
	for index, entry := range manifest.Files {
		if !safeSnapshotPath(entry.Path) || entry.Path == "manifest.json" {
			return fmt.Errorf("manifest file %d has unsafe path %q", index, entry.Path)
		}
		if index > 0 && manifest.Files[index-1].Path >= entry.Path {
			return fmt.Errorf("manifest files are not strictly bytewise path-sorted")
		}
		data, err := os.ReadFile(filepath.Join(directory, filepath.FromSlash(entry.Path)))
		if err != nil {
			return fmt.Errorf("read manifest file %s: %w", entry.Path, err)
		}
		digest := sha256.Sum256(data)
		if entry.Bytes != int64(len(data)) || entry.SHA256 != hex.EncodeToString(digest[:]) {
			return fmt.Errorf("manifest size or SHA256 mismatch for %s", entry.Path)
		}
		manifestPaths = append(manifestPaths, entry.Path)
	}
	wantFiles := append(append([]string(nil), manifestPaths...), "manifest.json")
	sort.Strings(wantFiles)
	if !reflect.DeepEqual(actualFiles, wantFiles) {
		return fmt.Errorf("snapshot file set mismatch: got %v, want %v", actualFiles, wantFiles)
	}

	notice, err := os.ReadFile(filepath.Join(directory, "NOTICE.md"))
	if err != nil || !bytes.Equal(notice, buildNotice(manifest.Commit)) {
		return fmt.Errorf("NOTICE.md does not match the pinned source and license notice")
	}
	english, err := os.ReadFile(filepath.Join(directory, englishSpecification))
	if err != nil {
		return fmt.Errorf("read English OpenAPI: %w", err)
	}
	chinese, err := os.ReadFile(filepath.Join(directory, chineseSpecification))
	if err != nil {
		return fmt.Errorf("read Chinese OpenAPI: %w", err)
	}
	englishDocument, err := parseOpenAPI("English", english)
	if err != nil {
		return err
	}
	chineseDocument, err := parseOpenAPI("Chinese", chinese)
	if err != nil {
		return err
	}
	examples := unionExternalValues(englishDocument.externalValues, chineseDocument.externalValues)
	if len(examples) != expectedExampleCount {
		return fmt.Errorf("snapshot has %d referenced unique examples, want %d", len(examples), expectedExampleCount)
	}
	for _, name := range examples {
		data, err := os.ReadFile(filepath.Join(directory, "examples", name))
		if err != nil {
			return fmt.Errorf("read referenced example %s: %w", name, err)
		}
		if !json.Valid(data) {
			return fmt.Errorf("referenced example %s is not valid JSON", name)
		}
	}
	comparison, err := compareLocales(englishDocument, chineseDocument)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(comparison, manifest.LocaleComparison) {
		return fmt.Errorf("locale comparison differs from manifest: got %#v, want %#v", comparison, manifest.LocaleComparison)
	}
	return nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	var trailing any
	err := decoder.Decode(&trailing)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return errors.New("multiple JSON values")
	}
	return err
}

func parseOpenAPI(locale string, data []byte) (*openAPIDocument, error) {
	var root map[string]any
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse %s OpenAPI YAML: %w", locale, err)
	}
	externalValues := make(map[string]bool)
	if err := validateReferences(root, externalValues); err != nil {
		return nil, fmt.Errorf("validate %s OpenAPI references: %w", locale, err)
	}
	paths, ok := asMap(root["paths"])
	if !ok || len(paths) == 0 {
		return nil, fmt.Errorf("%s OpenAPI does not contain paths", locale)
	}
	operations := make(map[string]operationContract)
	methods := map[string]bool{"get": true, "put": true, "post": true, "delete": true, "patch": true, "head": true, "options": true, "trace": true}
	for _, pathName := range sortedKeys(paths) {
		pathItem, ok := asMap(paths[pathName])
		if !ok {
			return nil, fmt.Errorf("%s OpenAPI path %s is not an object", locale, pathName)
		}
		pathParameters, err := parameterSet(pathItem["parameters"])
		if err != nil {
			return nil, fmt.Errorf("%s OpenAPI path %s parameters: %w", locale, pathName, err)
		}
		for _, method := range sortedKeys(pathItem) {
			if !methods[strings.ToLower(method)] {
				continue
			}
			operation, ok := asMap(pathItem[method])
			if !ok {
				return nil, fmt.Errorf("%s OpenAPI %s %s is not an object", locale, strings.ToUpper(method), pathName)
			}
			operationID, ok := operation["operationId"].(string)
			if !ok || operationID == "" {
				return nil, fmt.Errorf("%s OpenAPI %s %s lacks operationId", locale, strings.ToUpper(method), pathName)
			}
			parameters, err := parameterSet(operation["parameters"])
			if err != nil {
				return nil, fmt.Errorf("%s OpenAPI %s %s parameters: %w", locale, strings.ToUpper(method), pathName, err)
			}
			parameters = append(parameters, pathParameters...)
			sort.Strings(parameters)
			responses, err := responseSet(operation["responses"])
			if err != nil {
				return nil, fmt.Errorf("%s OpenAPI %s %s responses: %w", locale, strings.ToUpper(method), pathName, err)
			}
			structure, err := structuralDigest(operation)
			if err != nil {
				return nil, fmt.Errorf("%s OpenAPI %s %s structure: %w", locale, strings.ToUpper(method), pathName, err)
			}
			key := strings.ToUpper(method) + " " + pathName
			operations[key] = operationContract{
				OperationID: operationID,
				Deprecated:  boolValue(operation["deprecated"]),
				Parameters:  parameters,
				Responses:   responses,
				Structure:   structure,
			}
		}
	}
	return &openAPIDocument{root: root, operations: operations, externalValues: externalValues}, nil
}

func validateReferences(value any, externalValues map[string]bool) error {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if key == "$ref" {
				reference, ok := child.(string)
				if !ok || !strings.HasPrefix(reference, "#/") {
					return fmt.Errorf("remote or malformed schema reference %q is forbidden", child)
				}
			}
			if key == "externalValue" {
				reference, ok := child.(string)
				if !ok || !strings.HasPrefix(reference, "./examples/") {
					return fmt.Errorf("remote or malformed example reference %q is forbidden", child)
				}
				name := strings.TrimPrefix(reference, "./examples/")
				if path.Base(name) != name || !strings.HasSuffix(name, ".json") || name == "" {
					return fmt.Errorf("unsafe example reference %q", reference)
				}
				externalValues[name] = true
			}
			if err := validateReferences(child, externalValues); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range typed {
			if err := validateReferences(child, externalValues); err != nil {
				return err
			}
		}
	}
	return nil
}

func compareLocales(english, chinese *openAPIDocument) (localeComparison, error) {
	englishKeys := sortedKeys(english.operations)
	chineseKeys := sortedKeys(chinese.operations)
	if !reflect.DeepEqual(englishKeys, chineseKeys) {
		return localeComparison{}, fmt.Errorf("locale path/method inventories differ")
	}
	for _, key := range englishKeys {
		left, right := english.operations[key], chinese.operations[key]
		if left.OperationID != right.OperationID {
			return localeComparison{}, fmt.Errorf("locale operationId differs for %s: %s != %s", key, left.OperationID, right.OperationID)
		}
		if left.Deprecated != right.Deprecated {
			return localeComparison{}, fmt.Errorf("locale lifecycle differs for %s", key)
		}
		if !reflect.DeepEqual(left.Parameters, right.Parameters) {
			return localeComparison{}, fmt.Errorf("locale parameter sets differ for %s: %v != %v", key, left.Parameters, right.Parameters)
		}
		if !reflect.DeepEqual(left.Responses, right.Responses) {
			return localeComparison{}, fmt.Errorf("locale response sets differ for %s: %v != %v", key, left.Responses, right.Responses)
		}
	}
	differences := make([]string, 0)
	for _, key := range englishKeys {
		if english.operations[key].Structure != chinese.operations[key].Structure {
			differences = append(differences, "operation "+key)
		}
	}
	leftComponents, _ := asMap(english.root["components"])
	rightComponents, _ := asMap(chinese.root["components"])
	for _, category := range []string{"parameters", "responses", "schemas"} {
		leftItems, _ := asMap(leftComponents[category])
		rightItems, _ := asMap(rightComponents[category])
		for _, name := range unionKeys(leftItems, rightItems) {
			left, leftExists := leftItems[name]
			right, rightExists := rightItems[name]
			if !leftExists || !rightExists {
				differences = append(differences, "components."+category+"."+name)
				continue
			}
			leftDigest, err := structuralDigest(left)
			if err != nil {
				return localeComparison{}, fmt.Errorf("digest English components.%s.%s: %w", category, name, err)
			}
			rightDigest, err := structuralDigest(right)
			if err != nil {
				return localeComparison{}, fmt.Errorf("digest Chinese components.%s.%s: %w", category, name, err)
			}
			if leftDigest != rightDigest {
				differences = append(differences, "components."+category+"."+name)
			}
		}
	}
	sort.Strings(differences)
	return localeComparison{OperationCount: len(englishKeys), SchemaDifferences: differences}, nil
}

func parameterSet(value any) ([]string, error) {
	if value == nil {
		return nil, nil
	}
	parameters, ok := asSlice(value)
	if !ok {
		return nil, fmt.Errorf("parameter list is not an array")
	}
	result := make([]string, 0, len(parameters))
	for _, item := range parameters {
		parameter, ok := asMap(item)
		if !ok {
			return nil, fmt.Errorf("parameter is not an object")
		}
		if reference, ok := parameter["$ref"].(string); ok {
			result = append(result, "ref:"+reference)
			continue
		}
		name, nameOK := parameter["name"].(string)
		location, locationOK := parameter["in"].(string)
		if !nameOK || !locationOK {
			return nil, fmt.Errorf("inline parameter lacks name or in")
		}
		result = append(result, fmt.Sprintf("%s:%s:required=%t", location, name, boolValue(parameter["required"])))
	}
	sort.Strings(result)
	return result, nil
}

func responseSet(value any) ([]string, error) {
	responses, ok := asMap(value)
	if !ok || len(responses) == 0 {
		return nil, fmt.Errorf("responses are not a non-empty object")
	}
	result := make([]string, 0)
	for _, status := range sortedKeys(responses) {
		response, ok := asMap(responses[status])
		if !ok {
			return nil, fmt.Errorf("response %s is not an object", status)
		}
		if reference, ok := response["$ref"].(string); ok {
			result = append(result, status+":ref:"+reference)
			continue
		}
		content, _ := asMap(response["content"])
		if len(content) == 0 {
			result = append(result, status)
			continue
		}
		for _, mediaType := range sortedKeys(content) {
			result = append(result, status+":"+mediaType)
		}
	}
	return result, nil
}

func structuralDigest(value any) (string, error) {
	canonical, err := json.Marshal(stripDocumentation(value))
	if err != nil {
		return "", fmt.Errorf("encode structural form: %w", err)
	}
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:]), nil
}

func stripDocumentation(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any)
		for key, child := range typed {
			switch key {
			case "description", "summary", "title", "externalDocs", "example", "examples":
				continue
			}
			result[key] = stripDocumentation(child)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, child := range typed {
			result[index] = stripDocumentation(child)
		}
		return result
	default:
		return typed
	}
}

func buildManifest(directory, commit string, comparison localeComparison) (snapshotManifest, error) {
	paths, err := snapshotFiles(directory)
	if err != nil {
		return snapshotManifest{}, err
	}
	files := make([]manifestFile, 0, len(paths))
	for _, relative := range paths {
		if relative == "manifest.json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(directory, filepath.FromSlash(relative)))
		if err != nil {
			return snapshotManifest{}, fmt.Errorf("read staged snapshot file %s: %w", relative, err)
		}
		digest := sha256.Sum256(data)
		files = append(files, manifestFile{Path: relative, Bytes: int64(len(data)), SHA256: hex.EncodeToString(digest[:])})
	}
	return snapshotManifest{Schema: manifestSchema, Repository: upstreamRepository, Commit: commit, Files: files, LocaleComparison: comparison}, nil
}

func marshalManifest(manifest snapshotManifest) ([]byte, error) {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode snapshot manifest: %w", err)
	}
	return append(data, '\n'), nil
}

func snapshotFiles(directory string) ([]string, error) {
	files := make([]string, 0)
	err := filepath.WalkDir(directory, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("snapshot entry %s is not a regular file", filePath)
		}
		relative, err := filepath.Rel(directory, filePath)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk snapshot: %w", err)
	}
	sort.Strings(files)
	return files, nil
}

func buildNotice(commit string) []byte {
	text := "# QWeather OpenAPI snapshot notice\n\n" +
		"- Creator: QWeather (和风天气)\n" +
		"- Source repository: <" + upstreamRepository + ">\n" +
		"- Pinned commit: [`" + commit + "`](" + upstreamRepository + "/commit/" + commit + ")\n\n" +
		"This directory redistributes unchanged documentation content from the pinned\n" +
		"commit: the English and Chinese OpenAPI YAML files and the 53 JSON examples\n" +
		"referenced by those files. QWeather identifies repository content as licensed\n" +
		"under [Creative Commons Attribution 4.0 International (CC BY 4.0)](https://creativecommons.org/licenses/by/4.0/).\n\n" +
		"Pinned source files:\n\n" +
		"- [English OpenAPI](" + upstreamRepository + "/blob/" + commit + "/" + upstreamOpenAPIRoot + "/" + englishSpecification + ")\n" +
		"- [Chinese OpenAPI](" + upstreamRepository + "/blob/" + commit + "/" + upstreamOpenAPIRoot + "/" + chineseSpecification + ")\n" +
		"- [Referenced examples](" + upstreamRepository + "/tree/" + commit + "/" + upstreamOpenAPIRoot + "/examples)\n\n" +
		"`manifest.json` records the path, size, and SHA256 of every other file in this\n" +
		"directory. It is the integrity root and therefore does not list itself.\n\n" +
		"Keep QWeather Attribution with QWeather data and documentation reuse. The\n" +
		"project's Apache-2.0 license does not replace the CC BY 4.0 terms for this\n" +
		"snapshot. Access to QWeather APIs, credentials, services, and returned data is\n" +
		"separately governed by the [QWeather Developers EULA](https://www.qweather.com/terms/developers-eula)\n" +
		"and other current QWeather terms; this documentation snapshot grants no API\n" +
		"service entitlement.\n"
	return []byte(text)
}

func gitOutput(source string, arguments ...string) ([]byte, error) {
	commandArguments := append([]string{"-C", source}, arguments...)
	command := exec.Command("git", commandArguments...)
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w", strings.Join(arguments, " "), err)
	}
	return output, nil
}

func gitFile(source, commit, filePath string) ([]byte, error) {
	data, err := gitOutput(source, "show", commit+":"+filePath)
	if err != nil {
		return nil, fmt.Errorf("read upstream %s at %s: %w", filePath, commit, err)
	}
	return data, nil
}

func normalizeGitURL(value string) string {
	value = strings.TrimSuffix(strings.TrimPrefix(value, "git+"), ".git")
	parsed, err := url.Parse(value)
	if err != nil {
		return value
	}
	parsed.Path = strings.TrimSuffix(parsed.Path, "/")
	return parsed.String()
}

func replaceDirectory(staging, target string) error {
	backup := staging + "-previous"
	targetExists := false
	if _, err := os.Stat(target); err == nil {
		targetExists = true
		if err := os.Rename(target, backup); err != nil {
			return fmt.Errorf("preserve previous snapshot: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect previous snapshot: %w", err)
	}
	if err := os.Rename(staging, target); err != nil {
		if targetExists {
			_ = os.Rename(backup, target)
		}
		return fmt.Errorf("install synchronized snapshot: %w", err)
	}
	if targetExists {
		if err := os.RemoveAll(backup); err != nil {
			return fmt.Errorf("remove previous snapshot backup: %w", err)
		}
	}
	return nil
}

func unionExternalValues(left, right map[string]bool) []string {
	values := make(map[string]bool, len(left)+len(right))
	for name := range left {
		values[name] = true
	}
	for name := range right {
		values[name] = true
	}
	return sortedKeys(values)
}

func unionKeys(left, right map[string]any) []string {
	values := make(map[string]bool, len(left)+len(right))
	for name := range left {
		values[name] = true
	}
	for name := range right {
		values[name] = true
	}
	return sortedKeys(values)
}

func asMap(value any) (map[string]any, bool) {
	typed, ok := value.(map[string]any)
	return typed, ok
}

func asSlice(value any) ([]any, bool) {
	typed, ok := value.([]any)
	return typed, ok
}

func boolValue(value any) bool {
	typed, _ := value.(bool)
	return typed
}

func safeSnapshotPath(value string) bool {
	return value != "" && value == path.Clean(value) && !strings.HasPrefix(value, "../") && !strings.HasPrefix(value, "/") && !strings.Contains(value, "\\")
}
