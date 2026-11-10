# syntax=docker/dockerfile:1
FROM golang:1.26-bookworm AS builder

WORKDIR /src
COPY go.mod go.sum ./
# The module cache is a build cache mount, so a download that the network cuts off
# keeps what it already fetched and the next attempt resumes instead of starting over.
RUN --mount=type=cache,target=/go/pkg/mod \
    for i in 1 2 3 4 5; do go mod download && exit 0; echo "go mod download failed (attempt $i); retrying" >&2; sleep 5; done; exit 1
COPY pkg ./pkg
# The case layer (conversations, notes, findings, members, audit) is its own package; api/forensic_records only registers its routes (case_layer.go).
COPY internal/caselayer ./internal/caselayer
COPY api/forensic_records ./api/forensic_records
# -p 2: at most two compile jobs at once. On the 16 GB laptop the Docker VM (7.6 GB) also holds the database and the model server, and an unrestricted compile froze it on 2026-11-10.
RUN --mount=type=cache,target=/go/pkg/mod CGO_ENABLED=0 go build -p 2 -trimpath -ldflags="-s -w" -o /out/forensic-records-api ./api/forensic_records

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/forensic-records-api /forensic-records-api
# The curated semantic layer travels with the binary. It is OPTIONAL at runtime —
# the compiler falls back to the inferred catalogue when it is absent — but
# shipping it is what turns its verified definitions into answers.
COPY semantic_layer /semantic_layer

EXPOSE 8091
ENTRYPOINT ["/forensic-records-api"]
