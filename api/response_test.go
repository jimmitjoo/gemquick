package api

import (
	"encoding/json"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJSON(t *testing.T) {
	tests := []struct {
		name   string
		status int
		data   interface{}
		opts   []ResponseOption
		want   Response
	}{
		{
			name:   "success response",
			status: http.StatusOK,
			data:   map[string]string{"message": "success"},
			want: Response{
				Success: true,
				Data:    map[string]string{"message": "success"},
			},
		},
		{
			name:   "error response",
			status: http.StatusBadRequest,
			data:   nil,
			want: Response{
				Success: false,
				Data:    nil,
			},
		},
		{
			name:   "with pagination",
			status: http.StatusOK,
			data:   []string{"item1", "item2"},
			opts:   []ResponseOption{WithPagination(1, 10, 100)},
			want: Response{
				Success: true,
				Data:    []string{"item1", "item2"},
				Meta: &Meta{
					Page:       1,
					PerPage:    10,
					Total:      100,
					TotalPages: 10,
				},
			},
		},
		{
			name:   "with request ID",
			status: http.StatusOK,
			data:   "test",
			opts:   []ResponseOption{WithRequestID("req-123")},
			want: Response{
				Success: true,
				Data:    "test",
				Meta:    &Meta{RequestID: "req-123"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			err := JSON(w, tt.status, tt.data, tt.opts...)
			if err != nil {
				t.Fatalf("JSON() error = %v", err)
			}

			if w.Code != tt.status {
				t.Errorf("JSON() status = %v, want %v", w.Code, tt.status)
			}

			var got Response
			if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if got.Success != tt.want.Success {
				t.Errorf("JSON() Success = %v, want %v", got.Success, tt.want.Success)
			}

			if tt.want.Meta != nil {
				if got.Meta == nil {
					t.Errorf("JSON() Meta = nil, want %v", tt.want.Meta)
				} else {
					if got.Meta.Page != tt.want.Meta.Page {
						t.Errorf("JSON() Meta.Page = %v, want %v", got.Meta.Page, tt.want.Meta.Page)
					}
					if got.Meta.RequestID != tt.want.Meta.RequestID {
						t.Errorf("JSON() Meta.RequestID = %v, want %v", got.Meta.RequestID, tt.want.Meta.RequestID)
					}
				}
			}
		})
	}
}

func TestXML(t *testing.T) {
	w := httptest.NewRecorder()
	// Use a struct for XML serialization
	type TestData struct {
		Message string `xml:"message"`
	}
	data := TestData{Message: "test"}
	
	err := XML(w, http.StatusOK, data, WithVersion("v1"))
	if err != nil {
		t.Fatalf("XML() error = %v", err)
	}

	if w.Header().Get("Content-Type") != "application/xml" {
		t.Errorf("XML() Content-Type = %v, want application/xml", w.Header().Get("Content-Type"))
	}

	var got Response
	if err := xml.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("Failed to decode XML response: %v", err)
	}

	if !got.Success {
		t.Errorf("XML() Success = false, want true")
	}

	if got.Meta == nil || got.Meta.Version != "v1" {
		t.Errorf("XML() Meta.Version = %v, want v1", got.Meta.Version)
	}
}

func TestError(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		code    string
		message string
		details map[string]interface{}
	}{
		{
			name:    "basic error",
			status:  http.StatusBadRequest,
			code:    "BAD_REQUEST",
			message: "Invalid request",
			details: nil,
		},
		{
			name:    "error with details",
			status:  http.StatusUnprocessableEntity,
			code:    "VALIDATION_ERROR",
			message: "Validation failed",
			details: map[string]interface{}{
				"field": "email",
				"error": "invalid format",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			err := Error(w, tt.status, tt.code, tt.message, tt.details)
			if err != nil {
				t.Fatalf("Error() error = %v", err)
			}

			if w.Code != tt.status {
				t.Errorf("Error() status = %v, want %v", w.Code, tt.status)
			}

			var got Response
			if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if got.Success {
				t.Errorf("Error() Success = true, want false")
			}

			if got.Error == nil {
				t.Fatal("Error() Error = nil, want error info")
			}

			if got.Error.Code != tt.code {
				t.Errorf("Error() Error.Code = %v, want %v", got.Error.Code, tt.code)
			}

			if got.Error.Message != tt.message {
				t.Errorf("Error() Error.Message = %v, want %v", got.Error.Message, tt.message)
			}
		})
	}
}

