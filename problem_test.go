package problem_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/nussjustin/problem"
)

type accountsSlice []int

func (a accountsSlice) MarshalJSON() ([]byte, error) {
	s := make([]string, len(a))
	for i := range a {
		s[i] = fmt.Sprintf("/account/%d", a[i])
	}
	return json.Marshal(s)
}

type panicMarshaler struct{}

func (panicMarshaler) MarshalJSON() ([]byte, error) {
	panic("MarshalJSON called")
}

type testError struct{}

func (testError) Error() string {
	return ""
}

func assertJSON(tb testing.TB, want string, got []byte) {
	tb.Helper()

	var gotMap map[string]any
	if err := json.Unmarshal(got, &gotMap); err != nil {
		tb.Fatalf("failed to unmarshal generated JSON: %s", err)
	}

	var wantMap map[string]any
	if err := json.Unmarshal([]byte(want), &wantMap); err != nil {
		tb.Fatalf("failed to unmarshal wanted JSON: %s", err)
	}

	if diff := cmp.Diff(wantMap, gotMap); diff != "" {
		tb.Errorf("JSON mismatch (-want +got):\n%s", diff)
	}
}

func assertResponse(tb testing.TB, rec *httptest.ResponseRecorder, wantStatus int, wantJSON string) {
	tb.Helper()

	if got, want := rec.Code, wantStatus; got != want {
		tb.Errorf("got status %d, want %d", got, want)
	}

	if got, want := rec.Header().Get("Content-Length"), ""; got != want {
		tb.Errorf("got Content-Length %s, want %s", got, want)
	}

	if got, want := rec.Header().Get("Content-Type"), problem.ContentType; got != want {
		tb.Errorf("got Content-Type %q, want %q", got, want)
	}

	if got, want := rec.Header().Get("X-Content-Type-Options"), "nosniff"; got != want {
		tb.Errorf("got X-Content-Type-Options %q, want %q", got, want)
	}

	assertJSON(tb, wantJSON, rec.Body.Bytes())
}

