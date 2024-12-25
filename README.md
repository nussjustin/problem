# RFC 9457 Problem Details API for Go

[![Go Reference](https://pkg.go.dev/badge/github.com/nussjustin/problem.svg)](https://pkg.go.dev/github.com/nussjustin/problem) [![Lint](https://github.com/nussjustin/problem/actions/workflows/golangci-lint.yml/badge.svg)](https://github.com/nussjustin/problem/actions/workflows/golangci-lint.yml) [![Test](https://github.com/nussjustin/problem/actions/workflows/test.yml/badge.svg)](https://github.com/nussjustin/problem/actions/workflows/test.yml)

This module provides an API for the [RFC 9457][0] problem details specification in Go.

> [!WARNING]  
> This module depends on the experimental github.com/go-json-experiment/json package.
> This package is planned to become part of the Go standard library in form of a future json/v2 package.
> Once that happens this module will be updated to use the new json/v2 package from the standard library instead.

## Examples

### Using problem details

Problem details are represented by the `problem.Details` type and can be constructed either directly via a struct
literal or using the global [New][1] function.

Example using a struct literal:

```go
var OutOfCreditProblemType = &problem.Type{
    Type:     "https://example.com/probs/out-of-credit",
    Title:    "You do not have enough credit.",
    Status:   http.StatusForbidden,
    Detail:   "Your current balance is 30, but that costs 50.",
    Instance: "/account/12345/msgs/abc",
    Extensions: map[string]any{
        "balance":  30,
        "accounts": []string{"/account/12345", "/account/67890"},
    },
}
```

All fields are optional, but at least the `Type`, `Title` and `Status` fields should always be set.

When marshaled as JSON the `Details` type will result in a single object containing all fields that do not have a zero
value as well as all added extensions.

Similarly, when unmarshalling, values for known fields are stored into the existing struct fields, with unknown fields
being added into the `Extensions` map.

The `Details` object implements the [http.Handler][2] interface making it possible to directly write a problem as a
response to an HTTP request.

Example:

```go
type (s *MyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // ...
    if outOfCredit {
        &problem.Type{
            Type:     "https://example.com/probs/out-of-credit",
            Title:    "You do not have enough credit.",
            Status:   http.StatusForbidden,
        }.ServeHTTP(w, r)
        return
    }
    // ...
}
```

### Defining and using reusable types

It is also possible to pre-defines specific problem types that can be used to create new Details instances.

The main use case is as package-level variables that can than be used across different types and functions. These
types can reduce boilerplate and serve as part of the documentation

Example:

```go
var OutOfCreditProblemType = &problem.Type{
    URI: "https://example.com/probs/out-of-credit",
    Title: "You do not have enough credit.",
    Status: http.StatusForbidden,
}
```

The [Details][3] method returns a new `Details` value that inherits the values from the type, while also making it
possible to add case specific details data and other relevant data using functional options.

Example:

```go
if outOfCredit {
    OutOfCreditProblemType.Details(
        problem.WithDetail("Your current balance is 30, but that costs 50."),
        problem.WithInstance("/account/12345/msgs/abc"),
        problem.WithExtension("balance", 30),
        problem.WithExtension("accounts", []string{"/account/12345", "/account/67890"}),
    ).ServeHTTP(w, r)
}
```

### Recovering from panics

The [Handler][4] function wraps an existing [http.Handler][2] and automatically recovers any panics and responds to the
request with JSON-encoded problem details.

Example:

```go
func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/", myHandler)

    // Wrap the handler to handle panics
    handler := problem.Handler(mux)

    log.Fatal(http.ListenAndServe(":8000", handler))
}
```

## Contributing
Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

Please make sure to update tests as appropriate.

## License
[MIT](https://choosealicense.com/licenses/mit/)

[0]: https://datatracker.ietf.org/doc/html/rfc9457
[1]: https://pkg.go.dev/github.com/nussjustin/problem#New
[2]: https://pkg.go.dev/net/http#Handler
[3]: https://pkg.go.dev/github.com/nussjustin/problem#Details
[4]: https://pkg.go.dev/github.com/nussjustin/problem#Handler