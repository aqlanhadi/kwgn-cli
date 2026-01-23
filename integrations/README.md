# Integrations

This directory contains integration modules for connecting kwgn to external services.

## Architecture

Each integration should be a separate package that:
1. Imports and uses the `extractor` package for core functionality
2. Provides its own configuration and initialization
3. Can be wired up via CLI commands in `cmd/`

## Adding a New Integration

1. Create a new directory: `integrations/myservice/`
2. Implement the integration logic
3. Add a CLI command in `cmd/` to enable it

### Example Structure

```
integrations/
├── webhook/
│   ├── webhook.go       # Webhook sender implementation
│   └── webhook_test.go
├── database/
│   ├── postgres.go      # Direct DB import
│   └── postgres_test.go
└── cloud/
    ├── s3.go            # S3 upload integration
    └── gcs.go           # GCS upload integration
```

### Example Integration (Webhook)

```go
// integrations/webhook/webhook.go
package webhook

import (
    "bytes"
    "encoding/json"
    "net/http"
    
    "github.com/aqlanhadi/kwgn/extractor"
    "github.com/aqlanhadi/kwgn/extractor/common"
)

type Config struct {
    URL     string
    Headers map[string]string
}

type Client struct {
    config Config
    http   *http.Client
}

func New(cfg Config) *Client {
    return &Client{
        config: cfg,
        http:   &http.Client{},
    }
}

func (c *Client) SendStatement(stmt common.Statement) error {
    output := extractor.CreateFinalOutput(stmt, false, false)
    body, _ := json.Marshal(output)
    
    req, _ := http.NewRequest("POST", c.config.URL, bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    for k, v := range c.config.Headers {
        req.Header.Set(k, v)
    }
    
    _, err := c.http.Do(req)
    return err
}
```

### Wiring Up via CLI

```go
// cmd/webhook.go
package cmd

import (
    "github.com/aqlanhadi/kwgn/integrations/webhook"
    "github.com/spf13/cobra"
)

var webhookCmd = &cobra.Command{
    Use:   "webhook",
    Short: "Send extracted data to a webhook",
    Run: func(cmd *cobra.Command, args []string) {
        // Implementation
    },
}

func init() {
    rootCmd.AddCommand(webhookCmd)
}
```

## Guidelines

- Keep integrations loosely coupled from core extraction logic
- Each integration should have its own tests
- Use interfaces where possible for easier testing/mocking
- Document configuration options