func TestNew(t *testing.T) {
	tests := []struct {
		Name     string
		Opts     []problem.Option
		Expected problem.Details
	}{
		{
			Name: "No options",
			Expected: problem.Details{
				Type:   "https://example.com/probs/out-of-credit",
				Title:  "You do not have enough credit.",
				Status: http.StatusForbidden,
			},
		},
		{
			Name: "With options",
			Opts: []problem.Option{
				problem.WithStatus(http.StatusTeapot),
				problem.WithDetail("Your current balance is 30, but that costs 50."),
				problem.WithInstance("/account/12345/msgs/abc"),
				problem.WithExtension("balance", 30),
				problem.WithExtension("accounts", []string{"/account/12345", "/account/67890"}),
				problem.WithUnderlying(testError{}),
			},
			Expected: problem.Details{
				Type:     "https://example.com/probs/out-of-credit",
				Title:    "You do not have enough credit.",
				Status:   http.StatusTeapot,
				Detail:   "Your current balance is 30, but that costs 50.",
				Instance: "/account/12345/msgs/abc",
				Extensions: map[string]any{
					"balance":  30,
					"accounts": []string{"/account/12345", "/account/67890"},
				},
				Underlying: testError{},
			},
		},
		{
			Name: "Mix of WithExtension and WithExtensions",
			Opts: []problem.Option{
				problem.WithDetail("Your current balance is 30, but that costs 50."),
				problem.WithInstance("/account/12345/msgs/abc"),
				problem.WithExtension("balance", 30),
				problem.WithExtensions(map[string]any{"accounts": []string{"/account/12345", "/account/67890"}}),
			},
			Expected: problem.Details{
				Type:     "https://example.com/probs/out-of-credit",
				Title:    "You do not have enough credit.",
				Status:   http.StatusForbidden,
				Detail:   "Your current balance is 30, but that costs 50.",
				Instance: "/account/12345/msgs/abc",
				Extensions: map[string]any{
					"balance":  30,
					"accounts": []string{"/account/12345", "/account/67890"},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			d := problem.New(
				"https://example.com/probs/out-of-credit",
				"You do not have enough credit.",
				http.StatusForbidden,
				test.Opts...)

			if diff := cmp.Diff(&test.Expected, d); diff != "" {
				t.Errorf("problem.New() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFrom(t *testing.T) {
	tests := []struct {
		Name      string
		Type      string
		Response  string
		Want      *problem.Details
		WantError bool
	}{
		{
			Name:      "No problem",
			Type:      "application/json",
			Response:  `ignored`,
			Want:      nil,
			WantError: false,
		},

		{
			Name:      "Invalid content type",
			Type:      "application/problem+xml",
			Response:  `ignored`,
			Want:      nil,
			WantError: false,
		},

		{
			Name:      "Invalid response",
			Type:      problem.ContentType,
			Response:  `invalid`,
			Want:      nil,
			WantError: true,
		},

		{
			Name: "Valid response",
			Type: problem.ContentType,
			Response: `{
				"type": "https://example.com/probs/out-of-credit",
				"title": "You do not have enough credit.",
				"status": 403,
				"detail": "Your current balance is 30, but that costs 50.",
				"instance": "/account/12345/msgs/abc",
				"balance": 30,
				"accounts": ["/account/12345", "/account/67890"]
			}`,
			Want: &problem.Details{
				Type:     "https://example.com/probs/out-of-credit",
				Title:    "You do not have enough credit.",
				Status:   http.StatusForbidden,
				Detail:   "Your current balance is 30, but that costs 50.",
				Instance: "/account/12345/msgs/abc",
				Extensions: map[string]any{
					"balance":  30.0,
					"accounts": []any{"/account/12345", "/account/67890"},
				},
			},
			WantError: false,
		},
		{
			Name: "Valid response with parameters in type",
			Type: problem.ContentType + ";a=1;b=2",
			Response: `{
				"type": "https://example.com/probs/out-of-credit",
				"status": 403
			}`,
			Want: &problem.Details{
				Type:   "https://example.com/probs/out-of-credit",
				Status: http.StatusForbidden,
			},
			WantError: false,
		},
		{
			Name: "Valid response with no status",
			Type: problem.ContentType + ";a=1;b=2",
			Response: `{
				"type": "https://example.com/probs/out-of-credit"
			}`,
			Want: &problem.Details{
				Type:   "https://example.com/probs/out-of-credit",
				Status: http.StatusTeapot,
			},
			WantError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			resp := &http.Response{Header: http.Header{}}
			resp.StatusCode = http.StatusTeapot
			resp.Header.Add("Content-Type", test.Type)
			resp.Body = io.NopCloser(strings.NewReader(test.Response))

			got, gotErr := problem.From(resp)

			if diff := cmp.Diff(test.Want, got); diff != "" {
				t.Errorf("From() mismatch (-want +got):\n%s", diff)
			}

			switch {
			case gotErr != nil && !test.WantError:
				t.Errorf("got error %v, want nil", gotErr)
			case gotErr == nil && test.WantError:
				t.Errorf("expected error not returned")
			}
		})
	}
}

func TestDetails_Unwrap(t *testing.T) {
	d := &problem.Details{
		Underlying: testError{},
	}

	got, want := d.Unwrap(), testError{}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Details.Unwrap() mismatch (-want +got):\n%s", diff)
	}

	if !errors.Is(d, testError{}) {
		t.Errorf("errors.Is() returned false, want true")
	}
}

func TestDetails_MarshalJSON(t *testing.T) {
	tests := []struct {
		Name  string
		Input problem.Details
		Want  string
	}{
		{
			Name:  "Empty",
			Input: problem.Details{},
			Want:  `{}`,
		},
		{
			Name: "Minimal",
			Input: problem.Details{
				Type:   "https://example.com/probs/out-of-credit",
				Title:  "You do not have enough credit.",
				Status: http.StatusForbidden,
			},
			Want: `{
				"type": "https://example.com/probs/out-of-credit",
				"title": "You do not have enough credit.",
				"status": 403
			}`,
		},
		{
			Name: "Full",
			Input: problem.Details{
				Type:     "https://example.com/probs/out-of-credit",
				Title:    "You do not have enough credit.",
				Status:   http.StatusForbidden,
				Detail:   "Your current balance is 30, but that costs 50.",
				Instance: "/account/12345/msgs/abc",
				Extensions: map[string]any{
					"balance":  30,
					"accounts": []string{"/account/12345", "/account/67890"},
				},
			},
			Want: `{
				"type": "https://example.com/probs/out-of-credit",
				"title": "You do not have enough credit.",
				"status": 403,
				"detail": "Your current balance is 30, but that costs 50.",
				"instance": "/account/12345/msgs/abc",
				"balance": 30,
				"accounts": ["/account/12345", "/account/67890"]
			}`,
		},
		{
			Name: "Conflicting extension keys",
			Input: problem.Details{
				Type:   "https://example.com/probs/out-of-credit",
				Title:  "You do not have enough credit.",
				Status: http.StatusForbidden,
				Detail: "Your current balance is 30, but that costs 50.",
				Extensions: map[string]any{
					"type":     problem.AboutBlankTypeURI,
					"title":    "I am a teapot",
					"status":   http.StatusTeapot,
					"detail":   "I am a teapot",
					"instance": "/428",
				},
			},
			Want: `{
				"type": "https://example.com/probs/out-of-credit",
				"title": "You do not have enough credit.",
				"status": 403,
				"detail": "Your current balance is 30, but that costs 50."
			}`,
		},
		{
			Name: "Complex extensions",
			Input: problem.Details{
				Type:   "https://example.com/probs/out-of-credit",
				Title:  "You do not have enough credit.",
				Status: http.StatusForbidden,
				Extensions: map[string]any{
					"balance": struct {
						Amount float64 `json:"amount"`
					}{
						Amount: 30,
					},
					"accounts": accountsSlice{12345, 67890},
				},
			},
			Want: `{
				"type": "https://example.com/probs/out-of-credit",
				"title": "You do not have enough credit.",
				"status": 403,
				"balance": {
					"amount": 30
				},
				"accounts": ["/account/12345", "/account/67890"]
			}`,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			actualJSON, err := json.Marshal(&test.Input)
			if err != nil {
				t.Fatalf("failed to marshal input: %s", err)
			}

			assertJSON(t, test.Want, actualJSON)
		})
	}
}

func TestDetails_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		Name  string
		Input string
		Want  problem.Details
	}{
		{
			Name:  "Empty",
			Input: `{}`,
			Want:  problem.Details{},
		},
		{
			Name: "Minimal",
			Input: `{
				"type": "https://example.com/probs/out-of-credit",
				"title": "You do not have enough credit.",
				"status": 403
			}`,
			Want: problem.Details{
				Type:   "https://example.com/probs/out-of-credit",
				Title:  "You do not have enough credit.",
				Status: http.StatusForbidden,
			},
		},
		{
			Name: "Full",
			Input: `{
				"type": "https://example.com/probs/out-of-credit",
				"title": "You do not have enough credit.",
				"status": 403,
				"detail": "Your current balance is 30, but that costs 50.",
				"instance": "/account/12345/msgs/abc",
				"balance": 30,
				"accounts": ["/account/12345", "/account/67890"]
			}`,
			Want: problem.Details{
				Type:     "https://example.com/probs/out-of-credit",
				Title:    "You do not have enough credit.",
				Status:   http.StatusForbidden,
				Detail:   "Your current balance is 30, but that costs 50.",
				Instance: "/account/12345/msgs/abc",
				Extensions: map[string]any{
					"balance":  30.0,
					"accounts": []any{"/account/12345", "/account/67890"},
				},
			},
		},
		{
			Name: "Wrong types",
			Input: `{
				"type": true,
				"title": false,
				"status": "403",
				"detail": 1,
				"instance": ["/account/12345/msgs/abc"]
			}`,
			Want: problem.Details{},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			var got problem.Details

			if err := json.Unmarshal([]byte(test.Input), &got); err != nil {
				t.Fatalf("failed to unmarshal input: %s", err)
			}

			if diff := cmp.Diff(test.Want, got); diff != "" {
				t.Errorf("unmarshaled details mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestDetails_ServeHTTP(t *testing.T) {
	t.Run("Panic", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/account/12345/msgs/abc", nil)

		rec := httptest.NewRecorder()
		rec.Header().Set("Content-Length", "1337")
		rec.Header().Set("Content-Type", "application/xml")

		defer func() {
			r := recover()

			if msg, _ := r.(string); msg != "MarshalJSON called" {
				t.Fatal("no panic was raised")
			}

			if rec.Body.Len() > 0 {
				t.Errorf("data was written")
			}
		}()

		(&problem.Details{
			Extensions: map[string]any{
				"invalid": panicMarshaler{},
			},
		}).ServeHTTP(rec, r)
	})

	t.Run("Minimal details", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/account/12345/msgs/abc", nil)

		rec := httptest.NewRecorder()
		rec.Header().Set("Content-Length", "1337")
		rec.Header().Set("Content-Type", "application/xml")

		(&problem.Details{
			Type:  "https://example.com/probs/out-of-credit",
			Title: "You do not have enough credit.",
		}).ServeHTTP(rec, r)

		if got, want := rec.Code, http.StatusInternalServerError; got != want {
			t.Errorf("got status %d, want %d", got, want)
		}

		wantJSON := `{
			"type": "https://example.com/probs/out-of-credit",
			"title": "You do not have enough credit."
		}`

		assertJSON(t, wantJSON, rec.Body.Bytes())
	})

	t.Run("Full details", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/account/12345/msgs/abc", nil)

		rec := httptest.NewRecorder()
		rec.Header().Set("Content-Length", "1337")
		rec.Header().Set("Content-Type", "application/xml")

		(&problem.Details{
			Type:     "https://example.com/probs/out-of-credit",
			Title:    "You do not have enough credit.",
			Status:   http.StatusForbidden,
			Detail:   "Your current balance is 30, but that costs 50.",
			Instance: "/account/12345/msgs/abc",
			Extensions: map[string]any{
				"balance":  30,
				"accounts": []string{"/account/12345", "/account/67890"},
			},
		}).ServeHTTP(rec, r)

		assertResponse(t, rec, http.StatusForbidden, `{
			"type": "https://example.com/probs/out-of-credit",
			"title": "You do not have enough credit.",
			"status": 403,
			"detail": "Your current balance is 30, but that costs 50.",
			"instance": "/account/12345/msgs/abc",
			"balance": 30,
			"accounts": ["/account/12345", "/account/67890"]
		}`)
	})
}

func TestIs(t *testing.T) {
	tests := []struct {
		Name  string
		Error error
		Type  problem.Type
		Want  bool
	}{
		{
			Name:  "No error",
			Error: nil,
			Type:  problem.Type{},
			Want:  false,
		},

		{
			Name:  "Not details",
			Error: io.EOF,
			Type:  problem.Type{},
			Want:  false,
		},

		{
			Name:  "Empty type",
			Error: &problem.Details{},
			Type:  problem.Type{},
			Want:  true,
		},

		{
			Name: "Full type",
			Error: &problem.Details{
				Type:     "https://example.com/probs/out-of-credit",
				Title:    "You do not have enough credit.",
				Status:   http.StatusForbidden,
				Detail:   "Your current balance is 30, but that costs 50.",
				Instance: "/account/12345/msgs/abc",
			},
			Type: problem.Type{
				URI:    "https://example.com/probs/out-of-credit",
				Title:  "You do not have enough credit.",
				Status: http.StatusForbidden,
			},
			Want: true,
		},
		{
			Name: "Full type with extensions",
			Error: &problem.Details{
				Type:     "https://example.com/probs/out-of-credit",
				Title:    "You do not have enough credit.",
				Status:   http.StatusForbidden,
				Detail:   "Your current balance is 30, but that costs 50.",
				Instance: "/account/12345/msgs/abc",
				Extensions: map[string]any{
					"InDetails": true,
				},
			},
			Type: problem.Type{
				URI:    "https://example.com/probs/out-of-credit",
				Title:  "You do not have enough credit.",
				Status: http.StatusForbidden,
				Extensions: map[string]any{
					"InType": true,
				},
			},
			Want: true,
		},

		{
			Name: "Type with only URI",
			Error: &problem.Details{
				Type:     "https://example.com/probs/out-of-credit",
				Title:    "You do not have enough credit.",
				Status:   http.StatusForbidden,
				Detail:   "Your current balance is 30, but that costs 50.",
				Instance: "/account/12345/msgs/abc",
			},
			Type: problem.Type{
				URI: "https://example.com/probs/out-of-credit",
			},
			Want: true,
		},
		{
			Name: "Type with only title",
			Error: &problem.Details{
				Type:     "https://example.com/probs/out-of-credit",
				Title:    "You do not have enough credit.",
				Status:   http.StatusForbidden,
				Detail:   "Your current balance is 30, but that costs 50.",
				Instance: "/account/12345/msgs/abc",
			},
			Type: problem.Type{
				Title: "You do not have enough credit.",
			},
			Want: true,
		},
		{
			Name: "Type with only status",
			Error: &problem.Details{
				Type:     "https://example.com/probs/out-of-credit",
				Title:    "You do not have enough credit.",
				Status:   http.StatusForbidden,
				Detail:   "Your current balance is 30, but that costs 50.",
				Instance: "/account/12345/msgs/abc",
			},
			Type: problem.Type{
				Status: http.StatusForbidden,
			},
			Want: true,
		},

		{
			Name: "Type with mismatched URI",
			Error: &problem.Details{
				Type:     "https://example.com/probs/out-of-credit",
				Title:    "You do not have enough credit.",
				Status:   http.StatusForbidden,
				Detail:   "Your current balance is 30, but that costs 50.",
				Instance: "/account/12345/msgs/abc",
			},
			Type: problem.Type{
				URI: "https://example.com/probs/out-of-motivation",
			},
			Want: false,
		},
		{
			Name: "Type with mismatched title",
			Error: &problem.Details{
				Type:     "https://example.com/probs/out-of-credit",
				Title:    "You do not have enough credit.",
				Status:   http.StatusForbidden,
				Detail:   "Your current balance is 30, but that costs 50.",
				Instance: "/account/12345/msgs/abc",
			},
			Type: problem.Type{
				Title: "You do not have enough motivation.",
			},
			Want: false,
		},
		{
			Name: "Type with mismatched status",
			Error: &problem.Details{
				Type:     "https://example.com/probs/out-of-credit",
				Title:    "You do not have enough credit.",
				Status:   http.StatusForbidden,
				Detail:   "Your current balance is 30, but that costs 50.",
				Instance: "/account/12345/msgs/abc",
			},
			Type: problem.Type{
				Status: http.StatusTeapot,
			},
			Want: false,
		},

		{
			Name:  "Type with about:blank URI matches empty type",
			Error: &problem.Details{},
			Type: problem.Type{
				URI: problem.AboutBlankTypeURI,
			},
			Want: true,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			if got := problem.Is(test.Error, &test.Type); got != test.Want {
				t.Errorf("got %t, want %t", got, test.Want)
			}
		})
	}
}

func TestType_Details(t *testing.T) {
	got := (&problem.Type{
		URI:    "https://example.com/probs/out-of-credit",
		Title:  "You do not have enough credit.",
		Status: http.StatusForbidden,
		Extensions: map[string]any{
			"currency":  "EUR",
			"overdraft": false,
		},
	}).Details(
		problem.WithStatus(http.StatusTeapot),
		problem.WithDetail("Your current balance is 30, but that costs 50."),
		problem.WithInstance("/account/12345/msgs/abc"),
		problem.WithExtension("balance", 30),
		problem.WithExtension("accounts", []string{"/account/12345", "/account/67890"}),
		problem.WithExtension("currency", "USD"),
	)

	want := &problem.Details{
		Type:     "https://example.com/probs/out-of-credit",
		Title:    "You do not have enough credit.",
		Status:   http.StatusTeapot,
		Detail:   "Your current balance is 30, but that costs 50.",
		Instance: "/account/12345/msgs/abc",
		Extensions: map[string]any{
			"balance":   30,
			"accounts":  []string{"/account/12345", "/account/67890"},
			"currency":  "USD",
			"overdraft": false,
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Type.Details() mismatch (-want +got):\n%s", diff)
	}
}
