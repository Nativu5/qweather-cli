package qweather

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Nativu5/qweather-cli/internal/catalog"
)

func TestClassifyPreservesLegacyBodyAndAttribution(t *testing.T) {
	body := []byte(`{"code":"200","now":{"temp":"20","futureField":true},"refer":{"sources":["QWeather"],"license":["license-url"]}}`)
	classified, problem := Classify(catalog.ResponseLegacyV1, Response{StatusCode: 200, Body: body}, "weather.city.current")
	if problem != nil {
		t.Fatal(problem)
	}
	if classified.Outcome != "ok" || string(classified.Data) != string(body) || len(classified.Attribution) != 2 {
		t.Fatalf("classified = %#v", classified)
	}
	var decoded map[string]any
	if err := json.Unmarshal(classified.Data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["now"].(map[string]any)["futureField"] != true {
		t.Fatal("unknown provider field was lost")
	}
}

func TestClassifyNoDataAndModernAttribution(t *testing.T) {
	legacy, problem := Classify(catalog.ResponseLegacyV1, Response{StatusCode: 200, Body: []byte(`{"code":"204"}`)}, "weather.history")
	if problem != nil || legacy.Outcome != "no_data" {
		t.Fatalf("legacy=%#v problem=%v", legacy, problem)
	}
	modern, problem := Classify(catalog.ResponseModernV1, Response{StatusCode: 200, Body: []byte(`{"metadata":{"attributions":[{"name":"QWeather"}]},"alerts":[]}`)}, "alert.current")
	if problem != nil || modern.Outcome != "ok" || len(modern.Attribution) != 1 {
		t.Fatalf("modern=%#v problem=%v", modern, problem)
	}
	empty, problem := Classify(catalog.ResponseModernV1, Response{StatusCode: 204}, "air.current")
	if problem != nil || empty.Outcome != "no_data" || string(empty.Data) != `{}` {
		t.Fatalf("empty=%#v problem=%v", empty, problem)
	}
}

func TestClassifyMapsProviderFailures(t *testing.T) {
	tests := []struct {
		name      string
		family    catalog.ResponseFamily
		response  Response
		exitCode  int
		code      string
		retryable bool
	}{
		{"legacy rejection", catalog.ResponseLegacyV1, Response{StatusCode: 200, Body: []byte(`{"code":"403"}`)}, 6, "UPSTREAM_REJECTED", false},
		{"rate limit", catalog.ResponseModernV1, Response{StatusCode: 429, Body: []byte(`{"status":429}`)}, 7, "RATE_LIMITED", true},
		{"server failure", catalog.ResponseModernV1, Response{StatusCode: 503, Body: []byte(`{"status":503}`)}, 8, "UPSTREAM_UNAVAILABLE", true},
		{"malformed", catalog.ResponseModernV1, Response{StatusCode: 200, Body: []byte(`not-json`)}, 9, "UPSTREAM_PROTOCOL_ERROR", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, problem := Classify(test.family, test.response, "test.capability")
			if problem == nil || problem.ExitCode != test.exitCode || problem.Code != test.code || problem.Retryable != test.retryable {
				t.Fatalf("problem = %#v", problem)
			}
		})
	}
}

func TestProblemForClientError(t *testing.T) {
	timeout := ProblemForError(&ClientError{Kind: ErrorNetwork, Err: context.DeadlineExceeded}, "air.current")
	if timeout.ExitCode != 8 || timeout.Code != "TIMEOUT" || !timeout.Retryable {
		t.Fatalf("timeout = %#v", timeout)
	}
	oversize := ProblemForError(&ClientError{Kind: ErrorOversize, Err: errors.New("too large")}, "air.current")
	if oversize.ExitCode != 9 || oversize.Code != "UPSTREAM_PROTOCOL_ERROR" {
		t.Fatalf("oversize = %#v", oversize)
	}
}

func TestSafeDebugErrorRedactsHeaderMarkers(t *testing.T) {
	message := SafeDebugError(errors.New("Authorization: Bearer secret X-QW-Api-Key: key-secret"))
	if strings.Contains(message, "secret") || strings.Contains(message, "Bearer ") || strings.Contains(message, "Authorization") || strings.Contains(message, "X-QW-Api-Key") {
		t.Fatalf("message = %q", message)
	}
}
