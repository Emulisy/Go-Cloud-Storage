# Build from the goCloudStorage directory:
# docker build -f dockerfile -t gocloudstorage .
FROM golang:1.26.4-alpine AS build

WORKDIR /src
ENV CGO_ENABLED=0 GOTOOLCHAIN=local

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -mod=readonly -trimpath -ldflags="-s -w" -o /out/gocloudstorage .

FROM alpine:3.23

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S -g 10001 app \
    && adduser -S -D -H -u 10001 -G app app

WORKDIR /app
COPY --from=build /out/gocloudstorage /app/gocloudstorage
COPY --from=build /src/static /app/static

USER app:app
EXPOSE 8080
ENTRYPOINT ["/app/gocloudstorage"]
