FROM golang:latest as builder
WORKDIR /
COPY go.mod go.sum gosqueal.go /
RUN set -x\
    && CGO_ENABLED=0 GOOS=linux go build -ldflags "-s" gosqueal.go

FROM scratch 
COPY --from=builder /gosqueal /gosqueal
CMD ["/gosqueal"]