func TestValidationErrors(t *testing.T) {
	w := httptest.NewRecorder()
	errors := []ValidationError{
		{Field: "email", Message: "invalid format", Value: "notanemail"},
		{Field: "age", Message: "must be positive", Value: "-5"},
	}

	err := ValidationErrors(w, errors)
	if err != nil {
		t.Fatalf("ValidationErrors() error = %v", err)
	}

	if w.Code != http.StatusBadRequest {
		t.Errorf("ValidationErrors() status = %v, want %v", w.Code, http.StatusBadRequest)
	}

	var got Response
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if got.Error.Code != "VALIDATION_ERROR" {
		t.Errorf("ValidationErrors() Error.Code = %v, want VALIDATION_ERROR", got.Error.Code)
	}

	if got.Error.Details == nil {
		t.Fatal("ValidationErrors() Error.Details = nil, want validation errors")
	}

	validationErrors, ok := got.Error.Details["validation_errors"]
	if !ok {
		t.Fatal("ValidationErrors() missing validation_errors in details")
	}

	errList, ok := validationErrors.([]interface{})
	if !ok || len(errList) != 2 {
		t.Errorf("ValidationErrors() validation_errors count = %v, want 2", len(errList))
	}
}

func TestHelperFunctions(t *testing.T) {
	t.Run("NotFound", func(t *testing.T) {
		w := httptest.NewRecorder()
		NotFound(w, "Resource not found")
		
		if w.Code != http.StatusNotFound {
			t.Errorf("NotFound() status = %v, want %v", w.Code, http.StatusNotFound)
		}
		
		var got Response
		json.NewDecoder(w.Body).Decode(&got)
		if got.Error.Code != "NOT_FOUND" {
			t.Errorf("NotFound() Error.Code = %v, want NOT_FOUND", got.Error.Code)
		}
	})

	t.Run("Unauthorized", func(t *testing.T) {
		w := httptest.NewRecorder()
		Unauthorized(w, "")
		
		if w.Code != http.StatusUnauthorized {
			t.Errorf("Unauthorized() status = %v, want %v", w.Code, http.StatusUnauthorized)
		}
		
		var got Response
		json.NewDecoder(w.Body).Decode(&got)
		if got.Error.Message != "Unauthorized" {
			t.Errorf("Unauthorized() default message = %v, want Unauthorized", got.Error.Message)
		}
	})

	t.Run("Forbidden", func(t *testing.T) {
		w := httptest.NewRecorder()
		Forbidden(w, "Access denied")
		
		if w.Code != http.StatusForbidden {
			t.Errorf("Forbidden() status = %v, want %v", w.Code, http.StatusForbidden)
		}
	})

	t.Run("InternalServerError", func(t *testing.T) {
		w := httptest.NewRecorder()
		InternalServerError(w, "")
		
		if w.Code != http.StatusInternalServerError {
			t.Errorf("InternalServerError() status = %v, want %v", w.Code, http.StatusInternalServerError)
		}
		
		var got Response
		json.NewDecoder(w.Body).Decode(&got)
		if got.Error.Message != "Internal server error" {
			t.Errorf("InternalServerError() default message = %v", got.Error.Message)
		}
	})

	t.Run("Created", func(t *testing.T) {
		w := httptest.NewRecorder()
		Created(w, map[string]int{"id": 123})
		
		if w.Code != http.StatusCreated {
			t.Errorf("Created() status = %v, want %v", w.Code, http.StatusCreated)
		}
	})

	t.Run("NoContent", func(t *testing.T) {
		w := httptest.NewRecorder()
		NoContent(w)
		
		if w.Code != http.StatusNoContent {
			t.Errorf("NoContent() status = %v, want %v", w.Code, http.StatusNoContent)
		}
		
		if w.Body.Len() != 0 {
			t.Errorf("NoContent() body length = %v, want 0", w.Body.Len())
		}
	})

	t.Run("RawJSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		data := map[string]string{"raw": "data"}
		RawJSON(w, http.StatusOK, data)
		
		if w.Header().Get("Content-Type") != "application/json" {
			t.Errorf("RawJSON() Content-Type = %v, want application/json", w.Header().Get("Content-Type"))
		}
		
		var got map[string]string
		json.NewDecoder(w.Body).Decode(&got)
		if got["raw"] != "data" {
			t.Errorf("RawJSON() data = %v, want raw:data", got)
		}
	})
}

func TestWithPagination(t *testing.T) {
	resp := &Response{}
	opt := WithPagination(2, 25, 150)
	opt(resp)

	if resp.Meta == nil {
		t.Fatal("WithPagination() Meta = nil")
	}

	if resp.Meta.Page != 2 {
		t.Errorf("WithPagination() Page = %v, want 2", resp.Meta.Page)
	}

	if resp.Meta.PerPage != 25 {
		t.Errorf("WithPagination() PerPage = %v, want 25", resp.Meta.PerPage)
	}

	if resp.Meta.Total != 150 {
		t.Errorf("WithPagination() Total = %v, want 150", resp.Meta.Total)
	}

	if resp.Meta.TotalPages != 6 {
		t.Errorf("WithPagination() TotalPages = %v, want 6", resp.Meta.TotalPages)
	}
}