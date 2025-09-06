package api

import (
	"encoding/json"
	"encoding/xml"
	"net/http"
	"time"
)

// Response represents a standard API response
type Response struct {
	Success   bool        `json:"success" xml:"success"`
	Data      interface{} `json:"data,omitempty" xml:"data,omitempty"`
	Error     *ErrorInfo  `json:"error,omitempty" xml:"error,omitempty"`
	Meta      *Meta       `json:"meta,omitempty" xml:"meta,omitempty"`
	Timestamp int64       `json:"timestamp" xml:"timestamp"`
}

// ErrorInfo contains error details
type ErrorInfo struct {
	Code    string                 `json:"code" xml:"code"`
	Message string                 `json:"message" xml:"message"`
	Details map[string]interface{} `json:"details,omitempty" xml:"details,omitempty"`
}

// Meta contains pagination and other metadata
type Meta struct {
	Page       int    `json:"page,omitempty" xml:"page,omitempty"`
	PerPage    int    `json:"per_page,omitempty" xml:"per_page,omitempty"`
	Total      int    `json:"total,omitempty" xml:"total,omitempty"`
	TotalPages int    `json:"total_pages,omitempty" xml:"total_pages,omitempty"`
	Version    string `json:"version,omitempty" xml:"version,omitempty"`
	RequestID  string `json:"request_id,omitempty" xml:"request_id,omitempty"`
}

// ValidationError represents field validation errors
type ValidationError struct {
	Field   string `json:"field" xml:"field"`
	Message string `json:"message" xml:"message"`
	Value   string `json:"value,omitempty" xml:"value,omitempty"`
}

// ResponseOption allows customizing responses
type ResponseOption func(*Response)

// WithMeta adds metadata to the response
func WithMeta(meta *Meta) ResponseOption {
	return func(r *Response) {
		r.Meta = meta
	}
}

// WithPagination adds pagination metadata
func WithPagination(page, perPage, total int) ResponseOption {
	return func(r *Response) {
		if r.Meta == nil {
			r.Meta = &Meta{}
		}
		r.Meta.Page = page
		r.Meta.PerPage = perPage
		r.Meta.Total = total
		if perPage > 0 {
			r.Meta.TotalPages = (total + perPage - 1) / perPage
		}
	}
}

// WithRequestID adds request ID to metadata
func WithRequestID(requestID string) ResponseOption {
	return func(r *Response) {
		if r.Meta == nil {
			r.Meta = &Meta{}
		}
		r.Meta.RequestID = requestID
	}
}

// WithVersion adds API version to metadata
func WithVersion(version string) ResponseOption {
	return func(r *Response) {
		if r.Meta == nil {
			r.Meta = &Meta{}
		}
		r.Meta.Version = version
	}
}

// JSON sends a JSON response
func JSON(w http.ResponseWriter, status int, data interface{}, opts ...ResponseOption) error {
	response := &Response{
		Success:   status < 400,
		Data:      data,
		Timestamp: time.Now().Unix(),
	}
	
	for _, opt := range opts {
		opt(response)
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(response)
}

// XML sends an XML response
func XML(w http.ResponseWriter, status int, data interface{}, opts ...ResponseOption) error {
	response := &Response{
		Success:   status < 400,
		Data:      data,
		Timestamp: time.Now().Unix(),
	}
	
	for _, opt := range opts {
		opt(response)
	}
	
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)
	return xml.NewEncoder(w).Encode(response)
}

// Error sends an error response
func Error(w http.ResponseWriter, status int, code, message string, details map[string]interface{}, opts ...ResponseOption) error {
	response := &Response{
		Success: false,
		Error: &ErrorInfo{
			Code:    code,
			Message: message,
			Details: details,
		},
		Timestamp: time.Now().Unix(),
	}
	
	for _, opt := range opts {
		opt(response)
	}
	
	// Check Accept header for response format
	accept := w.Header().Get("Accept")
	if accept == "application/xml" {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(status)
		return xml.NewEncoder(w).Encode(response)
	}
	
	// Default to JSON
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(response)
}

// ValidationErrors sends validation error response
func ValidationErrors(w http.ResponseWriter, errors []ValidationError, opts ...ResponseOption) error {
	details := make(map[string]interface{})
	details["validation_errors"] = errors
	
	return Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Validation failed", details, opts...)
}

// NotFound sends a 404 response
func NotFound(w http.ResponseWriter, message string, opts ...ResponseOption) error {
	if message == "" {
		message = "Resource not found"
	}
	return Error(w, http.StatusNotFound, "NOT_FOUND", message, nil, opts...)
}

// Unauthorized sends a 401 response
func Unauthorized(w http.ResponseWriter, message string, opts ...ResponseOption) error {
	if message == "" {
		message = "Unauthorized"
	}
	return Error(w, http.StatusUnauthorized, "UNAUTHORIZED", message, nil, opts...)
}

// Forbidden sends a 403 response
func Forbidden(w http.ResponseWriter, message string, opts ...ResponseOption) error {
	if message == "" {
		message = "Forbidden"
	}
	return Error(w, http.StatusForbidden, "FORBIDDEN", message, nil, opts...)
}

// InternalServerError sends a 500 response
func InternalServerError(w http.ResponseWriter, message string, opts ...ResponseOption) error {
	if message == "" {
		message = "Internal server error"
	}
	return Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", message, nil, opts...)
}

// Created sends a 201 response
func Created(w http.ResponseWriter, data interface{}, opts ...ResponseOption) error {
	return JSON(w, http.StatusCreated, data, opts...)
}

// NoContent sends a 204 response
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// RawJSON sends raw JSON without wrapper
func RawJSON(w http.ResponseWriter, status int, data interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(data)
}