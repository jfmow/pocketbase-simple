package account

import (
	"sync"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
)

/*
TODO: Remove, just let the user have no pages
Will be removed in the future
*/
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
		//A page was found so don't make the default page
		data["id"] = pagesSearchRecord.Id

		return c.JSON(302, data)
	}

	//No page was found so make the default preview page
	pageId, err := CreatePreviewPage(app, authRecord.Id)

	if err != nil {
		return err
	}

	data["id"] = pageId

	return c.JSON(302, data)
}
