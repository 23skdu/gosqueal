FROM golang:1.25.5 as builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /gosqueal ./cmd/gosqueal

FROM scratch
COPY --from=builder /gosqueal /gosqueal
CMD ["/gosqueal"]
