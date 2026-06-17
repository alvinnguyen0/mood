FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /mood-tracker .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /mood-tracker /mood-tracker
ENV ADDR=":8080"
EXPOSE 8080
VOLUME ["/data"]
ENV DB_DSN="file:/data/mood.db?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)"
ENTRYPOINT ["/mood-tracker"]
