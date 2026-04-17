# syntax=docker/dockerfile:1.7

FROM golang:1.26-bookworm AS build
WORKDIR /src

COPY go.mod ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

ARG TARGETOS=linux
ARG TARGETARCH=amd64
ENV CGO_ENABLED=0

RUN --mount=type=cache,target=/root/.cache/go-build \
    GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath=false -buildvcs=true -o /out/authd ./cmd/authd && \
    GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath=false -buildvcs=true -o /out/gamed ./cmd/gamed

FROM gcr.io/distroless/static-debian12:nonroot AS runtime
WORKDIR /app
COPY --from=build /out/authd /app/authd
COPY --from=build /out/gamed /app/gamed
ENV METIN2_PPROF_ADDR=:6060
EXPOSE 6060 6061
USER nonroot:nonroot
ENTRYPOINT ["/app/gamed"]

FROM gcr.io/distroless/static-debian12:debug-nonroot AS runtime-debug
WORKDIR /app
COPY --from=build /out/authd /app/authd
COPY --from=build /out/gamed /app/gamed
ENV METIN2_PPROF_ADDR=:6060
EXPOSE 6060 6061
USER nonroot:nonroot
ENTRYPOINT ["/app/gamed"]
