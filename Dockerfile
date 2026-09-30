FROM golang:1.27.1-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/wagering ./cmd/wagering

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/wagering /wagering
COPY migrations /migrations
ENTRYPOINT ["/wagering"]
