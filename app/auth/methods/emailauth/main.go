package emailauth

import (
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func RegisterEmailAuthRoutes(e *core.ServeEvent, app *pocketbase.PocketBase) {
	e.Router.POST("/api/collections/:collection/auth-with-sso/:method", func(c echo.Context) error {
		return handleMethodAsign(c, app)
	})
}

func handleMethodAsign(c echo.Context, app *pocketbase.PocketBase) error {
	// Get current user from an auth record
	switch c.PathParam("method") {
	case "startsignup":
		return startSignup(app, c)
	case "finishsignup":
		return finishSignup(app, c)
	case "startlogin":
		return startLogin(app, c)
	case "finishlogin":
		return finishLogin(app, c)
	}
	return apis.NewNotFoundError("Method not found", nil)
}
