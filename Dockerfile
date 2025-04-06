FROM golang:latest

# Set the Current Working Directory inside the container
WORKDIR /app
# Copy everything from the current directory to the working directory inside the container
COPY . .
# Install dependencies
RUN go mod download

# Build the Go app
RUN go build -o etherpad-proxy .

# Expose port 8080 to the outside world
EXPOSE 9000

FROM gcr.io/distroless/base-debian12
# Copy the Pre-built binary file from the previous stage
COPY --from=0 /app/etherpad-proxy /app/etherpad-proxy
# Command to run the executable
ENTRYPOINT ["/app/etherpad-proxy"]
