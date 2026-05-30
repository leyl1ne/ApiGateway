package response

import "github.com/gin-gonic/gin"

type Error struct {
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error Error `json:"error"`
}

func WriteError(c *gin.Context, status int, message string) {
	c.JSON(status, ErrorResponse{
		Error: Error{
			Message: message,
		},
	})
}

func WriteErrorAbort(c *gin.Context, status int, msg string) {
	c.AbortWithStatusJSON(status, ErrorResponse{
		Error: Error{
			Message: msg,
		},
	})
}
