# Base image
FROM golang:1.20

# Set the working directory
WORKDIR /app

# Copy Go module files and download dependencies
COPY go.mod ./
RUN go mod download

# Copy the application source code
COPY . .

# Build the Go binary
RUN go build -o video-uploader main.go

# Set the entrypoint
CMD ["./video-uploader"]