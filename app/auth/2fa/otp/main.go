package otp

import (
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/tools/security"
)

func RegisterOTPRoutes(e *core.ServeEvent, app *pocketbase.PocketBase) {
	e.Router.POST("/api/collections/:collection/2fa/:method", func(c echo.Context) error {
		return handlePostMethodAsign(c, app)
	})
	e.Router.GET("/api/collections/:collection/2fa/:method", func(c echo.Context) error {
		return handleGetMethodAsign(c, app)
	})
}

func handlePostMethodAsign(c echo.Context, app *pocketbase.PocketBase) error {
	// Get current user from an auth record
	switch c.PathParam("method") {
	case "toggle":
		return Toggle2FA(app, c)
	case "setup-verify":
		return VerifySetup2FA(app, c)
	}
	return apis.NewNotFoundError("Method not found", nil)
}

func handleGetMethodAsign(c echo.Context, app *pocketbase.PocketBase) error {
	// Get current user from an auth record
	switch c.PathParam("method") {
	case "state":
		return Query2FAState(app, c)
	}
	return apis.NewNotFoundError("Method not found", nil)
}

func VerifySetup2FA(app *pocketbase.PocketBase, c echo.Context) error {
	record, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)
	if record == nil || !record.Collection().IsAuth() {
		return apis.NewBadRequestError("You must be signed in to enable 2FA", nil)
	}

	code := c.FormValue("code")

	if len(code) != 6 {
		return apis.NewBadRequestError("Code invalid", nil)
	}

	identifier := security.SHA256(record.Id + record.Email() + record.Collection().Name)

	resData := make(map[string]interface{})

	if CheckIfOTPUserExists(app, identifier) {
		valid, err := AuthWithOTP(app, identifier, code)
		if err != nil || !valid {
			return apis.NewBadRequestError("Code invalid", nil)
		}
		resData["message"] = "Code verified"
		resData["code"] = 200
	} else {
		return apis.NewBadRequestError("2FA is not enabled", nil)
	}

	return c.JSON(200, resData)
}

func Toggle2FA(app *pocketbase.PocketBase, c echo.Context) error {
	record, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)
	if record == nil || !record.Collection().IsAuth() {
		return apis.NewBadRequestError("You must be signed in to enable 2FA", nil)
	}

	identifier := security.SHA256(record.Id + record.Email() + record.Collection().Name)

	resData := make(map[string]interface{})

	if CheckIfOTPUserExists(app, identifier) {
		err := RemoveOTPRecord(app, identifier)
		if err != nil {
			return apis.NewApiError(500, err.Error(), nil)
		}
		resData["message"] = "2FA disabled"
		resData["code"] = 200
		resData["state"] = false
	} else {
		url, err := CreateNewOTPRecord(app, identifier)
		if err != nil {
			return apis.NewApiError(500, err.Error(), nil)
		}
		resData["message"] = "2FA enabled"
		resData["code"] = 200
		resData["url"] = url
		resData["state"] = true
	}

	return c.JSON(200, resData)
}

func Query2FAState(app *pocketbase.PocketBase, c echo.Context) error {
	record, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)
	if record == nil || !record.Collection().IsAuth() {
		return apis.NewBadRequestError("You must be signed in to continue", nil)
	}

	identifier := security.SHA256(record.Id + record.Email() + record.Collection().Name)

	resData := make(map[string]interface{})

	if CheckIfOTPUserExists(app, identifier) {
		resData["message"] = "2FA enabled"
		resData["code"] = 200
		resData["state"] = true
	} else {
		resData["message"] = "2FA not enabled"
		resData["code"] = 200
		resData["state"] = false
	}

	return c.JSON(200, resData)
}
