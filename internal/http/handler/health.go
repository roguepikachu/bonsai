package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/roguepikachu/bonsai/pkg"
)

func HealthCheck(c *gin.Context) {
	response := pkg.NewResponse(http.StatusOK, nil, "pong")
	c.JSON(response.Code, response)
}