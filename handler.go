package problem

import (
	"errors"
	"net/http"
)

// InternalServerError is used by [Handler] to serve as response if no callback is defined.
var InternalServerError = &Details{
	Status: http.StatusInternalServerError,
	Title:  "Internal Server Error",
}

// Handler wraps the given http.Handler and automatically recovers panics from given handler.
//
// When recovering from a panic, if the recovered value is an error, the handler will first try converting it into a
// value of type *Details using [errors.As] and, if successful, serve the value using [Details.ServeHTTP].
//
// Otherwise [InternalServerError] is served as response.
func Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}

			var details *Details

			if err, ok := recovered.(error); ok {
				errors.As(err, &details)
			}

			if details == nil {
				details = InternalServerError
			}

			details.ServeHTTP(w, r)
		}()

		next.ServeHTTP(w, r)
	})
}
