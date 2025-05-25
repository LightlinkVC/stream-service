FROM kurento/kurento-media-server:6.17

RUN apt-get update && \
    apt-get install -y curl ca-certificates && \
    rm -rf /var/lib/apt/lists/*

RUN curl -L https://github.com/vi/websocat/releases/download/v1.10.0/websocat.x86_64-unknown-linux-musl -o /usr/local/bin/websocat && \
    chmod +x /usr/local/bin/websocat
