FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY mood-tracker /mood-tracker
ENV ADDR=":8080"
EXPOSE 8080
VOLUME ["/data"]
ENV DB_DSN="file:/data/mood.db?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)"
ENTRYPOINT ["/mood-tracker"]
