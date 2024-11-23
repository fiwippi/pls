# -- Build pls
FROM golang:1.22-alpine as builder

WORKDIR /app
COPY . .

RUN CGO_ENABLED=0 go build -o ./bin/pls main.go

# -- Run pls
FROM alpine:latest

COPY --from=builder /app/bin/pls /bin/pls

ENTRYPOINT ["/bin/pls"]