FROM golang:1.22 as builder
WORKDIR /
COPY go.mod go.sum gosqueal.go /
RUN set -x\
    && GOOS=linux go build -ldflags "-linkmode 'external' -extldflags '-static'" gosqueal.go

FROM scratch 
COPY --from=builder /gosqueal /gosqueal
CMD ["/gosqueal"]
