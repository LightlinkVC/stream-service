FROM golang:alpine AS builder
LABEL stage=gobuilder

ENV CGO_ENABLED 0
ENV GOOS linux

RUN apk update --no-cache && apk add --no-cache tzdata

WORKDIR /build

COPY ./go.mod .
COPY ./go.sum .

RUN go mod download

COPY . .

RUN go build -ldflags="-s -w" -o /build/bin/main ./cmd/

FROM alpine

RUN apk update --no-cache

WORKDIR /stream-service/bin

COPY --from=builder /build/cmd/.env .
COPY --from=builder /build/bin/main .

EXPOSE 8085

CMD ["./main"]