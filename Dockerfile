# syntax=docker/dockerfile:1.7

FROM golang:1.26.6-alpine AS dependencies
WORKDIR /src
RUN apk add --no-cache ca-certificates git tzdata
COPY go.mod go.sum ./
RUN go mod download

FROM dependencies AS test
COPY . .
RUN go test ./...
RUN go vet ./...

FROM dependencies AS build
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags "-s -w -X domainmonitor/internal/buildinfo.Version=${VERSION} -X domainmonitor/internal/buildinfo.Commit=${COMMIT} -X domainmonitor/internal/buildinfo.Date=${BUILD_DATE}" \
    -o /out/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/scheduler ./cmd/scheduler
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/worker ./cmd/worker
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/migrate ./cmd/migrate
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/seed ./cmd/seed
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags "-s -w -X domainmonitor/internal/buildinfo.Version=${VERSION} -X domainmonitor/internal/buildinfo.Commit=${COMMIT} -X domainmonitor/internal/buildinfo.Date=${BUILD_DATE}" \
    -o /out/probe ./cmd/probe

FROM alpine:3.23 AS runtime
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S -g 10001 app \
    && adduser -S -D -H -u 10001 -G app app \
    && mkdir -p /var/lib/domain-monitor-probe \
    && chown -R app:app /var/lib/domain-monitor-probe
WORKDIR /app
COPY --from=build /out/api /usr/local/bin/api
COPY --from=build /out/scheduler /usr/local/bin/scheduler
COPY --from=build /out/worker /usr/local/bin/worker
COPY --from=build /out/migrate /usr/local/bin/migrate
COPY --from=build /out/seed /usr/local/bin/seed
COPY --from=build /out/probe /usr/local/bin/probe
COPY migrations /app/migrations
USER app
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/health || exit 1
CMD ["/usr/local/bin/api"]
