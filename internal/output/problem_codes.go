package output

// ProblemCode is a stable qweather.problem/v1 decision code.
type ProblemCode string

const (
	CodeInvalidInvocation        ProblemCode = "INVALID_INVOCATION"
	CodeUnknownCapability        ProblemCode = "UNKNOWN_CAPABILITY"
	CodeConfigInvalid            ProblemCode = "CONFIG_INVALID"
	CodeProductGateRequired      ProblemCode = "PRODUCT_GATE_REQUIRED"
	CodeAmbiguousPlace           ProblemCode = "AMBIGUOUS_PLACE"
	CodePlaceNotFound            ProblemCode = "PLACE_NOT_FOUND"
	CodeUpstreamRejected         ProblemCode = "UPSTREAM_REJECTED"
	CodeRateLimited              ProblemCode = "RATE_LIMITED"
	CodeNetworkError             ProblemCode = "NETWORK_ERROR"
	CodeTimeout                  ProblemCode = "TIMEOUT"
	CodeUpstreamUnavailable      ProblemCode = "UPSTREAM_UNAVAILABLE"
	CodeUpstreamProtocolError    ProblemCode = "UPSTREAM_PROTOCOL_ERROR"
	CodeCacheIOError             ProblemCode = "CACHE_IO_ERROR"
	CodeCapabilityNotImplemented ProblemCode = "CAPABILITY_NOT_IMPLEMENTED"
	CodeControlNotImplemented    ProblemCode = "CONTROL_NOT_IMPLEMENTED"
	CodeRegistryInvalid          ProblemCode = "REGISTRY_INVALID"
	CodeCommandTreeInvalid       ProblemCode = "COMMAND_TREE_INVALID"
	CodeInternalError            ProblemCode = "INTERNAL_ERROR"
	CodeOutputError              ProblemCode = "OUTPUT_ERROR"
)

// ProblemDefinition describes one stable Machine Problem decision code.
type ProblemDefinition struct {
	Code      ProblemCode
	ExitCode  int
	Retryable bool
	Meaning   string
}

var problemDefinitions = [...]ProblemDefinition{
	{Code: CodeInvalidInvocation, ExitCode: 2, Meaning: "Invocation syntax, typed input, or local validation failed."},
	{Code: CodeUnknownCapability, ExitCode: 2, Meaning: "The requested Capability ID is not in the compiled catalog."},
	{Code: CodeConfigInvalid, ExitCode: 3, Meaning: "Configuration or authentication material is invalid."},
	{Code: CodeProductGateRequired, ExitCode: 4, Meaning: "A billed product or sensitive-output acknowledgement is missing."},
	{Code: CodeAmbiguousPlace, ExitCode: 5, Meaning: "A Place Spec matched multiple safe candidates."},
	{Code: CodePlaceNotFound, ExitCode: 5, Meaning: "A Place Spec did not match a QWeather place."},
	{Code: CodeUpstreamRejected, ExitCode: 6, Meaning: "QWeather permanently rejected the request."},
	{Code: CodeRateLimited, ExitCode: 7, Retryable: true, Meaning: "A rate or usage limit was reached."},
	{Code: CodeNetworkError, ExitCode: 8, Retryable: true, Meaning: "The provider request failed at the network boundary."},
	{Code: CodeTimeout, ExitCode: 8, Retryable: true, Meaning: "The provider request exceeded its deadline."},
	{Code: CodeUpstreamUnavailable, ExitCode: 8, Retryable: true, Meaning: "QWeather returned a temporary server failure."},
	{Code: CodeUpstreamProtocolError, ExitCode: 9, Meaning: "The provider response or request violated the protocol contract."},
	{Code: CodeCacheIOError, ExitCode: 10, Meaning: "A persistent cache operation failed."},
	{Code: CodeCapabilityNotImplemented, ExitCode: 10, Meaning: "A compiled Capability mapping is unavailable."},
	{Code: CodeCommandTreeInvalid, ExitCode: 10, Meaning: "The compiled command tree could not be constructed."},
	{Code: CodeControlNotImplemented, ExitCode: 10, Meaning: "A compiled local control operation is unavailable."},
	{Code: CodeInternalError, ExitCode: 10, Meaning: "A broken internal invariant was detected."},
	{Code: CodeOutputError, ExitCode: 10, Meaning: "Writing the selected output failed."},
	{Code: CodeRegistryInvalid, ExitCode: 10, Meaning: "The compiled Capability registry is invalid or cannot be hashed."},
}

// ProblemDefinitions returns the complete stable problem-code catalog in
// deterministic exit-code and code order.
func ProblemDefinitions() []ProblemDefinition {
	return append([]ProblemDefinition(nil), problemDefinitions[:]...)
}
