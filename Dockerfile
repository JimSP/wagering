FROM golang:1.27.1-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/wagering ./cmd/wagering
RUN CGO_ENABLED=0 go build -trimpath -o /out/healthcheck ./cmd/healthcheck

FROM haproxy:3.2.25-alpine AS gateway
COPY --from=build /out/healthcheck /healthcheck
COPY deploy/haproxy.cfg /usr/local/etc/haproxy/haproxy.cfg

FROM gcr.io/distroless/static-debian12:nonroot AS app
COPY --from=build /out/wagering /wagering
COPY --from=build /out/healthcheck /healthcheck
COPY migrations /migrations
ENTRYPOINT ["/wagering"]
