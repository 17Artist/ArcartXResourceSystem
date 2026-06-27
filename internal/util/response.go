// SPDX-License-Identifier: GPL-3.0-or-later

package util

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
}

func OK(c *gin.Context, data interface{}, message string) {
	c.JSON(http.StatusOK, Response{
		Success: true,
		Data:    data,
		Message: message,
	})
}

func OKMsg(c *gin.Context, message string) {
	c.JSON(http.StatusOK, Response{
		Success: true,
		Message: message,
	})
}

func Fail(c *gin.Context, status int, err string) {
	c.JSON(status, Response{
		Success: false,
		Error:   err,
	})
}

func BadRequest(c *gin.Context, err string) {
	Fail(c, http.StatusBadRequest, err)
}

func Unauthorized(c *gin.Context, err string) {
	Fail(c, http.StatusUnauthorized, err)
}

func Forbidden(c *gin.Context, err string) {
	Fail(c, http.StatusForbidden, err)
}

func NotFound(c *gin.Context, err string) {
	Fail(c, http.StatusNotFound, err)
}

func TooManyRequests(c *gin.Context, err string) {
	Fail(c, http.StatusTooManyRequests, err)
}

func InternalError(c *gin.Context, err string) {
	Fail(c, http.StatusInternalServerError, err)
}
