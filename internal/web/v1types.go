package web

// Request and answer bodies of v1 routes that are small enough to be a pair of
// fields. They are named so that the OpenAPI description is built from the same
// types the handlers decode and encode.

type v1ApplySetRequest struct {
	Set    string   `json:"set"`
	Theme  string   `json:"theme"`
	Panels []string `json:"panels"`
}

type v1Applied struct {
	Applied []string `json:"applied"`
}

type v1Scene struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

type v1ActivateRequest struct {
	Persist bool `json:"persist"`
}

type v1EnabledRequest struct {
	Enabled *bool `json:"enabled"`
}

type v1Enabled struct {
	Enabled bool `json:"enabled"`
}

type v1PowerRequest struct {
	On *bool `json:"on"`
}

type v1PowerState struct {
	Power bool `json:"power"`
}

type v1GamingRequest struct {
	Active       *bool  `json:"active"`
	Game         string `json:"game"`
	PID          int    `json:"pid"`
	LeaseSeconds *int   `json:"leaseSeconds"`
}

type v1GamingState struct {
	Active bool   `json:"active"`
	Source string `json:"source"`
	Game   string `json:"game"`
}

type v1CommunityEntry struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Author string `json:"author"`
	Set    string `json:"set"`
	Theme  string `json:"theme"`
}

type v1ID struct {
	ID string `json:"id"`
}

// v1PairRequest is the body of POST /pair.
type v1PairRequest struct {
	Name      string   `json:"name"`
	Scopes    []string `json:"scopes"`
	Interface string   `json:"interface"`
}

// v1PairAccepted is the answer to POST /pair.
type v1PairAccepted struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	ExpiresIn int    `json:"expiresIn"`
	Poll      string `json:"poll"`
}

// v1TokenInfo is the answer of GET /token.
type v1TokenInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Scope string `json:"scope"` // read or control
	// System: the program may also quit and update CaseHUD (separate from scope).
	System    bool   `json:"system"`
	Kind      string `json:"kind"`
	Interface string `json:"interface"`
}

// v1Revision is the answer of the changes of the cooling configuration.
type v1Revision struct {
	Revision int64 `json:"revision"`
}

// v1ModeChanged is the answer of PUT /cooling/mode.
type v1ModeChanged struct {
	Revision int64  `json:"revision"`
	Mode     string `json:"mode"`
}

// v1MeasureRequest is the body of POST /cooling/measure.
type v1MeasureRequest struct {
	ID      string `json:"id"`
	All     bool   `json:"all"`
	Confirm bool   `json:"confirm"`
}

// v1MeasurePumpRequest is the body of POST /cooling/measure-pump.
type v1MeasurePumpRequest struct {
	ID      string `json:"id"`
	Confirm bool   `json:"confirm"`
}

// v1IDRequest is a body that names one output or preset.
type v1IDRequest struct {
	ID string `json:"id"`
}

// v1NameRequest is the body that names a preset.
type v1NameRequest struct {
	Name string `json:"name"`
}

// v1ModeRequest is the body of PUT /cooling/mode.
type v1ModeRequest struct {
	Mode string `json:"mode"`
}
