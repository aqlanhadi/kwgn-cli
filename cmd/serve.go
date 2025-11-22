package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/aqlanhadi/kwgn/extractor"
	"github.com/aqlanhadi/kwgn/extractor/common"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start HTTP API server",
	Run: func(cmd *cobra.Command, args []string) {
		// Force logging to stdout for server to aid debugging
		log.SetOutput(os.Stdout)
		log.SetFlags(log.Ltime | log.Lmsgprefix)
		log.SetPrefix("SERVER: ")

		http.HandleFunc("/extract", func(w http.ResponseWriter, r *http.Request) {
			log.Printf("Received request from %s", r.RemoteAddr)

			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}

			// Parse multipart form with 32MB max memory
			if err := r.ParseMultipartForm(32 << 20); err != nil {
				log.Printf("Error parsing multipart form: %v", err)
				http.Error(w, "Could not parse multipart form: "+err.Error(), http.StatusBadRequest)
				return
			}

			file, handler, err := r.FormFile("file")
			if err != nil {
				log.Printf("Error getting file from form: %v", err)
				http.Error(w, "Could not get uploaded file: "+err.Error(), http.StatusBadRequest)
				return
			}
			defer file.Close()

			// Read file into memory
			fileBytes, err := io.ReadAll(file)
			if err != nil {
				log.Printf("Error reading file bytes: %v", err)
				http.Error(w, "Could not read file: "+err.Error(), http.StatusInternalServerError)
				return
			}

			fileReader := bytes.NewReader(fileBytes)

			// Extract flags
			statementOnly := r.FormValue("statement_only") == "true" || r.URL.Query().Get("statement_only") == "true"
			transactionOnly := r.FormValue("transaction_only") == "true" || r.URL.Query().Get("transaction_only") == "true"
			textOnly := r.FormValue("text_only") == "true" || r.URL.Query().Get("text_only") == "true"
			statementType := r.FormValue("statement_type")
			if statementType == "" {
				statementType = r.URL.Query().Get("statement_type")
			}

			if textOnly {
				rows, err := common.ExtractRowsFromPDFReader(fileReader)
				if err != nil || len(*rows) < 1 {
					log.Printf("Error extracting text: %v", err)
					http.Error(w, "Could not extract text from file: "+err.Error(), http.StatusBadRequest)
					return
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{
					"filename": handler.Filename,
					"text":     strings.Join(*rows, "\n"),
				})
				return
			}

			// Reset reader
			fileReader.Seek(0, io.SeekStart)

			// Process
			result := extractor.ProcessReader(fileReader, handler.Filename, statementType)

			finalOutput := extractor.CreateFinalOutput(result, transactionOnly, statementOnly)

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(finalOutput)
		})

		port := ":8080"
		log.Printf("Starting API server on %s", port)
		if err := http.ListenAndServe(port, nil); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
