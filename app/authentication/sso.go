package authentication

import (
	"net/mail"
	"os"
	"path/filepath"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/tools/security"
	"suddsy.dev/m/v2/emails"
	"suddsy.dev/m/v2/tools"
)

// TODO:
/*
* Delete unused sso tokens after they expire, (cron?)
 */

func RegisterSSORoutes(e *core.ServeEvent, app *pocketbase.PocketBase) {
	e.Router.POST("/api/collections/:collection/auth-with-sso/:method", func(c echo.Context) error {
		return handleMethodAsign(c, app)
	})
}

func handleMethodAsign(c echo.Context, app *pocketbase.PocketBase) error {
	// Get current user from an auth record
	switch c.PathParam("method") {
	case "login":
		return login(c, app)
	case "signup":
		return signup(c, app)
	case "code":
		return code(c, app)
		// DEPRECATED:
		//case "toggle":
		//	return toggle(c, app)
	}
	return apis.NewNotFoundError("Method not found", nil)
}

func signup(c echo.Context, app *pocketbase.PocketBase) error {
	collection, err := app.Dao().FindCollectionByNameOrId(c.PathParam("collection"))
	if err != nil {
		return err
	}
	if collection.Type != "auth" {
		return apis.NewNotFoundError("Auth collection not found", nil)
	}

	username := c.FormValue("username")
	email := c.FormValue("email")

	if email == "" || username == "" {
		return apis.NewBadRequestError("Missing required values", nil)
	}

	// Check if email or username already exists
	record, _ := app.Dao().FindFirstRecordByData(collection.Id, "email", email)
	if record != nil {
		return apis.NewBadRequestError("A user already has that email", nil)
	}

	record, _ = app.Dao().FindFirstRecordByData(collection.Id, "username", username)
	if record != nil {
		return apis.NewBadRequestError("A user already has that username", nil)
	}

	// If not continue with signup

	newUserRecord := models.NewRecord(collection)

	newUserRecord.Set("username", username)
	newUserRecord.Set("email", email)

	// Set the users password
	randomPassword := security.RandomString(38)
	randomTokenKey := security.RandomString(24)
	newUserRecord.SetPassword(randomPassword)
	if !newUserRecord.ValidatePassword(randomPassword) {
		return apis.NewApiError(500, "Failed to validate p", nil)
	}

	newUserRecord.Set("tokenKey", randomTokenKey)

	c.Set("sso", true)

	canAccess, err := app.Dao().CanAccessRecord(newUserRecord, apis.RequestInfo(c), newUserRecord.Collection().CreateRule)
	if !canAccess {
		return apis.NewForbiddenError("", err)
	}

	if err := app.Dao().SaveRecord(newUserRecord); err != nil {
		return err
	}
	// Create a new instance of RecordCreateEvent
	event := &core.RecordCreateEvent{
		BaseCollectionEvent: core.BaseCollectionEvent{Collection: collection}, // Initialize the embedded struct if any
		HttpContext:         c,                                                // Assign your echo.Context
		Record:              newUserRecord,                                    // Assign your models.Record
		UploadedFiles:       nil,                                              // Assign your map[string][]*filesystem.File
	}

	app.OnRecordAfterCreateRequest(collection.Name).Trigger(event)

	return apis.RecordAuthResponse(app, c, newUserRecord, nil)
}

