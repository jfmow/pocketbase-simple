package pages

import (
	"log"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/tools/security"
)

func RegisterAccPagesRoutes(e *core.ServeEvent, app *pocketbase.PocketBase) {
	e.Router.POST("/api/page/duplicate", func(c echo.Context) error {
		return duplicatePage(c, app)
	})
}

func duplicatePage(c echo.Context, app *pocketbase.PocketBase) error {
	//Check the user is signed in
	authRecord, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)
	if authRecord == nil {
		return apis.NewUnauthorizedError("You must be signed in to access this", nil)
	}

	if authRecord.Collection().Type != "auth" {
		return apis.NewNotFoundError("Auth collection not found", nil)
	}

	// Get the requested page id from the request
	requestedPageId := c.FormValue("page")
	if requestedPageId == "" || len(requestedPageId) != 15 {
		return apis.NewBadRequestError("", nil)
	}

	//Get the page that we a copying
	pageToCopyRecord, err := app.Dao().FindRecordById("pages", requestedPageId)
	if err != nil {
		log.Println("A")
		return err
	}

	//Check the user is allowed to access that page
	canAccess, err := app.Dao().CanAccessRecord(pageToCopyRecord, apis.RequestInfo(c), pageToCopyRecord.Collection().ViewRule)
	if !canAccess {
		return apis.NewForbiddenError("", err)
	}

	//Copy the page
	newCopyOfPageRecord := models.NewRecord(pageToCopyRecord.Collection())

	newCopyOfPageRecord.Set("title", pageToCopyRecord.GetString("title")+" "+security.RandomString(4))
	newCopyOfPageRecord.Set("content", pageToCopyRecord.GetString("content"))
	newCopyOfPageRecord.Set("unsplash", pageToCopyRecord.GetString("unsplash"))
	newCopyOfPageRecord.Set("icon", pageToCopyRecord.GetString("icon"))
	newCopyOfPageRecord.Set("parentId", pageToCopyRecord.GetString("parentId"))
	newCopyOfPageRecord.Set("owner", authRecord.Id)

	//Save
	if err := app.Dao().SaveRecord(newCopyOfPageRecord); err != nil {
		return err
	}

	//Return the id of the duplicate page
	return c.String(200, newCopyOfPageRecord.Id)
}
