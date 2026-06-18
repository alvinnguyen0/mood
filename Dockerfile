# Build stage
FROM golang:1.25-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /mood-tracker .

# Final stage
FROM scratch
COPY --from=build /mood-tracker /mood-tracker
ENTRYPOINT ["/mood-tracker"]
