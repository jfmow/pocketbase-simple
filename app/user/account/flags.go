package account

import (
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
)

func HandleRegisterRoutes(e *core.ServeEvent, app *pocketbase.PocketBase) {
	e.Router.POST("/api/collections/:collection/flags/update", func(c echo.Context) error {
		return updateFlagsDynamic(c, app)
	})
}

func updateFlagsDynamic(c echo.Context, app *pocketbase.PocketBase) error {
	authRecord, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)
	if authRecord == nil || authRecord.Collection().Name != "admins" {
		return apis.NewUnauthorizedError("You must be signed in to access this", nil)
	}

	collection, err := app.Dao().FindCollectionByNameOrId(c.PathParam("collection"))
	if err != nil {
		return err
	}
	if collection.Type != "auth" {
		return apis.NewNotFoundError("Auth collection not found", nil)
	}

	values, err := c.FormValues()
	if err != nil {
		// Handle the error
		return err
	}

	record, err := app.Dao().FindFirstRecordByFilter(
		"user_flags", "user = {:userID} && collection = {:collectionID}",
		dbx.Params{"collectionID": collection.Id, "userID": c.FormValue("user")},
	)
	if err != nil {
		return err
	}

	// Now you can use `values`
	for i := range values {
		// Your code here
		record.Set(i, c.FormValue(i))
	}

	if err := app.Dao().SaveRecord(record); err != nil {
		return err
	}

	return c.JSON(200, record)
}
