FROM golang:1.22 AS builder
WORKDIR /src
COPY . .
RUN go build -o /app

FROM alpine:3.19 AS runtime
COPY --from=builder /app /app
EXPOSE 8080
ENTRYPOINT ["/app"]
