FROM golang:1.26.5-alpine AS build
WORKDIR /app
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN mkdir -p dist && CGO_ENABLED=0 go build -ldflags="-s -w" -o dist ./cmd/migrate

FROM alpine:3.24.1 AS final
WORKDIR /app
COPY --from=build app/dist/migrate .
RUN adduser -D -u 10001 appuser
USER 10001:10001
CMD ["./migrate"]
