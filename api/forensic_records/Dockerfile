FROM golang:1.26-bookworm AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY pkg ./pkg
COPY api/forensic_records ./api/forensic_records
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/forensic-records-api ./api/forensic_records

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/forensic-records-api /forensic-records-api
# The curated semantic layer travels with the binary. It is OPTIONAL at runtime —
# the compiler falls back to the inferred catalogue when it is absent — but
# shipping it is what turns its verified definitions into answers.
COPY semantic_layer /semantic_layer

EXPOSE 8091
ENTRYPOINT ["/forensic-records-api"]
