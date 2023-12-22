FROM golang:latest as builder
WORKDIR /
COPY go.mod go.sum gosqueal.go /
RUN set -x\
    && GOOS=linux go build go build -ldflags="-extldflags=-static" -tags sqlite_omit_load_extension gosqueal.go

FROM scratch
COPY --from=builder /gosqueal.go /gosqueal
CMD ["/gosqueal"]
