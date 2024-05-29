package emailauth

import (
	"net/mail"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/models"
	"suddsy.dev/m/v2/emails"
)

func logDescriptiveErrorToLogs(app *pocketbase.PocketBase, errorMessage string, fullError any) {
	app.Logger().Error(errorMessage, "details", fullError)
}

func sendEmailWithToken(app *pocketbase.PocketBase, emailData map[string]interface{}) error {
	subject := emailData["subject"].(string)
	recp := emailData["recp"].(string)
	recpName := emailData["recpName"].(string)

	email, err := emails.LoadEmailDataToHTML(app, "emailAuth", emailData)
	if err != nil {
		logDescriptiveErrorToLogs(app, "Failed to write the email data to the html file or load html file", err)
		return apis.NewApiError(500, "An error occured processing your request", nil)
	}

	go emails.SendCustomEmail(subject, []mail.Address{
		{Name: recpName, Address: recp},
	}, email, app)

	return nil
}

func getUserRecord(app *pocketbase.PocketBase, authCollection *models.Collection, userEmail string) (*models.Record, error) {
	userRecord, err := app.Dao().FindAuthRecordByEmail(authCollection.Id, userEmail)
	if err != nil {
		return nil, err
	}

	return userRecord, err
}

func isValidEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}
