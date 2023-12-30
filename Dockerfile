FROM golang:latest as builder
WORKDIR /
COPY go.mod go.sum gosqueal.go /
RUN set -x\
    && GOOS=linux go build -ldflags "-s" gosqueal.go

FROM debian:trixie-slim 
COPY --from=builder /gosqueal /gosqueal
CMD ["/gosqueal"]
