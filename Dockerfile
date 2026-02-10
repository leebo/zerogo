FROM golang:1.24-bookworm AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o /bin/zerogo-agent ./cmd/zerogo-agent
RUN CGO_ENABLED=1 GOOS=linux go build -o /bin/zerogo-controller ./cmd/zerogo-controller
RUN CGO_ENABLED=1 GOOS=linux go build -o /bin/zerogo-relay ./cmd/zerogo-relay
RUN CGO_ENABLED=1 GOOS=linux go build -o /bin/zerogo-cli ./cmd/zerogo-cli

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    iproute2 \
    iputils-ping \
    arping \
    iperf3 \
    tcpdump \
    net-tools \
    procps \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /bin/zerogo-agent /usr/local/bin/
COPY --from=builder /bin/zerogo-controller /usr/local/bin/
COPY --from=builder /bin/zerogo-relay /usr/local/bin/
COPY --from=builder /bin/zerogo-cli /usr/local/bin/

RUN mkdir -p /etc/zerogo /var/lib/zerogo

ENTRYPOINT ["zerogo-agent"]
