# Build stage
FROM golang:1.20-alpine AS builder
WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./

# Runtime stage
FROM alpine:3.18
RUN apk add --no-cache ca-certificates ffmpeg
WORKDIR /app

# Create required directories
RUN mkdir -p /app/videos /app/temp

COPY --from=builder /app/server /app/server

EXPOSE 8080
ENTRYPOINT ["/app/server"]
