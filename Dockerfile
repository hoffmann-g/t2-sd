# Image of the "gerador" container: the Go tools of this project, which reach
# the nodes through the client network.
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -o /out/ ./cmd/loadgen ./cmd/stale

FROM alpine:3.22
COPY --from=build /out/ /usr/local/bin/
WORKDIR /results
CMD ["sleep", "infinity"]
