package twofa

import (
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
)

func Register2FARoutes(e *core.ServeEvent, app *pocketbase.PocketBase) {
	e.Router.POST("/api/collections/:collection/2fa/:method", func(c echo.Context) error {
		return handlePostMethodAssign(c, app)
	})
	e.Router.GET("/api/collections/:collection/2fa/:method", func(c echo.Context) error {
		return handleGetMethodAssign(c, app)
	})
}

func handlePostMethodAssign(c echo.Context, app *pocketbase.PocketBase) error {
	// Get current user from an auth record
	switch c.PathParam("method") {
	case "enable":
		return enable2FA(app, c)
	case "disable":
		return disable2FA(app, c)
	case "finish-setup":
		return finish2FASetup(app, c)
	}
	return apis.NewNotFoundError("Method not found", nil)
}

func handleGetMethodAssign(c echo.Context, app *pocketbase.PocketBase) error {
	// Get current user from an auth record
	switch c.PathParam("method") {
	case "state":
		return get2FAState(app, c)
	}
	return apis.NewNotFoundError("Method not found", nil)
}

func enable2FA(app *pocketbase.PocketBase, c echo.Context) error {
	record, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

	if record == nil {
		return apis.NewForbiddenError("", nil)
	}

	//Load the record into 2FA

	otp, err := Create(app, record)
	if err != nil {
		return apis.NewApiError(500, err.Error(), nil)
	}

	res := make(map[string]interface{})

	res["code"] = 200

	res["secret"] = otp.record.secret
	res["url"] = otp.record.url
	res["state"] = true
	res["message"] = "Code verification required"

	return c.JSON(200, res)
}

func disable2FA(app *pocketbase.PocketBase, c echo.Context) error {
	record, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

	if record == nil {
		return apis.NewForbiddenError("", nil)
	}

	//Load the record into 2FA

	otp, err := Load(app, record)
	if err != nil {
		return apis.NewApiError(500, err.Error(), nil)
	}
	if err := otp.Disable("", true); err != nil {
		return apis.NewApiError(500, err.Error(), nil)
	}
	res := make(map[string]interface{})

	res["code"] = 200

	res["state"] = false
	res["message"] = "2FA no longer enabled"
	return c.JSON(200, res)

}
func finish2FASetup(app *pocketbase.PocketBase, c echo.Context) error {
	record, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

	if record == nil {
		return apis.NewForbiddenError("", nil)
	}

	//Load the record into 2FA

	otp, err := Load(app, record)
	if err != nil {
		return apis.NewApiError(500, err.Error(), nil)
	}
	if err := otp.Enable(c.FormValue("code")); err != nil {
		return apis.NewApiError(500, err.Error(), nil)
	}
	res := make(map[string]interface{})

	res["code"] = 200

	res["state"] = true
	res["message"] = "2FA enabled"
	return c.JSON(200, res)

}

func get2FAState(app *pocketbase.PocketBase, c echo.Context) error {
	record, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

	if record == nil {
		return apis.NewForbiddenError("", nil)
	}

	//Load the record into 2FA

	res := make(map[string]interface{})

	res["code"] = 200

	_, err := Load(app, record)
	if err != nil {
		res["state"] = false
		res["message"] = "2FA not enabled"
		return c.JSON(200, res)
	} else {
		res["state"] = true
		res["message"] = "2FA enabled"
		return c.JSON(200, res)
	}
}
