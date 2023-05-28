FROM golang:alpine AS builder
RUN apk --no-cache add \
    build-base \
    ca-certificates \
    git \
    vips-dev \
    ffmpeg
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -ldflags "-w" .

FROM alpine:latest
RUN apk --no-cache add \
    ca-certificates \
    vips-dev \
    ffmpeg
WORKDIR /app
COPY --from=builder /app/resources ./resources
COPY --from=builder /app/theresa-go ./
EXPOSE 8000
CMD ["./theresa-go"]
