# syntax=docker/dockerfile:1.4
FROM --platform=linux/arm64 ubuntu:18.04 AS builder

RUN apt update && apt install -y        \
    wget gcc libc6-dev libpam0g-dev  && \
    apt clean                        && \
    rm -rf /var/lib/apt/lists/*

RUN wget -O /tmp/go.tar.gz https://go.dev/dl/go1.24.3.linux-arm64.tar.gz && \
    tar -C /usr/local -xzf /tmp/go.tar.gz && \
    rm /tmp/go.tar.gz
ENV PATH="/usr/local/go/bin:${PATH}"

WORKDIR /app
COPY . .

ENV GOOS=linux
ENV GOARCH=arm64
ENV CGO_ENABLED=1

RUN go build -o /out/baddoor cmd/baddoor/main.go
