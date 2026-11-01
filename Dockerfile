# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS build

RUN apk add --no-cache git ca-certificates
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

# Render the markdown from the exact gothic release the site is built against.
RUN set -eux; \
	version="$(go list -m -f '{{.Version}}' github.com/metruzanca/gothic)"; \
	git clone --depth 1 --branch "$version" https://github.com/metruzanca/gothic.git /gothic

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-w -s" -o /out/gothic.run .

FROM alpine:3.22

RUN adduser -D -u 10001 app
WORKDIR /app

COPY --from=build /out/gothic.run ./gothic.run
COPY --from=build /gothic/README.md ./content/README.md
COPY --from=build /gothic/docs ./content/docs

ENV PORT=8080
ENV DOCS_DIR=/app/content

USER app
EXPOSE 8080

CMD ["./gothic.run"]
