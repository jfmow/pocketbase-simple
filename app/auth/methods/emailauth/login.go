package emailauth

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	twofa "suddsy.dev/m/v2/app/auth/TwoFA"
	"suddsy.dev/m/v2/app/auth/tokens"
)

func startLogin(app *pocketbase.PocketBase, c echo.Context) error {
	email := c.FormValue("email")
	collectionIdOrName := c.PathParam("collection")

	if !isValidEmail(email) {
		return apis.NewBadRequestError("Invalid or missing email", nil)
	}

	if !isValidEmail(email) {
		return apis.NewBadRequestError("Invalid or missing email", nil)
	}

	collection, err := app.Dao().FindCollectionByNameOrId(collectionIdOrName)
	if err != nil {
		return apis.NewApiError(500, "Invalid auth collection", nil)
	}

	userRecord, err := getUserRecord(app, collection, email)
	if err != nil || userRecord == nil {
		apis.NewBadRequestError("No user found", nil)
	}

	/*canView, err := app.Dao().CanAccessRecord(userRecord, apis.RequestInfo(c), collection.ViewRule)
	if !canView {
		return apis.NewForbiddenError("", err)
	}*/

	token, err := tokens.Initialise(email, collection, true).CreateNewToken("emailauthlogin", app)
	if err != nil {
		return apis.NewApiError(500, "Problem occured creating a temp auth token", nil)
	}

	fmt.Println(token.Value)

	if token.CheckExistingToken() {
		return apis.NewApiError(500, "A token already exists that hasn't expired", nil)
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
	emailData["subject"] = "Login token"
	emailData["recp"] = email
	emailData["replyTo"] = replyToAddress

	emailData["recpName"] = ""

	//Save the token to the db
	_, err = token.Save()
	if err != nil {
		return apis.NewApiError(500, "An error occured while trying to save", nil)
	}

	resData := make(map[string]interface{})
	resData["message"] = "Token email sent to: " + email
	resData["code"] = 200

	otp, err := twofa.Load(app, userRecord)
	if err != nil || otp == nil || !otp.IsEnabled() {
		emailData["buttonLink"] = appURLEnv + "/auth/login?token=" + token.Value + "&email=" + email
	} else {
		resData["2fa"] = "required"
		emailData["buttonLink"] = appURLEnv + "/auth/login?token=" + token.Value + "&email=" + email + "&2fa=1"
	}

	err = sendEmailWithToken(app, emailData)
	if err != nil {
		return apis.NewApiError(500, "Problem sending email", nil)
	}

	return c.JSON(200, resData)

}

func finishLogin(app *pocketbase.PocketBase, c echo.Context) error {
	email := c.FormValue("email")
	formToken := c.FormValue("token")
	collectionIdOrName := c.PathParam("collection")

	collection, err := app.Dao().FindCollectionByNameOrId(collectionIdOrName)
	if err != nil {
		return apis.NewApiError(500, "Invalid auth collection", nil)
	}

	if !isValidEmail(email) {
		return apis.NewBadRequestError("Invalid or missing email", nil)
	}

	token := tokens.Initialise(email, collection, false).RebuildToken(formToken, "emailauthlogin")

	if err := token.Verify(app); err != nil {
		return apis.NewUnauthorizedError(err.Error(), nil)
	}

	userRecord, err := getUserRecord(app, collection, email)
	if err != nil || userRecord == nil {
		apis.NewBadRequestError("No user found", nil)
	}

	otp, err := twofa.Load(app, userRecord)
	if err == nil && otp != nil {
		err := otp.AuthWith(c.FormValue("2fa"))
		if err != nil {
			return apis.NewUnauthorizedError("Invalid 2fa code", nil)
		}
	}
	//End 2FA

	_ = token.RemoveToken(app)

	return apis.RecordAuthResponse(app, c, userRecord, nil)

}
