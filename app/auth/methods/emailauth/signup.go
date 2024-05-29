package emailauth

import (
	"net/url"
	"os"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/tools/security"
	"suddsy.dev/m/v2/app/auth/tokens"
)

func startSignup(app *pocketbase.PocketBase, c echo.Context) error {

	email := c.FormValue("email")
	collectionIdOrName := c.PathParam("collection")

	if !isValidEmail(email) {
		return apis.NewBadRequestError("Invalid or missing email", nil)
	}

	collection, err := app.Dao().FindCollectionByNameOrId(collectionIdOrName)
	if err != nil {
		return apis.NewApiError(500, "Invalid auth collection", nil)
	}

	existantRecord, err := getUserRecord(app, collection, email)
	if err == nil || existantRecord != nil {
		canView, err := app.Dao().CanAccessRecord(existantRecord, apis.RequestInfo(c), existantRecord.Collection().ViewRule)
		if !canView {
			return apis.NewForbiddenError("", err)
		}
		return apis.NewApiError(500, "A user with that email already exists", nil)
	}

	canCreate, err := app.Dao().CanAccessRecord(nil, apis.RequestInfo(c), collection.CreateRule)
	if !canCreate {
		return apis.NewForbiddenError("", err)
	}

	token, err := tokens.Initialise(email, collection, false).CreateNewToken("emailauthsignup", app)
	if err != nil {
		return apis.NewApiError(500, "Problem occured creating a temp auth token", nil)
	}

	emailData := make(map[string]interface{})

	replyToAddress, found := os.LookupEnv("email_reply_to")
	if !found {
		app.Logger().Error("No reply to email env found")
		return apis.NewApiError(500, "Internal server error", nil)
	}
	appURLEnv, found := os.LookupEnv("website_url")
	if !found {
		app.Logger().Error("No website url env found")
		return apis.NewApiError(500, "Internal server error", nil)
	}
	// Remove the urls trailing /
	appURLEnv = strings.TrimSuffix(appURLEnv, "/")

	parsedURL, err := url.Parse(appURLEnv)
	if err != nil {
		app.Logger().Error("Error parsing app url env")
		return apis.NewApiError(500, "Internal server error", nil)
	}

	// Check if the URL is valid
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		app.Logger().Error("App url env invalid type. Not in url format")
		return apis.NewApiError(500, "Internal server error", nil)
	}

	emailData["token"] = token.Value
	emailData["subject"] = "Signup confirmation"
	emailData["recp"] = email
	emailData["replyTo"] = replyToAddress
	emailData["buttonLink"] = appURLEnv + "/auth/signup/confirm?token=" + token.Value + "&email=" + email
	emailData["recpName"] = ""

	//Save the token to the db
	_, err = token.Save()
	if err != nil {
		return apis.NewApiError(500, "An error occured while trying to save", nil)
	}

	go sendEmailWithToken(app, emailData)

	return c.String(200, "Token email sent to: "+email)
}

func finishSignup(app *pocketbase.PocketBase, c echo.Context) error {
	email := c.FormValue("email")
	username := c.FormValue("username")
	formToken := c.FormValue("token")
	collectionIdOrName := c.PathParam("collection")

	collection, err := app.Dao().FindCollectionByNameOrId(collectionIdOrName)
	if err != nil {
		return apis.NewApiError(500, "Invalid auth collection", nil)
	}

	if !isValidEmail(email) || len(username) < 3 {
		return apis.NewBadRequestError("Invalid or missing data", nil)
	}

	token := tokens.Initialise(email, collection, false).RebuildToken(formToken, "emailauthsignup")

	if err := token.Verify(app); err != nil {
		return apis.NewUnauthorizedError(err.Error(), nil)
	}

	userRecord, err := app.Dao().FindFirstRecordByFilter(
		collection.Id, "email = {:email} || username = {:username}",
		dbx.Params{"email": email, "username": username},
	)
	if userRecord != nil || err == nil {
		//Resave the token
		return apis.NewBadRequestError("A user with that email/username has already been registered.", nil)
	} else {
		_ = token.RemoveToken(app)
	}

	//Create the new user

	newUserRecord := models.NewRecord(collection)

	newUserRecord.Set("username", username)
	newUserRecord.Set("email", email)

	// Set the users password
	randomPassword := security.RandomString(33)
	randomTokenKey := security.RandomString(32)
	newUserRecord.SetPassword(randomPassword)
	if !newUserRecord.ValidatePassword(randomPassword) {
		logDescriptiveErrorToLogs(app, "Failed to validate the random password when creating a new user", nil)
		return apis.NewApiError(500, "A problem occured while creating your account.", nil)
	}

	newUserRecord.Set("tokenKey", randomTokenKey)

	canAccess, err := app.Dao().CanAccessRecord(newUserRecord, apis.RequestInfo(c), newUserRecord.Collection().CreateRule)
	if !canAccess || err != nil {
		logDescriptiveErrorToLogs(app, "Create rule not allowing account creation for request", collection.Name)
		return apis.NewApiError(500, "A problem occured while creating your account.", nil)
	}

	if err := app.Dao().SaveRecord(newUserRecord); err != nil {
		return apis.NewApiError(500, "A problem occured while creating your account.", nil)
	}
	// Create a new instance of RecordCreateEvent
	event := &core.RecordCreateEvent{
		BaseCollectionEvent: core.BaseCollectionEvent{Collection: collection}, // Initialize the embedded struct if any
		HttpContext:         c,                                                // Assign your echo.Context
		Record:              newUserRecord,                                    // Assign your models.Record
		UploadedFiles:       nil,                                              // Assign your map[string][]*filesystem.File
	}

	err = app.OnRecordAfterCreateRequest(collection.Id).Trigger(event)
	if err != nil {
		return apis.NewApiError(500, "A problem occured while creating your account.", nil)
	}

	return apis.RecordAuthResponse(app, c, newUserRecord, nil)
}
