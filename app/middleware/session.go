package middleware

import (
	"github.com/gin-gonic/gin"
	"core/app/constants"
	"core/app/models"
	"core/common/app"
)

func RequireConsoleLogin() gin.HandlerFunc {
	return func(c *gin.Context) {
		var err error
		handler := app.NewHandler(c)
		item := models.Staff{}
		err = handler.GetSessionVal(constants.ConsoleSessionName, constants.StaffInfo, &item)
		if err != nil {
			handler.ResponseError(err)
			c.Abort()
			return
		}
		c.Next()
	}
}

func RequireStaffLogin() gin.HandlerFunc {
	return func(c *gin.Context) {
		var err error
		handler := app.NewHandler(c)
		item := models.Staff{}
		err = handler.GetSessionVal(constants.StaffSessionName, constants.StaffInfo, &item)
		if err != nil {
			handler.ResponseError(err)
			c.Abort()
			return
		}
		c.Next()
	}
}

func RequireCustomerLogin() gin.HandlerFunc {
	return func(c *gin.Context) {
		var err error
		handler := app.NewHandler(c)
		item := models.Customer{}
		err = handler.GetSessionVal(constants.CustomerSessionName, constants.CustomerInfo, &item)
		if err != nil {
			handler.ResponseError(err)
			c.Abort()
			return
		}
		c.Next()
	}
}
