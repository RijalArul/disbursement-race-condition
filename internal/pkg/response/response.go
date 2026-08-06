package response

import (
	"github.com/gin-gonic/gin"

	respconst "github.com/RijalArul/disbursement-race-condition/internal/constants/response"
)

func OK(c *gin.Context, status int, data interface{}) {
	c.JSON(status, respconst.Envelope{Success: true, Data: data})
}

func OKWithMeta(c *gin.Context, status int, data interface{}, meta interface{}) {
	c.JSON(status, respconst.Envelope{Success: true, Data: data, Meta: meta})
}

func Err(c *gin.Context, status int, code, message string) {
	c.JSON(status, respconst.Envelope{Success: false, Error: &respconst.ErrorBody{Code: code, Message: message}})
}
