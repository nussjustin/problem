package problem_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nussjustin/problem"
)

func textHandler(text string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")

		_, _ = io.WriteString(w, text)
	}
}

func panicHandler(v any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		panic(v)
	}
}

var teapotDetails = &problem.Details{
	Status: http.StatusTeapot,
	Title:  "I am a teapot",
}

type asTeapotDetailsError struct{}

func (e asTeapotDetailsError) As(v any) bool {
	details, ok := v.(**problem.Details)
	if ok {
		*details = teapotDetails
	}
	return ok
}

func (e asTeapotDetailsError) Error() string {
	return ""
}

type wrappedDetailsError struct{}

func (w wrappedDetailsError) Error() string {
	return ""
}

func (w wrappedDetailsError) Unwrap() error {
	return teapotDetails
}

func TestHandler(t *testing.T) {
	tests := []struct {
		Name                string
		Handler             http.HandlerFunc
		ExpectedStatus      int
		ExpectedContentType string
		ExpectedResponse    string
	}{
		{
			Name:                "No error",
			Handler:             textHandler("Hello World"),
			ExpectedStatus:      http.StatusOK,
			ExpectedContentType: "text/plain",
			ExpectedResponse:    "Hello World",
		},
		{
			Name:                "Not an error",
			Handler:             panicHandler(nil),
			ExpectedStatus:      http.StatusInternalServerError,
			ExpectedContentType: problem.ContentType,
			ExpectedResponse:    `{"status":500,"title":"Internal Server Error"}`,
		},
		{
			Name:                "Not details",
			Handler:             panicHandler(errors.New("not details")),
			ExpectedStatus:      http.StatusInternalServerError,
			ExpectedContentType: problem.ContentType,
			ExpectedResponse:    `{"status":500,"title":"Internal Server Error"}`,
		},
		{
			Name:                "Details",
			Handler:             panicHandler(teapotDetails),
			ExpectedStatus:      http.StatusTeapot,
			ExpectedContentType: problem.ContentType,
			ExpectedResponse:    `{"status":418,"title":"I am a teapot"}`,
		},
		{
			Name:                "Error as details",
			Handler:             panicHandler(asTeapotDetailsError{}),
			ExpectedStatus:      http.StatusTeapot,
			ExpectedContentType: problem.ContentType,
			ExpectedResponse:    `{"status":418,"title":"I am a teapot"}`,
		},
		{
			Name:                "Wrapped Details",
			Handler:             panicHandler(wrappedDetailsError{}),
			ExpectedStatus:      http.StatusTeapot,
			ExpectedContentType: problem.ContentType,
			ExpectedResponse:    `{"status":418,"title":"I am a teapot"}`,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)

			problem.Handler(test.Handler).ServeHTTP(w, r)

			result := w.Result()

			if result.StatusCode != test.ExpectedStatus {
				t.Errorf("got status %d, want %d", result.StatusCode, test.ExpectedStatus)
			}

			if result.Header.Get("Content-Type") != test.ExpectedContentType {
				t.Errorf("got content-type %s, want %s", result.Header.Get("Content-Type"), test.ExpectedContentType)
			}

			body, _ := io.ReadAll(result.Body)

			if string(body) != test.ExpectedResponse {
				t.Errorf("got response %s, want %s", string(body), test.ExpectedResponse)
			}
		})
	}
}
