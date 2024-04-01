package authentication

import (
	"net/mail"
	"regexp"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/tools/security"
	"suddsy.dev/m/v2/emails"
)

/*
	User email & other things from c and provided into these function because they don't know if formdata or pathparams are used
	and i'm indecisive so i'll just pass them in for simplicty :)
*/

var (
	tokenRegex           = "^[A-Za-z0-9]{20}$"
	tokenLength          = 20
	tokenExpiryTime      = time.Now().Add(5 * time.Minute).UTC()
	randomPasswordLength = 38
)

var (
	genericEmailAuthServerError           = apis.NewApiError(500, "An error occured processing your request", nil)
	genericInvalidRequestDataError        = apis.NewBadRequestError("Invalid request data", nil)
	missingRequestDataError               = apis.NewApiError(500, "Missing setup values", nil)
	serverMissingEnviromentVariablesError = apis.NewApiError(500, "A server error occured processing your request", nil)
	accountAlreadyExistsError             = apis.NewBadRequestError("An account already exists with that username/email", nil)
	authMethodNotSupportedError           = apis.NewBadRequestError("Authentication method not supported", nil)
	tokenRateLimitError                   = apis.NewBadRequestError("You must wait 5 minutes before requesting a new token", nil)
)

/*
Creates a new account.

sso is enabled in the flags in the after create event which is triggred by this function
*/

func logDescriptiveErrorToLogs(app *pocketbase.PocketBase, errorMessage string, fullError any) {
	app.Logger().Error(errorMessage, "details", fullError)
}

func createNewUser(c echo.Context, app *pocketbase.PocketBase, authCollection *models.Collection, userName string, userEmail string) (error, *models.Record) {
	record, _ := app.Dao().FindFirstRecordByFilter(
		authCollection.Id, "email = {:email} || username = {:username}",
		dbx.Params{"email": userEmail, "username": userName},
	)
	if record != nil {
		return accountAlreadyExistsError, nil
	}

	newUserRecord := models.NewRecord(authCollection)

	newUserRecord.Set("username", userName)
	newUserRecord.Set("email", userEmail)

	// Set the users password
	randomPassword := security.RandomString(randomPasswordLength)
	randomTokenKey := security.RandomString(randomPasswordLength)
	newUserRecord.SetPassword(randomPassword)
	if !newUserRecord.ValidatePassword(randomPassword) {
		logDescriptiveErrorToLogs(app, "Failed to validate the random password when creating a new user", nil)
		return genericEmailAuthServerError, nil
	}

	newUserRecord.Set("tokenKey", randomTokenKey)

	c.Set("sso", true)

	canAccess, err := app.Dao().CanAccessRecord(newUserRecord, apis.RequestInfo(c), newUserRecord.Collection().CreateRule)
	if !canAccess || err != nil {
		logDescriptiveErrorToLogs(app, "Create rule not allowing account creation for request", authCollection.Name)
		return genericEmailAuthServerError, nil
	}

	if err := app.Dao().SaveRecord(newUserRecord); err != nil {
		return err, nil
	}
	// Create a new instance of RecordCreateEvent
	event := &core.RecordCreateEvent{
		BaseCollectionEvent: core.BaseCollectionEvent{Collection: authCollection}, // Initialize the embedded struct if any
		HttpContext:         c,                                                    // Assign your echo.Context
		Record:              newUserRecord,                                        // Assign your models.Record
		UploadedFiles:       nil,                                                  // Assign your map[string][]*filesystem.File
	}

	app.OnRecordAfterCreateRequest(authCollection.Name).Trigger(event)

	return nil, newUserRecord
}

/*
Gets and returns a user
*/
func getUserRecord(app *pocketbase.PocketBase, authCollection *models.Collection, userEmail string, emailAuthIsEnabledCheck bool) (error, *models.Record) {
	userRecord, err := app.Dao().FindAuthRecordByEmail(authCollection.Id, userEmail)
	if err != nil {
		return err, nil
	}

	err, usersFlags := getUsersFlags(app, userRecord)
	if err != nil {
		// The provided error should do fine
		return err, nil
	}

	if emailAuthIsEnabledCheck {
		if !usersFlags.GetBool("sso") {
			return authMethodNotSupportedError, nil
		}
	}

	return nil, userRecord
}

/*
Creates a new sso token

	They are stored with the following data:
	- User -> the email provided (not the id)
	- Collection -> the collection they are trying to auth with
	- Expires -> the time the token should be considred invalid
	- Token -> the token itself

- Errors with apis. for simplicty
*/
func createEmailAuthToken(app *pocketbase.PocketBase, authCollection *models.Collection, userEmail string, tokenMethod string) (error, *models.Record) {
	randomToken := security.RandomString(tokenLength) /* Doesn't have to be super long as there is no threat of it being guessed that quick as it is random*/
	collectionId := authCollection.Id                 /* This param will be able to be compared aganist when trying to reterive the token*/
	tokenExpireyDate := tokenExpiryTime               /* Store the time in utc as thats what pocketbase itself wants*/

	/* Get the collection where the tokens are stored*/

	tokenCollection, err := app.Dao().FindCollectionByNameOrId("sso_tokens")
	if err != nil {
		logDescriptiveErrorToLogs(app, "Unable to find the sso_tokens collection", nil)
		return genericEmailAuthServerError, nil
	}

	/* Check the user has no old or pending tokens */

	oldTokenRecord, err := app.Dao().FindFirstRecordByFilter(
		"sso_tokens", "user = {:email} && collection = {:collection}",
		dbx.Params{"email": userEmail, "collection": authCollection.Id},
	)
	if err == nil || oldTokenRecord != nil {
		if time.Now().After(oldTokenRecord.GetDateTime("expires").Time()) {
			if err := app.Dao().DeleteRecord(oldTokenRecord); err != nil {
				logDescriptiveErrorToLogs(app, "Unable to delete a users sso token record", err)
				return err, nil
			}
		} else {
			return tokenRateLimitError, nil
		}
	}

	/* Create the new record */

	newTokenRecord := models.NewRecord(tokenCollection)

	newTokenRecord.Set("user", userEmail)
	newTokenRecord.Set("token", randomToken)
	newTokenRecord.Set("collection", collectionId)
	newTokenRecord.Set("expires", tokenExpireyDate)
	newTokenRecord.Set("method", tokenMethod)

	if err := app.Dao().SaveRecord(newTokenRecord); err != nil {
		logDescriptiveErrorToLogs(app, "Unable to create a users sso token record", err)
		return genericEmailAuthServerError, nil
	}

	return nil, newTokenRecord
}

