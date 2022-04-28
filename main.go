package main

import (
	"theresa-go/cmd/service"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load(".env")
	service.Bootstrap()
}
