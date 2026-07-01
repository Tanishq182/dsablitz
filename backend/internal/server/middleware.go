package server

import "github.com/gin-gonic/gin"

func middlewares() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		gin.Logger(),
		gin.Recovery(),
	}
}
