package main

import (
	"github.com/roguepikachu/bonsai/internal/http/router"
)

func main() {
	router := router.Router()
	router.Run("localhost:8080")
}
