FROM ubuntu:22.04 AS builder

# Установка зависимостей
RUN apt-get update && apt-get install -y \
    wget \
    build-essential \
    pkg-config \
    libgstreamer1.0-dev \
    libgstreamer-plugins-base1.0-dev \
    libgstreamer-plugins-good1.0-dev \
    gstreamer1.0-plugins-bad \
    gstreamer1.0-plugins-ugly \
    gstreamer1.0-libav

# Установка Go 1.22
RUN wget https://go.dev/dl/go1.22.1.linux-amd64.tar.gz
RUN tar -C /usr/local -xzf go1.22.1.linux-amd64.tar.gz
ENV PATH="/usr/local/go/bin:${PATH}"

WORKDIR /build
COPY . .

# Компиляция
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /build/bin/main ./cmd/

# Финальный образ
FROM ubuntu:22.04

RUN apt-get update && apt-get install -y \
    libgstreamer1.0-0 \
    gstreamer1.0-plugins-base \
    gstreamer1.0-plugins-good \
    gstreamer1.0-plugins-bad \
    gstreamer1.0-plugins-ugly \
    gstreamer1.0-libav

WORKDIR /app
COPY --from=builder /build/bin/main .
COPY --from=builder /build/cmd/.env .

ENV GST_PLUGIN_SYSTEM_PATH=/usr/lib/x86_64-linux-gnu/gstreamer-1.0
ENV CGO_ENABLED=1

EXPOSE 8085
CMD ["./main"]