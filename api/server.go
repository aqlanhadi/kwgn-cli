// Package api provides HTTP API capabilities for the kwgn extractor.
// This is a capability module that can be enabled via the CLI or used programmatically.
package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/aqlanhadi/kwgn/extractor"
	"github.com/aqlanhadi/kwgn/extractor/common"
)

// Config holds the API server configuration
type Config struct {
	Port            string
	DefaultTextOnly bool
	LogPrefix       string
}

// DefaultConfig returns the default API configuration
func DefaultConfig() Config {
	return Config{
		Port:      ":8080",
		LogPrefix: "API: ",
	}
}

// Server represents the HTTP API server
type Server struct {
	config Config
	mux    *http.ServeMux
}

// New creates a new API server with the given configuration
func New(cfg Config) *Server {
	s := &Server{
		config: cfg,
		mux:    http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

// registerRoutes sets up the API endpoints
func (s *Server) registerRoutes() {
	s.mux.HandleFunc("/extract", s.handleExtract)
	s.mux.HandleFunc("/health", s.handleHealth)
}

// Handler returns the http.Handler for the server
// This allows the server to be used with custom http.Server configurations
func (s *Server) Handler() http.Handler {
	return s.mux
}

// Start starts the HTTP server (blocking)
func (s *Server) Start() error {
	log.Printf("%sStarting server on %s", s.config.LogPrefix, s.config.Port)
	return http.ListenAndServe(s.config.Port, s.mux)
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleExtract handles PDF extraction requests
func (s *Server) handleExtract(w http.ResponseWriter, r *http.Request) {
	log.Printf("%sReceived request from %s", s.config.LogPrefix, r.RemoteAddr)

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form with 32MB max memory
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		log.Printf("%sError parsing multipart form: %v", s.config.LogPrefix, err)
		http.Error(w, "Could not parse multipart form: "+err.Error(), http.StatusBadRequest)
		return
	}

	file, handler, err := r.FormFile("file")
	if err != nil {
		log.Printf("%sError getting file from form: %v", s.config.LogPrefix, err)
		http.Error(w, "Could not get uploaded file: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Read file into memory
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		log.Printf("%sError reading file bytes: %v", s.config.LogPrefix, err)
		http.Error(w, "Could not read file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	fileReader := bytes.NewReader(fileBytes)

	// Extract flags from request
	opts := s.parseExtractOptions(r)

	if opts.TextOnly {
		s.handleTextOnlyExtract(w, fileReader, handler.Filename)
		return
	}

	// Reset reader and process
	fileReader.Seek(0, io.SeekStart)
	result := extractor.ProcessReader(fileReader, handler.Filename, opts.StatementType)
	finalOutput := extractor.CreateFinalOutput(result, opts.TransactionOnly, opts.StatementOnly)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(finalOutput)
}

// ExtractOptions holds the options for extraction
type ExtractOptions struct {
	StatementOnly   bool
	TransactionOnly bool
	TextOnly        bool
	StatementType   string
}

// parseExtractOptions extracts options from the HTTP request
func (s *Server) parseExtractOptions(r *http.Request) ExtractOptions {
	return ExtractOptions{
		StatementOnly:   r.FormValue("statement_only") == "true" || r.URL.Query().Get("statement_only") == "true",
		TransactionOnly: r.FormValue("transaction_only") == "true" || r.URL.Query().Get("transaction_only") == "true",
		TextOnly:        r.FormValue("text_only") == "true" || r.URL.Query().Get("text_only") == "true",
		StatementType:   coalesce(r.FormValue("statement_type"), r.URL.Query().Get("statement_type")),
	}
}

// handleTextOnlyExtract handles text-only extraction mode
func (s *Server) handleTextOnlyExtract(w http.ResponseWriter, reader *bytes.Reader, filename string) {
	rows, err := common.ExtractRowsFromPDFReader(reader)
	if err != nil || len(*rows) < 1 {
		log.Printf("%sError extracting text: %v", s.config.LogPrefix, err)
		http.Error(w, "Could not extract text from file: "+err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"filename": filename,
		"text":     strings.Join(*rows, "\n"),
	})
}

// coalesce returns the first non-empty string
func coalesce(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
