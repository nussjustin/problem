// Package problem implements the RFC 9457 problem details specification in Go.
//
// It also provides some functionality for directly responding to HTTP requests with problems
// and for defining reusable problem types.
package problem

import (
	"cmp"
	"maps"
	"net/http"

	"github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
)

const (
	// AboutBlankTypeURI is the default problem type and is equivalent to not specifying a problem type.
	//
	// See also https://datatracker.ietf.org/doc/html/rfc9457#name-aboutblank
	AboutBlankTypeURI = "about:blank"
)

const (
	// ContentType is the media type used for problem responses, as defined by IANA.
	//
	// See also https://datatracker.ietf.org/doc/html/rfc9457#name-iana-considerations
	ContentType = "application/problem+json"
)

// Details defines an RFC 9457 problem details object.
//
// Details also implements the [error] interface and can optionally wrap an existing [error] value.
type Details struct {
	// Type contains the problem type as a URI.
	//
	// If empty, this is the same as "about:blank". See [AboutBlankTypeURI] for more information.
	//
	// See also https://datatracker.ietf.org/doc/html/rfc9457#name-type
	Type string

	// Status is indicating the HTTP status code generated for this occurrence of the problem.
	//
	// This should be the same code as used for the HTTP response and is only advisory.
	//
	// See also https://datatracker.ietf.org/doc/html/rfc9457#name-status
	Status int

	// Title is string containing a short, human-readable summary of the problem type
	//
	// See also https://datatracker.ietf.org/doc/html/rfc9457#name-title
	Title string

	// Detail is string containing a human-readable explanation specific to this occurrence of the problem.
	//
	// See also https://datatracker.ietf.org/doc/html/rfc9457#name-detail
	Detail string

	// Instance is string containing a URI reference that identifies the specific occurrence of the problem
	//
	// See also https://datatracker.ietf.org/doc/html/rfc9457#name-instance
	Instance string

	// Extensions contains any extensions that should be added to the response.
	//
	// If the problem was parsed from a JSON response this will include all extension fields.
	//
	// See also https://datatracker.ietf.org/doc/html/rfc9457#name-extension-members
	Extensions map[string]any

	// Underlying optionally contains the underlying error that lead to / is described by this problem.
	//
	// This field is not part of RFC 9457 and is neither included in generated JSON nor populated during unmarshaling.
	Underlying error
}

// Option defines functional options that can be used to fill in optional values when creating a [Details] via
// [New] or via [Type.Details].
type Option func(*Details)

// WithStatus sets the Status for a new Details value.
func WithStatus(status int) Option {
	return func(d *Details) {
		d.Status = status
	}
}

// WithDetail sets the Detail for a new Details value.
func WithDetail(detail string) Option {
	return func(d *Details) {
		d.Detail = detail
	}
}

// WithInstance sets the Instance for a new Details value.
func WithInstance(instance string) Option {
	return func(d *Details) {
		d.Instance = instance
	}
}

// WithExtension adds the given key-value pair to the Extensions of a new Details value.
func WithExtension(key string, value any) Option {
	return func(d *Details) {
		if d.Extensions == nil {
			d.Extensions = make(map[string]any)
		}
		d.Extensions[key] = value
	}
}

// WithExtensions adds the values to the Extensions of a new Details value.
func WithExtensions(extensions map[string]any) Option {
	return func(d *Details) {
		if d.Extensions == nil {
			d.Extensions = make(map[string]any, len(extensions))
		}
		maps.Copy(d.Extensions, extensions)
	}
}

// WithUnderlying sets the given value as the underlying error of a new Details value.
func WithUnderlying(err error) Option {
	return func(d *Details) {
		d.Underlying = err
	}
}

