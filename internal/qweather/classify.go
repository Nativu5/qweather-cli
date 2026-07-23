package qweather

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"

	"github.com/Nativu5/qweather-cli/internal/catalog"
	"github.com/Nativu5/qweather-cli/internal/output"
)

type Classified struct {
	Outcome     string
	Data        json.RawMessage
	Attribution []any
}

func Classify(family catalog.ResponseFamily, response Response, capabilityID string) (Classified, *output.Problem) {
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return Classified{}, upstreamProblem(response.StatusCode, response.Body, capabilityID)
	}
	if response.StatusCode == http.StatusNoContent && len(bytes.TrimSpace(response.Body)) == 0 {
		return Classified{Outcome: "no_data", Data: json.RawMessage(`{}`), Attribution: []any{}}, nil
	}
	data, object, problem := decodeObject(response.Body, capabilityID)
	if problem != nil {
		return Classified{}, problem
	}
	if family == catalog.ResponseLegacyV1 {
		code, ok := object["code"].(string)
		if !ok || code == "" {
			return Classified{}, protocolProblem(capabilityID, "legacy response does not contain a string code")
		}
		if code != "200" && code != "204" {
			return Classified{}, legacyCodeProblem(code, capabilityID)
		}
		outcome := "ok"
		if code == "204" {
			outcome = "no_data"
		}
		return Classified{Outcome: outcome, Data: data, Attribution: extractAttribution(object)}, nil
	}
	if family != catalog.ResponseModernV1 && family != catalog.ResponseConsoleV1 {
		return Classified{}, protocolProblem(capabilityID, "response family is not recognized")
	}
	return Classified{Outcome: "ok", Data: data, Attribution: extractAttribution(object)}, nil
}

func ProblemForError(err error, capabilityID string) *output.Problem {
	var clientError *ClientError
	if errors.As(err, &clientError) {
		switch clientError.Kind {
		case ErrorOversize, ErrorProtocol:
			problem := output.NewProblem(9, "UPSTREAM_PROTOCOL_ERROR", "provider response or request violated the protocol contract")
			problem.Capability = capabilityID
			problem.Cause = err
			return problem
		case ErrorNetwork:
			code := "NETWORK_ERROR"
			message := "provider request failed"
			if errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
				code = "TIMEOUT"
				message = "provider request timed out"
			}
			problem := output.NewProblem(8, code, message)
			problem.Capability = capabilityID
			problem.Retryable = true
			problem.Cause = err
			return problem
		}
	}
	problem := output.NewProblem(10, "INTERNAL_ERROR", "unexpected provider client failure")
	problem.Capability = capabilityID
	problem.Cause = err
	return problem
}

func decodeObject(body []byte, capabilityID string) (json.RawMessage, map[string]any, *output.Problem) {
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, nil, protocolProblem(capabilityID, "provider returned an empty JSON body")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var object map[string]any
	if err := decoder.Decode(&object); err != nil {
		return nil, nil, protocolProblem(capabilityID, "provider returned malformed JSON")
	}
	if object == nil {
		return nil, nil, protocolProblem(capabilityID, "provider response must be a JSON object")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, nil, protocolProblem(capabilityID, "provider returned multiple JSON values")
	}
	return append(json.RawMessage(nil), body...), object, nil
}

func upstreamProblem(status int, body []byte, capabilityID string) *output.Problem {
	code := "UPSTREAM_REJECTED"
	message := "provider rejected the request"
	exitCode := 6
	retryable := false
	if status == http.StatusTooManyRequests {
		code = "RATE_LIMITED"
		message = "provider rate or usage limit reached"
		exitCode = 7
		retryable = true
	} else if status >= 500 {
		code = "UPSTREAM_UNAVAILABLE"
		message = "provider is temporarily unavailable"
		exitCode = 8
		retryable = true
	}
	problem := output.NewProblem(exitCode, code, message)
	problem.Capability = capabilityID
	problem.Retryable = retryable
	details := map[string]any{"httpStatus": status}
	var object map[string]any
	if json.Unmarshal(body, &object) == nil {
		for _, key := range []string{"code", "status", "title"} {
			if value, exists := object[key]; exists {
				details[key] = value
			}
		}
	}
	problem.Details = details
	return problem
}

func legacyCodeProblem(code, capabilityID string) *output.Problem {
	status := 0
	_, _ = fmt.Sscanf(code, "%d", &status)
	return upstreamProblem(status, nil, capabilityID)
}

func protocolProblem(capabilityID, reason string) *output.Problem {
	problem := output.NewProblem(9, "UPSTREAM_PROTOCOL_ERROR", reason)
	problem.Capability = capabilityID
	return problem
}

func extractAttribution(object map[string]any) []any {
	result := make([]any, 0)
	if metadata, ok := object["metadata"].(map[string]any); ok {
		if values, ok := metadata["attributions"].([]any); ok {
			result = append(result, values...)
		}
	}
	if refer, ok := object["refer"].(map[string]any); ok {
		if values, ok := refer["sources"].([]any); ok {
			for _, value := range values {
				result = append(result, map[string]any{"source": value})
			}
		}
		if values, ok := refer["license"].([]any); ok {
			for _, value := range values {
				result = append(result, map[string]any{"license": value})
			}
		}
	}
	return result
}

func isTimeout(err error) bool {
	var networkError net.Error
	return errors.As(err, &networkError) && networkError.Timeout()
}

func SafeDebugError(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)Authorization\s*:\s*[^\s]+\s+[^\s]+`),
		regexp.MustCompile(`(?i)Bearer\s+[^\s]+`),
		regexp.MustCompile(`(?i)X-QW-Api-Key\s*:\s*[^\s]+`),
	}
	for _, pattern := range patterns {
		message = pattern.ReplaceAllString(message, "[redacted]")
	}
	return message
}
