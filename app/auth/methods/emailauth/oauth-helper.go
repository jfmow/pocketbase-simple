package emailauth

import (
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/security"
)

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
