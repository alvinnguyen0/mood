# Build stage
FROM golang:1.25-alpine AS build
ARG VERSION=dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /mood-tracker .

# Final stage
FROM scratch
COPY --from=build /mood-tracker /mood-tracker
ENTRYPOINT ["/mood-tracker"]
