# syntax=docker/dockerfile:1.7

FROM docker.io/library/golang:1.26-bookworm AS build
WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .

ARG TARGETOS=linux
ARG TARGETARCH=amd64
ENV CGO_ENABLED=0

RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath=false -buildvcs=true -o /out/authd ./cmd/authd && \
    GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath=false -buildvcs=true -o /out/gamed ./cmd/gamed && \
    GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath=false -buildvcs=true -o /out/metin2-migrate ./cmd/metin2-migrate

FROM gcr.io/distroless/static-debian12:nonroot AS runtime
WORKDIR /app
COPY --from=build /out/authd /app/authd
COPY --from=build /out/gamed /app/gamed
COPY --from=build /out/metin2-migrate /app/metin2-migrate
ENV METIN2_GAMED_PPROF_ADDR=127.0.0.1:6060 \
    METIN2_AUTHD_PPROF_ADDR=127.0.0.1:6061
USER nonroot:nonroot
ENTRYPOINT ["/app/gamed"]

FROM gcr.io/distroless/static-debian12:debug-nonroot AS runtime-debug
WORKDIR /app
COPY --from=build /out/authd /app/authd
COPY --from=build /out/gamed /app/gamed
COPY --from=build /out/metin2-migrate /app/metin2-migrate
ENV METIN2_GAMED_PPROF_ADDR=127.0.0.1:6060 \
    METIN2_AUTHD_PPROF_ADDR=127.0.0.1:6061
USER nonroot:nonroot
ENTRYPOINT ["/app/gamed"]