// New returns a new Details instance using the given type, status and title.
//
// It is also possible to set the Detail and Instance fields as well as extensions by
// providing one or more [Option] values.
//
// Most users should prefer creating a Details instance via a struct literal or using [Type.Details] instead.
func New(typ string, title string, status int, opts ...Option) *Details {
	p := &Details{
		Type:   typ,
		Status: status,
		Title:  title,
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

var _ error = (*Details)(nil)

// Error implements the error interface. The returned value is the same as d.Title.
func (d *Details) Error() string {
	return d.Title
}

// Unwrap implements the interface used functions like [errors.Is] and [errors.As] to get the underlying error, if any.
func (d *Details) Unwrap() error {
	return d.Underlying
}

// MarshalJSON implements the json.Marshaler interface.
//
// See MarshalJSONTo for details.
func (d *Details) MarshalJSON() ([]byte, error) {
	// This will call (*Details).MarshalJSONTo.
	return json.Marshal(d)
}

// MarshalJSONTo implements the json.MarshalerTo interface.
//
// If no Type is set, "about:blank" is used. See also [AboutBlankTypeURI].
//
// Extension fields named "type", "status", "title", "detail" or "instance" are ignored when marshaling in favor
// of the respective struct fields even if the field is empty.
func (d *Details) MarshalJSONTo(enc *jsontext.Encoder, opts json.Options) error {
	// We implement marshalling ourselves so that we can put the defined fields and the extensions
	// into a single JSON object.
	//
	// As a nice benefit this is also faster than using the default, reflection-based approach.
	if err := enc.WriteToken(jsontext.ObjectStart); err != nil {
		return err
	}

	typ := cmp.Or(d.Type, AboutBlankTypeURI)

	if d.Type != "" {
		if err := enc.WriteToken(jsontext.String("type")); err != nil {
			return err
		}

		if err := enc.WriteToken(jsontext.String(typ)); err != nil {
			return err
		}
	}

	if d.Status != 0 {
		if err := enc.WriteToken(jsontext.String("status")); err != nil {
			return err
		}

		if err := enc.WriteToken(jsontext.Int(int64(d.Status))); err != nil {
			return err
		}
	}

	if d.Title != "" {
		if err := enc.WriteToken(jsontext.String("title")); err != nil {
			return err
		}

		if err := enc.WriteToken(jsontext.String(d.Title)); err != nil {
			return err
		}
	}

	if d.Detail != "" {
		if err := enc.WriteToken(jsontext.String("detail")); err != nil {
			return err
		}

		if err := enc.WriteToken(jsontext.String(d.Detail)); err != nil {
			return err
		}
	}

	if d.Instance != "" {
		if err := enc.WriteToken(jsontext.String("instance")); err != nil {
			return err
		}

		if err := enc.WriteToken(jsontext.String(d.Instance)); err != nil {
			return err
		}
	}

	for k, v := range d.Extensions {
		if k == "type" || k == "status" || k == "title" || k == "detail" || k == "instance" {
			continue
		}

		if err := enc.WriteToken(jsontext.String(k)); err != nil {
			return err
		}

		if err := json.MarshalEncode(enc, v, opts); err != nil {
			return err
		}
	}

	if err := enc.WriteToken(jsontext.ObjectEnd); err != nil {
		return err
	}

	return nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
//
// See UnmarshalJSONV2 for details.
func (d *Details) UnmarshalJSON(b []byte) error {
	// This will call (*Details).UnmarshalJSONV2.
	return json.Unmarshal(b, d)
}

// UnmarshalJSONFrom implements the json.UnmarshalerFrom interface.
//
// As required by RFC 9457 UnmarshalJSONV2 will ignore values for known fields if those values have the wrong type.
//
// For example if the parsed JSON contains a field "status" with the code "400" as a JSON string, the field will be
// ignored even if it may be possible to parse it as an integer.
func (d *Details) UnmarshalJSONFrom(dec *jsontext.Decoder, opts json.Options) error {
	var m map[string]any

	if err := json.UnmarshalDecode(dec, &m, opts); err != nil {
		return err
	}

	//  3.1. Members of a Problem Details Object
	//
	// 	Problem detail objects can have the following members. If a member's
	// 	value type does not match the specified type, the member MUST be
	// 	ignored -- i.e., processing will continue as if the member had not
	// 	been present.
	//
	// https://datatracker.ietf.org/doc/html/rfc9457#name-members-of-a-problem-detail

	if v, ok := m["type"].(string); ok {
		d.Type = v
	}

	if v, ok := m["status"].(float64); ok && float64(int(v)) == v {
		d.Status = int(v)
	}

	if v, ok := m["title"].(string); ok {
		d.Title = v
	}

	if v, ok := m["detail"].(string); ok {
		d.Detail = v
	}

	if v, ok := m["instance"].(string); ok {
		d.Instance = v
	}

	delete(m, "type")
	delete(m, "status")
	delete(m, "title")
	delete(m, "detail")
	delete(m, "instance")

	if len(m) != 0 {
		d.Extensions = m
	}

	return nil
}

// ServeHTTP encodes the value as JSON and writes it to the given response writer.
//
// If encoding fails, no data will be written and ServeHTTP will panic.
//
// ServeHTTP deletes any existing Content-Length header, sets Content-Type to “application/problem+json”, and sets
// X-Content-Type-Options to “nosniff”.
//
// If set the Status field is used to set the HTTP status. Otherwise [http.StatusInternalServerError] is used.
//
// ServeHTTP implements the [http.Handler] interface.
func (d *Details) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	b, err := json.Marshal(d)
	if err != nil {
		// If we get an error here we consider this a bug and panic.
		panic(err)
	}

	// Remove the Content-Length header and set X-Content-Type-Options as done by [http.Error].
	h := w.Header()
	h.Del("Content-Length")
	h.Set("Content-Type", ContentType)
	h.Set("X-Content-Type-Options", "nosniff")

	if d.Status != 0 {
		w.WriteHeader(d.Status)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}

	_, _ = w.Write(b)
}

// Type defines a specific problem type that can be used to create new Details instances.
//
// The main use case is as package-level variables that can than be used across different types and functions. These
// types can reduce boilerplate and serve as part of the documentation.
//
// Example:
//
//	var OutOfCreditProblemType = &problem.Type{
//		URI: "https://example.com/probs/out-of-credit",
//		Title: "You do not have enough credit.",
//		Status: http.StatusForbidden,
//	}
//
// Than in a handler:
//
//	type (s *MyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
//		// ...
//		if outOfCredit {
//			OutOfCreditProblemType.Details().ServeHTTP(w, r)
//			return
//		}
//		// ...
//	}
//
// When available, extra information can be added using [Option]s:
//
//	if outOfCredit {
//		OutOfCreditProblemType.Details(
//			problem.WithDetail("Your current balance is 30, but that costs 50."),
//			problem.WithInstance("/account/12345/msgs/abc"),
//			problem.WithExtension("balance", 30),
//			problem.WithExtension("accounts", []string{"/account/12345", "/account/67890"}),
//		).ServeHTTP(w, r)
//		return
//	}
type Type struct {
	// URI defines the type URI (typically, with the "http" or "https" scheme)
	URI string

	// Title contains a short, human-readable summary of the problem type.
	Title string

	// Status is the HTTP status code that should be used for responses.
	Status int

	// Extensions contains fixed extensions that are automatically added to Details instances
	// created from this type.
	Extensions map[string]any
}

// Is returns true if the given error can be converted to a [*Details] using [errors.As] and the URI, Title and Status
// match the given type.
//
// If any of [Type.URI], [Type.Title] or [Type.Status] is empty / zero, the field is skipped.
//
// For example, for a type with only a URI and no title or status, only the URI will be compared.
func Is(err error, t *Type) bool {
	var d *Details

	if !errors.As(err, &d) {
		return false
	}

	switch {
	case t.URI != "" && t.URI != cmp.Or(d.Type, AboutBlankTypeURI):
		return false
	case t.Title != "" && t.Title != d.Title:
		return false
	case t.Status != 0 && t.Status != d.Status:
		return false
	default:
		return true
	}
}

// Details creates a new [Details] instance from this type.
//
// It is equivalent to calling New(p.URI, p.Status, p.Title, opts...).
func (t *Type) Details(opts ...Option) *Details {
	d := New(t.URI, t.Title, t.Status)

	// Note: Conceptually what we want is to pass our extensions to New via WithExtensions as
	// the first option, so that later options can override any values that come from the type,
	//
	// Unfortunately doing this is kinda verbose and, more importantly, forces an allocation,
	// which we want to avoid.
	//
	// So, instead we handle the options ourselves instead of passing them to New and set the
	// extensions manually before applying the options.
	if len(t.Extensions) > 0 {
		d.Extensions = maps.Clone(t.Extensions)
	}

	for _, opt := range opts {
		opt(d)
	}

	return d
}
