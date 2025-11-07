FROM alpine:3.20

ARG MIGRATE_VERSION=4.16.2

RUN addgroup -S migrate && adduser -S migrate -G migrate && \
    apk add --no-cache ca-certificates curl && \
    curl -sSL "https://github.com/golang-migrate/migrate/releases/download/v${MIGRATE_VERSION}/migrate.linux-amd64.tar.gz" \
    | tar -xz -C /usr/local/bin && \
    chmod +x /usr/local/bin/migrate

COPY db/migrations /migrations

USER migrate

ENV DATABASE_URL=""

ENTRYPOINT ["migrate"]
CMD ["-path", "/migrations", "-database", "${DATABASE_URL}", "up"]
