package pages

import (
	"sync"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
	"suddsy.dev/m/v2/app/user/account"
)

func RegisterAccPagesRoutes(e *core.ServeEvent, app *pocketbase.PocketBase) {
	e.Router.GET("/api/collections/:collection/account/create-empty-page", func(c echo.Context) error {
		return createBlankPage(c, app)
	})
}

var authMutex sync.Mutex

func createBlankPage(c echo.Context, app *pocketbase.PocketBase) error {
	authRecord, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)
	if authRecord == nil {
		return apis.NewUnauthorizedError("You must be signed in to access this", nil)
	}

	collection, err := app.Dao().FindCollectionByNameOrId(c.PathParam("collection"))
	if err != nil {
		return err
	}
	if collection.Type != "auth" {
		return apis.NewNotFoundError("Auth collection not found", nil)
	}

	if authRecord.Collection().Id != collection.Id {
		return apis.NewForbiddenError("Not authorized", nil)
	}

	data := make(map[string]interface{})

	// Check the user doesn't really have any pages
	authMutex.Lock()
	defer authMutex.Unlock()
	pagesSearchRecord, err := app.Dao().FindFirstRecordByData("pages", "user", authRecord.Id)

	if err == nil {
		data["id"] = pagesSearchRecord.Id

		return c.JSON(302, data)
	}

	err, pageId := account.CreatePreviewPage(app, authRecord.Id)

	if err != nil {
		return err
	}

	data["id"] = pageId

	return c.JSON(302, data)
}
