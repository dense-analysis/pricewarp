// Package lax implements tools for building easy RESTful APIs.
//
//      ^ ^
//  ("\(-_-)/")
//  )(       )(
// ((...) (...))
//
// Take it easy!
package lax

import (
    "encoding/json"
    "net/http"
    "strings"
)

// A flag for debugging the server.
var debug bool

// EnableDebugMode enables debugging for the API, so debug output is printed.
func EnableDebugMode() {
    debug = true
}

// DisableDebugMode disables debugging for the API, so debug output is hidden.
func DisableDebugMode() {
    debug = false
}

// DebugModeEnabled returns `true` if debug mode is enabled.
func DebugModeEnabled() bool {
    return debug
}

// Request wraps http.Request to provide convenience methods.
type Request struct {
    *http.Request
}

// JSON loads JSON data from a request into the given address.
func (request *Request) JSON(ptr interface{}) error {
    return json.NewDecoder(request.Body).Decode(ptr)
}

// MethodHandler is a handle for an HTTP method.
type MethodHandler = func(request *Request) interface{}

// View represents a view for a RESTful API.
type View struct {
    // The handler for HEAD requests.
    Head MethodHandler
    // The handler for GET requests.
    Get MethodHandler
    // The handler for POST requests.
    Post MethodHandler
    // The handler for PUT requests.
    Put MethodHandler
    // The handler for DELETE requests.
    Delete MethodHandler
}

// Response represents a response to return.
type Response struct {
    Status int
    Data interface{}
}

// IssueDescription is an issue created with Issue.
type IssueDescription struct {
    Path string `json:"path"`
    Problem string `json:"problem"`
}

// Issue creates an issue for use with MakeErrorListResponse.
func Issue(path, problem string) IssueDescription {
    return IssueDescription{path, problem}
}

// MakeResponse creates a response with a status code and data.
func MakeResponse(status int, data interface{}) *Response {
    return &Response{status, data}
}

// MakeBadRequestResponse creates a 400 error response from one object.
func MakeBadRequestResponse(data interface{}) *Response {
    switch v := data.(type) {
    case error:
        // Get the string from errors for 400 responses.
        return &Response{http.StatusBadRequest, v.Error()}
    default:
        return &Response{http.StatusBadRequest, v}
    }
}

// MakeErrorListResponse creates a 400 error response from parts.
func MakeErrorListResponse(parts ...IssueDescription) *Response {
    return &Response{http.StatusBadRequest, parts}
}

// A default handler for handling methods that are not allowed.
func methodNotAllowedHandler(request *Request) interface{} {
    return &Response{http.StatusMethodNotAllowed, "Method Not Allowed"}
}

// Get the pointer to the handler for the HTTP request method.
func dispatch(view *View, requestMethod string) (MethodHandler, int) {
    var handler MethodHandler
    defaultStatus := http.StatusOK

    if (strings.EqualFold(requestMethod, "get")) {
        handler = view.Get
    } else if (strings.EqualFold(requestMethod, "post")) {
        handler = view.Post
        defaultStatus = http.StatusCreated
    } else if (strings.EqualFold(requestMethod, "put")) {
        handler = view.Put
    } else if (strings.EqualFold(requestMethod, "delete")) {
        handler = view.Delete
        defaultStatus = http.StatusNoContent
    } else if (strings.EqualFold(requestMethod, "head")) {
        handler = view.Head
    }

    if (handler == nil) {
        handler = methodNotAllowedHandler
        defaultStatus = http.StatusMethodNotAllowed
    }

    return handler, defaultStatus
}

// Normalise response data so we can consume it.
func normalise(response interface{}, defaultStatus int) (*Response, error) {
    switch v := response.(type) {
    case *Response:
        return v, nil
    case error:
        return &Response{http.StatusInternalServerError, nil}, v
    default:
        return &Response{defaultStatus, v}, nil
    }
}

// Wrap creates an HandlerFunc from a View.
func Wrap(view View) http.HandlerFunc {
    return func(writer http.ResponseWriter, httpRequest *http.Request) {
        request := Request{httpRequest}
        method, defaultStatus := dispatch(&view, request.Method)
        response, responseErr := normalise(method(&request), defaultStatus)

        if (responseErr != nil) {
            if (response.Status < 500 || debug) {
                http.Error(writer, responseErr.Error(), response.Status)
            } else {
                http.Error(writer, "Internal Server Error", response.Status)
            }

            return
        }

        outputEncoder := json.NewEncoder(writer)
        outputEncoder.SetEscapeHTML(false)
        writer.WriteHeader(response.Status)

        if err := outputEncoder.Encode(response.Data); err != nil {
            if debug {
                http.Error(writer, err.Error(), http.StatusInternalServerError)
            } else {
                http.Error(
                    writer,
                    "Internal Server Error",
                    http.StatusInternalServerError,
                )
            }
        }
    }
}