/*
Verifys a provided sso token is valid and the user requested is the one for that token

- provides a boolen value for extra clarity

- Errors with apis. for simplicty
*/
func authenticateEmailAuthToken(app *pocketbase.PocketBase, authCollection *models.Collection, userEmail string, token string, tokenMethod string) error {
	if userEmail == "" || !isValidEmail(userEmail) {
		return genericInvalidRequestDataError
	}
	regex := regexp.MustCompile(tokenRegex)

	// Match the text against the pattern
	if !regex.MatchString(token) {
		return apis.NewBadRequestError("Invalid token", nil)
	}

	userTokenRecord, err := app.Dao().FindFirstRecordByFilter(
		"sso_tokens", "user = {:email} && collection = {:collection} && token = {:token} && method = {:method}",
		dbx.Params{"email": userEmail, "collection": authCollection.Id, "token": token, "method": tokenMethod},
	)
	if err != nil || userTokenRecord == nil {
		// The record was not found or another error
		return apis.NewBadRequestError("Invalid token", nil)
	}

	if time.Now().UTC().After(userTokenRecord.GetDateTime("expires").Time()) {
		// The token was expired
		return apis.NewBadRequestError("Invalid token", nil)
	}

	// This is in a go func becuase if its not for some reasons this whole code block just breaks. This is just a If it works it works moment
	go func() {
		app.Dao().DeleteRecord(userTokenRecord)
	}()
	/*
		The token is valid

		We doubbly know that the token is valid even if it doesn't look like it because
		- its not expired
		- the db querry is matching the user, collection and token
			- meaning if one is wrong nothing will even show up
	*/
	return nil
}

/*
Finds and returns a provided auth collection users flags

# This is global

not just EmailAuth behavior
*/
func getUsersFlags(app *pocketbase.PocketBase, userRecord *models.Record) (error, *models.Record) {
	userFlags, err := app.Dao().FindFirstRecordByFilter(
		"user_flags", "user = {:userId} && collection = {:collectionId}",
		dbx.Params{"userId": userRecord.Id, "collectionId": userRecord.Collection().Id},
	)
	if err != nil {
		return err, nil
	}
	return nil, userFlags
}

/*
Params

  - token := emailData["token"]
  - message := emailData["message"]
  - footer := emailData["footer"]
  - subject := emailData["subject"]
  - previewText := emailData["previewText"]
  - appName := emailData["appName"]
  - recp := emailData["recp"]
  - replyTo := emailData["replyTo"]
  - buttonLink := emailData["buttonLink"]
  - buttonText := emailData["buttonText"]
  - logoURL := emailData["logoURL"]
  - recpName := emailData["recpName"]
*/
func sendEmailWithToken(app *pocketbase.PocketBase, emailData map[string]interface{}) error {
	subject := emailData["subject"].(string)
	recp := emailData["recp"].(string)
	recpName := emailData["recpName"].(string)

	email, err := emails.LoadEmailDataToHTML(app, "emailAuth", emailData)
	if err != nil {
		logDescriptiveErrorToLogs(app, "Failed to write the email data to the html file or load html file", err)
		return genericEmailAuthServerError
	}

	go emails.SendCustomEmail(subject, []mail.Address{
		{Name: recpName, Address: recp},
	}, email, app)

	return nil
}

/*
Check if an email is a valid email
*/
func isValidEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

type userHttpRequestType struct {
	Email    string
	Username string
	Token    string
}

/*
Get the basic request data.
Makes it easy if I want to change how data is read in future
*/
func getUserRequestData(c echo.Context, app *pocketbase.PocketBase, authCollection *models.Collection, tokenMethod string, fieldsToCheck ...string) (*userHttpRequestType, error) {
	var (
		email, username, token string
		err                    error
	)

	for _, field := range fieldsToCheck {
		switch field {
		case "email":
			email = c.FormValue("email")
			if email == "" || !isValidEmail(email) {
				return nil, apis.NewBadRequestError("Email in invalid format", nil)
			}
		case "username":
			username = c.FormValue("username")
			if len(username) < 3 || len(username) > 20 {
				return nil, apis.NewBadRequestError("Username must be less than 20 but more than 3 characters long", nil)
			}
		case "token":
			token = c.FormValue("token")
			if token != "" {
				err = authenticateEmailAuthToken(app, authCollection, email, token, tokenMethod)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	userRequestData := &userHttpRequestType{
		Email:    email,
		Username: username,
		Token:    token,
	}
	return userRequestData, nil
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
