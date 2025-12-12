FROM golang:1.25.5 as builder

WORKDIR /app

# Install build dependencies
RUN apt-get update && apt-get install -y wget gcc libc6-dev

# Download sqlite-vss extensions
RUN mkdir -p /deps
RUN wget -O /deps/vss0.so https://github.com/asg017/sqlite-vss/releases/download/v0.1.2/vss0-linux-x86_64.so
RUN wget -O /deps/vector0.so https://github.com/asg017/sqlite-vss/releases/download/v0.1.2/vector0-linux-x86_64.so

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build with CGO enabled
RUN CGO_ENABLED=1 GOOS=linux go build -o /app/binary ./cmd/gosqueal

FROM debian:bookworm-slim

WORKDIR /app

# Install runtime dependencies for sqlite-vss (libgomp1 is often needed for vector operations)
RUN apt-get update && apt-get install -y libgomp1 ca-certificates && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/binary .
COPY --from=builder /deps/vss0.so /usr/lib/vss0.so
COPY --from=builder /deps/vector0.so /usr/lib/vector0.so

CMD ["/app/binary"]
