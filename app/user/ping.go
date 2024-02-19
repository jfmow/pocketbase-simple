package user

import (
	"time"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
)

func RegisterPingRoutes(e *core.ServeEvent, app *pocketbase.PocketBase) error {
	e.Router.GET("/ping", func(c echo.Context) error {
		//Get current user from a auth record
		authRecord, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)
		if authRecord == nil {
			return apis.NewForbiddenError("Only auth records can access this endpoint", nil)
		}

		if authRecord.Collection().Name != "users" {
			return nil
		}

		record, _ := app.Dao().FindRecordById(authRecord.Collection().Name, authRecord.Id)
		record.Set("last_active", time.Now().UTC())

		if err := app.Dao().SaveRecord(record); err != nil {
			return err
		}

		return c.String(200, "Received ping :)")
	})
	return nil
}
