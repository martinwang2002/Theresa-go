FROM golang:alpine AS builder
RUN apk update && apk --no-cache add \
    build-base \
    ca-certificates \
    git \
    vips-dev \
    ffmpeg
WORKDIR /app
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN go build .

FROM alpine:latest
RUN apk update && apk --no-cache add \
    ca-certificates \
    vips-dev \
    ffmpeg
WORKDIR /app
COPY --from=builder /app ./
EXPOSE 8000
CMD ["./theresa-go"]
