FROM golang:1.24-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

FROM alpine:latest

RUN apk --no-cache add ca-certificates
RUN adduser -D -s /bin/sh appuser
RUN mkdir -p /data && chown appuser:appuser /data

WORKDIR /root/

COPY --from=builder /app/main .
RUN chown appuser:appuser main
USER appuser
VOLUME ["/data"]
EXPOSE 8080

CMD ["./main"]
