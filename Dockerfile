# Build.
FROM golang:1.26-alpine AS build
WORKDIR /src

# Dependencies first, so a code change does not re-download the module cache.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# CGO off gives a static binary, which is what lets the final image be scratch-thin.
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/api .

# Run.
FROM alpine:3.21
# Certificates for talking to SMTP over TLS, and a timezone database so
# timestamps in the log read as JST rather than UTC.
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 10001 app
ENV TZ=Asia/Tokyo

COPY --from=build /out/api /usr/local/bin/api

# Handouts live here. Mount a volume over it, or the images go when the
# container does.
RUN mkdir -p /data/prints && chown -R app:app /data
VOLUME /data
ENV BLOB_DIR=/data/prints

USER app
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/api"]