func login(c echo.Context, app *pocketbase.PocketBase) error {
	collection, err := app.Dao().FindCollectionByNameOrId(c.PathParam("collection"))
	if err != nil {
		return err
	}
	if collection.Type != "auth" {
		return apis.NewNotFoundError("Auth collection not found", nil)
	}

	userEmail := c.FormValue("email")
	token := c.FormValue("token")

	if userEmail == "" || token == "" {
		return apis.NewBadRequestError("Missing required values", nil)
	}

	userRecord, err := app.Dao().FindAuthRecordByEmail(collection.Id, userEmail)
	if err != nil {
		return apis.NewApiError(500, "Failed to find account", nil)
	}

	userFlags, err := app.Dao().FindFirstRecordByFilter(
		"user_flags", "user = {:userId} && collection = {:collectionId}",
		dbx.Params{"userId": userRecord.Id, "collectionId": collection.Id},
	)
	if err != nil || !userFlags.GetBool("sso") {
		return apis.NewUnauthorizedError("Method not allowed", nil)
	}

	tokenRecord, err := app.Dao().FindFirstRecordByData("sso_tokens", "user", userRecord.Id)
	if err != nil {
		return apis.NewUnauthorizedError("Invalid details", nil)
	}

	if tokenRecord.Get("token") != token {
		return apis.NewUnauthorizedError("Invalid token", nil)
	}

	if err := app.Dao().DeleteRecord(tokenRecord); err != nil {
		return err
	}

	return apis.RecordAuthResponse(app, c, userRecord, nil)
}

func code(c echo.Context, app *pocketbase.PocketBase) error {
	collection, err := app.Dao().FindCollectionByNameOrId(c.PathParam("collection"))
	if err != nil {
		return err
	}
	if collection.Type != "auth" {
		return apis.NewNotFoundError("Auth collection not found", nil)
	}

	email := c.FormValue("email")

	if email == "" {
		return apis.NewBadRequestError("Missing required values", nil)
	}

	userRecord, err := app.Dao().FindAuthRecordByEmail(collection.Id, email)
	if err != nil || userRecord == nil {
		return apis.NewForbiddenError("Not account found with that email", nil)
	}

	userFlagsRecord, err := app.Dao().FindFirstRecordByFilter(
		"user_flags", "user = {:userId} && collection = {:collectionId}",
		dbx.Params{"userId": userRecord.Id, "collectionId": collection.Id},
	)
	if err != nil || !userFlagsRecord.GetBool("sso") {
		return apis.NewForbiddenError("Method not allowed", nil)
	}

	newLoginToken := security.RandomString(32)
	appUrl, found := os.LookupEnv("WEBSITE_URL")

	if !found {
		return apis.NewApiError(500, "Missing website url env", nil)
	}

	wd := tools.GetWorkDir()
	if err != nil {
		return apis.NewApiError(500, "Unable to process login", nil)
	}

	data := make(map[string]interface{})

	// Assign values to the map
	data["LinkUrl"] = appUrl + "/auth/login?token=" + newLoginToken + "&user=" + userRecord.Email()
	data["Token"] = newLoginToken
	data["HomePage"] = appUrl

	err, email = emails.LoadHtmlFile(filepath.Join(wd, "assets", "emails", "tokenEmail.html"), data)
	if err != nil {
		return apis.NewApiError(500, "Unable to process login email", nil)
	}

	ssoCollection, err := app.Dao().FindCollectionByNameOrId("sso_tokens")
	if err != nil {
		return apis.NewApiError(500, "Missing setup values", nil)
	}

	oldRecord, err := app.Dao().FindFirstRecordByData("sso_tokens", "user", userRecord.Id)
	if err == nil || oldRecord != nil {
		if time.Now().After(oldRecord.GetDateTime("expires").Time()) {
			if err := app.Dao().DeleteRecord(oldRecord); err != nil {
				return err
			}
		} else {
			return apis.NewBadRequestError("You must wait 5 minutes before requesting a new token", nil)
		}
	}

	record := models.NewRecord(ssoCollection)

	record.Set("user", userRecord.Id)
	record.Set("token", newLoginToken)
	record.Set("expires", time.Now().Add(5*time.Minute))

	if err := app.Dao().SaveRecord(record); err != nil {
		return apis.NewApiError(500, "Failed to create a token", nil)
	}

	// Make the email non-blocking in a go func
	go emails.SendCustomEmail("SSO Login Token", []mail.Address{
		{Name: userRecord.Username(), Address: userRecord.Email()},
	}, email, app)

	return nil
}

