FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN for c in api seed import-candidates admin; do \
      CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/bin/$c ./cmd/$c || exit 1; \
    done && mkdir -p /out/data/uploads

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/bin /app/bin
COPY --from=build --chown=65532:65532 /out/data /app/data
COPY seed /app/seed
ENV UPLOAD_DIR=/app/data/uploads ADDR=:8080
EXPOSE 8080
ENTRYPOINT ["/app/bin/api"]
