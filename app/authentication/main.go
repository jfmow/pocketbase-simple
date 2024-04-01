package authentication

import (
	"net/url"
	"os"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
)

func RegisterSSORoutes(e *core.ServeEvent, app *pocketbase.PocketBase) {
	e.Router.POST("/api/collections/:collection/auth-with-sso/:method", func(c echo.Context) error {
		return handleMethodAsign(c, app)
	})
}

func handleMethodAsign(c echo.Context, app *pocketbase.PocketBase) error {
	// Get current user from an auth record
	collection, err := app.Dao().FindCollectionByNameOrId(c.PathParam("collection"))
	if err != nil {
		return err
	}
	if collection.Type != "auth" {
		return apis.NewNotFoundError("Requested resouce not found", nil)
	}
	switch c.PathParam("method") {
	case "startsignup":
		return startSignup(c, app, collection)
	case "finishsignup":
		return finishSignup(c, app, collection)
	case "startlogin":
		return startLogin(c, app, collection)
	case "finishlogin":
		return finishLogin(c, app, collection)
	}
	return apis.NewNotFoundError("Method not found", nil)
}

/*
 --Start route functions--
*/

// Signup start

func startSignup(c echo.Context, app *pocketbase.PocketBase, authCollection *models.Collection) error {
	requestData, err := getUserRequestData(c, app, authCollection, "signup", "email")
	if err != nil {
		return err
	}
	email := requestData.Email

	_, record := getUserRecord(app, authCollection, email, false) // false because we're checking for in general if someone already has that info
	if record != nil {
		return apis.NewBadRequestError("An account with that email already exists", nil)
	}

	err, tokenRecord := createEmailAuthToken(app, authCollection, email, "signup")
	if err != nil {
		return err
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

	emailData["token"] = tokenRecord.Get("token").(string)
	emailData["message"] = "Copy the code below or use the button to confirm your email to continue the signup process."
	emailData["footer"] = "If you didn't request to signup you can safely ignore this email as the account will not be created without it."
	emailData["subject"] = "Signup confirmation"
	emailData["previewText"] = "Use the code to confirm your email and continue the signup process"
	emailData["appName"] = "Note"
	emailData["recp"] = email
	emailData["replyTo"] = replyToAddress
	emailData["buttonLink"] = appURLEnv + "/auth/signup/confirm?token=" + tokenRecord.Get("token").(string) + "&email=" + email
	emailData["buttonText"] = "Continue signup"
	emailData["logoURL"] = appURLEnv + "/logo.webp"
	emailData["recpName"] = ""

	err = sendEmailWithToken(app, emailData)
	if err != nil {
		return apis.NewApiError(500, "Problem sending email", nil)
	}

	return nil
}

func finishSignup(c echo.Context, app *pocketbase.PocketBase, authCollection *models.Collection) error {
	requestData, err := getUserRequestData(c, app, authCollection, "signup", "email", "username", "token")
	if err != nil {
		return err
	}
	email := requestData.Email
	username := requestData.Username
	token := requestData.Token

	err = authenticateEmailAuthToken(app, authCollection, email, token, "signup")
	if err != nil {
		return err
	}

	err, userRecord := createNewUser(c, app, authCollection, username, email)
	if err != nil {
		return err
	}

	return apis.RecordAuthResponse(app, c, userRecord, nil)
}

//Signup end

//Login start

func startLogin(c echo.Context, app *pocketbase.PocketBase, authCollection *models.Collection) error {
	requestData, err := getUserRequestData(c, app, authCollection, "login", "email")
	if err != nil {
		return err
	}
	email := requestData.Email

	err, userRecord := getUserRecord(app, authCollection, email, true)
	if err != nil {
		return err
	}
	if userRecord == nil {
		return apis.NewNotFoundError("Requested account not found", nil)
	}

	err, tokenRecord := createEmailAuthToken(app, authCollection, email, "login")
	if err != nil {
		return err
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

	emailData["token"] = tokenRecord.Get("token").(string)
	emailData["message"] = "Copy the code below or use the button to login"
	emailData["footer"] = "If you didn't request this code you can safely ignore it as your account has not been accesed."
	emailData["subject"] = "Login code"
	emailData["previewText"] = "Use this code to login to your Note account"
	emailData["appName"] = "Note"
	emailData["recp"] = email
	emailData["replyTo"] = replyToAddress
	emailData["buttonLink"] = appURLEnv + "/auth/login?token=" + tokenRecord.Get("token").(string) + "&email=" + email
	emailData["buttonText"] = "Login"
	emailData["logoURL"] = appURLEnv + "/logo.webp"
	emailData["recpName"] = userRecord.Username()

	err = sendEmailWithToken(app, emailData)
	if err != nil {
		return apis.NewApiError(500, "Problem sending email", nil)
	}
	return nil
}

func finishLogin(c echo.Context, app *pocketbase.PocketBase, authCollection *models.Collection) error {
	userRequestData, err := getUserRequestData(c, app, authCollection, "login", "email", "token")
	if err != nil {
		return err
	}

	err, userRecord := getUserRecord(app, authCollection, userRequestData.Email, true)
	if err != nil {
		return err
	}
	if userRecord == nil {
		return apis.NewNotFoundError("Requested account not found", nil)
	}

	err = authenticateEmailAuthToken(app, authCollection, userRequestData.Email, userRequestData.Token, "login")
	if err != nil {
		return err
	}

	return apis.RecordAuthResponse(app, c, userRecord, nil)
}

//Login end
