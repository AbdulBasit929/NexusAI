FROM golang:1.26-bookworm AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY pkg/httpclient ./pkg/httpclient
COPY api/forensic_records ./api/forensic_records
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/forensic-records-api ./api/forensic_records

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/forensic-records-api /forensic-records-api

EXPOSE 8091
ENTRYPOINT ["/forensic-records-api"]
