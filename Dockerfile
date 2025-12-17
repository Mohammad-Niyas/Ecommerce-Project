# ---------- BUILD STAGE ----------
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod files first
COPY go.mod go.sum ./
RUN go mod download

# Copy entire source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ecommerce-app

# ---------- RUNTIME STAGE ----------
FROM alpine:latest

WORKDIR /app

# Copy built binary
COPY --from=builder /app/ecommerce-app .

# Copy required folders
COPY --from=builder /app/views ./views
COPY --from=builder /app/utils ./utils

# App runs on 8080
EXPOSE 8080

# Start the app
CMD ["./ecommerce-app"]
