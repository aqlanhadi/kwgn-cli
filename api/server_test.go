package api

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNew(t *testing.T) {
	cfg := DefaultConfig()
	server := New(cfg)

	if server == nil {
		t.Fatal("Expected server to be created")
	}
	if server.mux == nil {
		t.Fatal("Expected mux to be initialized")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Port != ":8080" {
		t.Errorf("Expected port ':8080', got '%s'", cfg.Port)
	}
}

func TestHealthEndpoint(t *testing.T) {
	server := New(DefaultConfig())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["status"] != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", response["status"])
	}
}

func TestExtractEndpoint_MethodNotAllowed(t *testing.T) {
	server := New(DefaultConfig())

	req := httptest.NewRequest(http.MethodGet, "/extract", nil)
	w := httptest.NewRecorder()

	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestExtractEndpoint_NoFile(t *testing.T) {
	server := New(DefaultConfig())

	req := httptest.NewRequest(http.MethodPost, "/extract", nil)
	req.Header.Set("Content-Type", "multipart/form-data")
	w := httptest.NewRecorder()

	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestExtractEndpoint_InvalidFile(t *testing.T) {
	server := New(DefaultConfig())

	// Create multipart form with invalid PDF
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.pdf")
	part.Write([]byte("not a valid pdf"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/extract", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	server.Handler().ServeHTTP(w, req)

	// Should return 200 with empty result (extractor handles invalid PDFs gracefully)
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestParseExtractOptions_FormValues(t *testing.T) {
	server := New(DefaultConfig())

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("statement_only", "true")
	writer.WriteField("statement_type", "MAYBANK_CASA_AND_MAE")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/extract", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.ParseMultipartForm(32 << 20)

	opts := server.parseExtractOptions(req)

	if !opts.StatementOnly {
		t.Error("Expected StatementOnly to be true")
	}
	if opts.StatementType != "MAYBANK_CASA_AND_MAE" {
		t.Errorf("Expected StatementType 'MAYBANK_CASA_AND_MAE', got '%s'", opts.StatementType)
	}
}

func TestParseExtractOptions_QueryParams(t *testing.T) {
	server := New(DefaultConfig())

	req := httptest.NewRequest(http.MethodPost, "/extract?transaction_only=true&text_only=true", nil)

	opts := server.parseExtractOptions(req)

	if !opts.TransactionOnly {
		t.Error("Expected TransactionOnly to be true")
	}
	if !opts.TextOnly {
		t.Error("Expected TextOnly to be true")
	}
}

func TestCoalesce(t *testing.T) {
	tests := []struct {
		input    []string
		expected string
	}{
		{[]string{"", "", "third"}, "third"},
		{[]string{"first", "second"}, "first"},
		{[]string{"", ""}, ""},
		{[]string{}, ""},
		{[]string{"only"}, "only"},
	}

	for _, tt := range tests {
		result := coalesce(tt.input...)
		if result != tt.expected {
			t.Errorf("coalesce(%v) = '%s', expected '%s'", tt.input, result, tt.expected)
		}
	}
}

func TestHandler(t *testing.T) {
	server := New(DefaultConfig())
	handler := server.Handler()

	if handler == nil {
		t.Fatal("Expected handler to be returned")
	}

	// Verify it's the same mux
	if handler != server.mux {
		t.Error("Expected handler to be the server's mux")
	}
}

// TestExtractEndpoint_WithMockPDF tests with a minimal valid-looking request
func TestExtractEndpoint_ContentType(t *testing.T) {
	server := New(DefaultConfig())

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "statement.pdf")
	io.WriteString(part, "mock content")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/extract", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	server.Handler().ServeHTTP(w, req)

	// Check content type of response
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
	}
}
