# Build from source: docker build -t stacklit .
# Run on a repo:    docker run --rm -v "$PWD":/work ghcr.io/glincker/stacklit init
FROM golang:1.25-alpine AS build
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X github.com/glincker/stacklit/internal/cli.Version=${VERSION}" \
    -o /out/stacklit ./cmd/stacklit

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/stacklit /usr/local/bin/stacklit
ENV HOME=/tmp
WORKDIR /work
ENTRYPOINT ["/usr/local/bin/stacklit"]
CMD ["--help"]
