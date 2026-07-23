package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

const (
	ResultSchema  = "qweather.result/v1"
	ProblemSchema = "qweather.problem/v1"
)

type ResolvedPlace struct {
	ID      string `json:"id,omitempty"`
	Name    string `json:"name,omitempty"`
	Adm1    string `json:"adm1,omitempty"`
	Adm2    string `json:"adm2,omitempty"`
	Country string `json:"country,omitempty"`
	Lat     string `json:"lat,omitempty"`
	Lon     string `json:"lon,omitempty"`
	TZ      string `json:"tz,omitempty"`
}

type Policy struct {
	BillingGroup string `json:"billingGroup"`
}

type Cache struct {
	Status            string `json:"status"`
	StoredAt          string `json:"storedAt,omitempty"`
	ExpiresAt         string `json:"expiresAt,omitempty"`
	AgeSeconds        int64  `json:"ageSeconds,omitempty"`
	UpstreamRequested bool   `json:"upstreamRequested"`
}

type Upstream struct {
	HTTPStatus     int    `json:"httpStatus"`
	ResponseFamily string `json:"responseFamily"`
}

type Result struct {
	Schema        string          `json:"schema"`
	Outcome       string          `json:"outcome"`
	Capability    string          `json:"capability"`
	ResolvedPlace *ResolvedPlace  `json:"resolvedPlace,omitempty"`
	Operations    []string        `json:"operations"`
	Policy        Policy          `json:"policy"`
	Cache         Cache           `json:"cache"`
	Upstream      Upstream        `json:"upstream"`
	Attribution   []any           `json:"attribution"`
	Data          json.RawMessage `json:"data"`
	ProviderBody  []byte          `json:"-"`
}

type Problem struct {
	Schema     string `json:"schema"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	Capability string `json:"capability,omitempty"`
	Retryable  bool   `json:"retryable"`
	Details    any    `json:"details,omitempty"`
	ExitCode   int    `json:"-"`
	Cause      error  `json:"-"`
}

func NewProblem(exitCode int, code, message string) *Problem {
	return &Problem{Schema: ProblemSchema, Code: code, Message: message, ExitCode: exitCode}
}

func (p *Problem) Error() string {
	return p.Message
}

func (p *Problem) Unwrap() error {
	return p.Cause
}

func RenderResult(writer io.Writer, result *Result, bodyOnly, pretty bool) error {
	if result == nil {
		return fmt.Errorf("result is nil")
	}
	if bodyOnly {
		body := result.ProviderBody
		if pretty {
			var formatted bytes.Buffer
			if err := json.Indent(&formatted, body, "", "  "); err != nil {
				return fmt.Errorf("format provider body: %w", err)
			}
			body = formatted.Bytes()
		}
		if _, err := writer.Write(body); err != nil {
			return err
		}
		if len(body) == 0 || body[len(body)-1] != '\n' {
			_, err := io.WriteString(writer, "\n")
			return err
		}
		return nil
	}
	return writeJSON(writer, result, pretty)
}

func RenderProblem(writer io.Writer, problem *Problem, pretty bool) error {
	if problem == nil {
		problem = NewProblem(10, "INTERNAL_ERROR", "missing problem details")
	}
	if problem.Schema == "" {
		problem.Schema = ProblemSchema
	}
	return writeJSON(writer, problem, pretty)
}

func WriteJSON(writer io.Writer, value any, pretty bool) error {
	return writeJSON(writer, value, pretty)
}

func writeJSON(writer io.Writer, value any, pretty bool) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	if pretty {
		encoder.SetIndent("", "  ")
	}
	return encoder.Encode(value)
}
