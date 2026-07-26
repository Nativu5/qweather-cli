package output

import (
	"encoding/json"
	"fmt"
	"io"
)

const (
	ResultSchema  = "qweather.result/v1"
	ProblemSchema = "qweather.problem/v1"
)

// Mode selects the process presentation without changing request semantics.
type Mode string

const (
	ModeText Mode = "text"
	ModeJSON Mode = "json"
	ModeBody Mode = "body"
)

func (m Mode) Valid() bool {
	return m == ModeText || m == ModeJSON || m == ModeBody
}

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
	Unit          string          `json:"-"`
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

func NewProblem(exitCode int, code ProblemCode, message string) *Problem {
	return &Problem{Schema: ProblemSchema, Code: string(code), Message: message, ExitCode: exitCode}
}

func (p *Problem) Error() string {
	return p.Message
}

func (p *Problem) Unwrap() error {
	return p.Cause
}

// WriteJSON emits one compact JSON value followed by a newline.
func WriteJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

// RenderProblem writes a QWeather-owned problem using the selected mode. Body
// mode deliberately shares Text presentation because provider error bodies are
// never exposed.
func RenderProblem(writer io.Writer, problem *Problem, mode Mode) error {
	if problem == nil {
		problem = NewProblem(10, CodeInternalError, "missing problem details")
	}
	if problem.Schema == "" {
		problem.Schema = ProblemSchema
	}
	if mode == ModeJSON {
		return WriteJSON(writer, problem)
	}
	return writeTextProblem(writer, problem)
}

// RenderCobraError preserves Cobra's ordinary text diagnostic boundary.
func RenderCobraError(writer io.Writer, err error) error {
	if err == nil {
		return nil
	}
	_, writeErr := fmt.Fprintf(writer, "Error: %v\n", err)
	return writeErr
}