// DEPRECATED!
func toggle(c echo.Context, app *pocketbase.PocketBase) error {
	// Disable sso for the user (if enabled), and give them a password to use (if they don't use oAuth2)

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

	if authRecord.Collection().Id != collection.Id {
		return apis.NewUnauthorizedError("Collection mis-match", nil)
	}

	// Get the users flags

	userFlagsRecord, err := app.Dao().FindFirstRecordByFilter(
		"user_flags", "user = {:userId} && collection = {:collectionId}",
		dbx.Params{"userId": authRecord.Id, "collectionId": collection.Id},
	)
	if err != nil {
		return apis.NewApiError(500, "Unable to find relation records", nil)
	}

	if userFlagsRecord.GetBool("sso") {
		return disable(c, app, userFlagsRecord, authRecord)
	} else {
		// Enable sso, (remove password)
		return enable(c, app, userFlagsRecord, authRecord)
	}
}

// DEPRECATED!
func enable(c echo.Context, app *pocketbase.PocketBase, userFlags *models.Record, authRecord *models.Record) error {

	userFlags.Set("sso", true)

	randomPassword := security.RandomString(21)
	authRecord.SetPassword(randomPassword)
	if !authRecord.ValidatePassword(randomPassword) {
		return apis.NewApiError(500, "Failed to validate p", nil)
	}
	authRecord.Set("tokenKey", security.RandomString(32))

	// Save the updated Records
	if err := app.Dao().SaveRecord(authRecord); err != nil {
		return err
	}
	if err := app.Dao().SaveRecord(userFlags); err != nil {
		return err
	}

	return nil
}

// DEPRECATED!
func disable(c echo.Context, app *pocketbase.PocketBase, userFlags *models.Record, authRecord *models.Record) error {

	userFlags.Set("sso", false)

	data := make(map[string]interface{})

	// Check to see if they use oAuth2 Provider:

	_, err := app.Dao().FindFirstRecordByFilter(
		"_externalAuths", "recordId = {:userId} && collectionId = {:collection}",
		dbx.Params{"userId": authRecord.Id, "collection": authRecord.Collection().Id},
	)

	if err != nil {
		//Create a password
		randomPassword := security.RandomString(12)
		authRecord.SetPassword(randomPassword)
		if !authRecord.ValidatePassword(randomPassword) {
			return apis.NewApiError(500, "Failed to validate new password", nil)
		}
		authRecord.Set("tokenKey", security.RandomString(32))
		data["password"] = randomPassword
	}

	// Save the updated Records
	if err := app.Dao().SaveRecord(authRecord); err != nil {
		return err
	}
	if err := app.Dao().SaveRecord(userFlags); err != nil {
		return err
	}

	return c.JSON(200, data)
}

func EnableFromOAuthUnlink(app *pocketbase.PocketBase, e *core.RecordUnlinkExternalAuthEvent) error {
	authRecord := e.Record
	collection := e.Record.Collection()

	userFlagsRecord, err := app.Dao().FindFirstRecordByFilter(
		"user_flags", "user = {:userId} && collection = {:collectionId}",
		dbx.Params{"userId": authRecord.Id, "collectionId": collection.Id},
	)
	if err != nil {
		return apis.NewApiError(500, "Unable to find relation records", nil)
	}

	userFlagsRecord.Set("sso", true)

	randomPassword := security.RandomString(21)
	authRecord.SetPassword(randomPassword)
	if !authRecord.ValidatePassword(randomPassword) {
		return apis.NewApiError(500, "Failed to validate p", nil)
	}
	authRecord.Set("tokenKey", security.RandomString(32))

	// Save the updated Records
	if err := app.Dao().SaveRecord(authRecord); err != nil {
		return err
	}
	if err := app.Dao().SaveRecord(userFlagsRecord); err != nil {
		return err
	}

	return nil
}
