package catalog

import "time"

type Lifecycle string

const (
	LifecycleCurrent    Lifecycle = "current"
	LifecycleDeprecated Lifecycle = "deprecated"
)

type BillingGroup string

const (
	BillingBasic  BillingGroup = "basic"
	BillingMarine BillingGroup = "marine"
	BillingSolar  BillingGroup = "solar"
)

type ProductGate string

const (
	GateNone             ProductGate = "none"
	GateMarine           ProductGate = "marine"
	GateSolar            ProductGate = "solar"
	GateSensitiveAccount ProductGate = "sensitive-account"
)

type TargetKind string

const (
	TargetNone        TargetKind = "none"
	TargetGeoLookup   TargetKind = "geo-lookup"
	TargetPlace       TargetKind = "place"
	TargetLocationID  TargetKind = "location-id"
	TargetCoordinate  TargetKind = "coordinate"
	TargetAirStation  TargetKind = "air-station"
	TargetTideStation TargetKind = "tide-station"
	TargetStorm       TargetKind = "storm"
)

type ResponseFamily string

const (
	ResponseCodeReferV1 ResponseFamily = "code-refer-v1"
	ResponseMetadataV1  ResponseFamily = "metadata-v1"
	ResponseConsoleV1   ResponseFamily = "console-v1"
)

type CacheMode string

const (
	CacheDisabled  CacheMode = "disabled"
	CacheEnabled   CacheMode = "enabled"
	CacheSensitive CacheMode = "sensitive"
)

type CacheBoundary string

const (
	BoundaryNone      CacheBoundary = "none"
	BoundaryLocalHour CacheBoundary = "local-hour"
	BoundaryLocalDay  CacheBoundary = "local-day"
	BoundaryUTCHour   CacheBoundary = "utc-hour"
	BoundaryUTCDay    CacheBoundary = "utc-day"
)

type CachePolicy struct {
	Mode        CacheMode     `json:"mode"`
	TTL         time.Duration `json:"ttl"`
	InactiveTTL time.Duration `json:"inactiveTtl,omitempty"`
	Boundary    CacheBoundary `json:"boundary"`
	Evidence    string        `json:"evidence"`
}

type Upstream struct {
	Method         string         `json:"method"`
	PathTemplate   string         `json:"pathTemplate"`
	ResponseFamily ResponseFamily `json:"responseFamily"`
}

type FlagKind string

const (
	FlagString      FlagKind = "string"
	FlagStringSlice FlagKind = "string-slice"
	FlagInt         FlagKind = "int"
	FlagIntSlice    FlagKind = "int-slice"
	FlagFloat       FlagKind = "float"
	FlagBool        FlagKind = "bool"
)

type Flag struct {
	Name     string   `json:"name"`
	Usage    string   `json:"usage"`
	Kind     FlagKind `json:"kind"`
	Required bool     `json:"required,omitempty"`
	Enum     []string `json:"enum,omitempty"`
	IntEnum  []int    `json:"intEnum,omitempty"`
	Min      *float64 `json:"min,omitempty"`
	Max      *float64 `json:"max,omitempty"`
	Default  string   `json:"default,omitempty"`
}

type Capability struct {
	ID              string       `json:"id"`
	CommandPath     string       `json:"commandPath,omitempty"`
	Domain          string       `json:"domain"`
	Summary         string       `json:"summary"`
	DocsURL         string       `json:"docsUrl"`
	Lifecycle       Lifecycle    `json:"lifecycle"`
	Replacement     string       `json:"replacement,omitempty"`
	Target          TargetKind   `json:"target"`
	Flags           []Flag       `json:"flags,omitempty"`
	Upstream        Upstream     `json:"upstream"`
	BillingGroup    BillingGroup `json:"billingGroup"`
	ProductGate     ProductGate  `json:"productGate"`
	Cache           CachePolicy  `json:"cache"`
	RequestRevision uint         `json:"requestRevision"`
}

// Input contains parsed public command values. A capability exposes only the
// subset named by its registry record.
type Input struct {
	Place           string
	PlaceID         string
	Coordinate      string
	Country         string
	Adm             string
	Query           string
	POIType         string
	Limit           int
	Language        string
	Unit            string
	Days            int
	Hours           int
	Indices         []int
	AllIndices      bool
	Date            string
	LocalTime       bool
	AirStationID    string
	StormID         string
	Year            int
	TideStationID   string
	IntervalMinutes int
	Includes        []string
	TiltDegrees     int
	AzimuthDegrees  int
	At              string
	AltitudeMeters  float64
	ProjectID       string
	CredentialID    string
}
