FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install git and other dependencies if needed
RUN apk add --no-cache git

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o kwgn main.go

# Final stage
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for HTTPS requests if needed
RUN apk add --no-cache ca-certificates

COPY --from=builder /app/kwgn .

EXPOSE 8080

ENTRYPOINT ["./kwgn"]
CMD ["serve"]





