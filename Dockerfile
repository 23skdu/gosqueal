FROM golang:latest as builder
WORKDIR /tmp
COPY gosqueal.go /tmp
RUN CGO_ENABLED=0 GOOS=linux go build gosqueal.go

FROM scratch
COPY --from=builder /tmp/gosqueal.go /gosqueal
CMD ["/gosqueal"]
