package twofa

import (
	"log"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/tools/security"
	"github.com/pquerna/otp/totp"
)

type TwoFAStruct struct {
	identifier string
	record     *TwoFARecordStruct
	app        *pocketbase.PocketBase
}

/*
URL may be ""
*/
type TwoFARecordStruct struct {
	secret  string
	enabled bool
	id      string
	url     string
}

func Load(app *pocketbase.PocketBase, authRecord *models.Record) (*TwoFAStruct, error) {

	if app == nil {
		return nil, NewTwoFAError("No pocketbase app was provided")
	}

	//Check the record is an auth record, for id creation
	if authRecord == nil || !authRecord.Collection().IsAuth() {
		return nil, NewTwoFAError("The provided authRecord is not from an auth collection")
	}

	//Create the unique identifier
	recordIdentifier := create2FAIdentifierFromRecord(authRecord)

	//Get the 2FA secrets record
	record, err := app.Dao().FindFirstRecordByData("2fa_secrets", "unid", recordIdentifier)

	if err != nil || record == nil {
		return nil, NewTwoFAError("The authRecord does not have 2FA enabled")
	}

	var (
		secret  = record.GetString("secret")
		enabled = record.GetBool("enabled")
	)

	if secret == "" {
		return nil, NewTwoFAError("The 2FA record is missing the secret \nIt is recommended to delete it and create a new one")
	}

	return &TwoFAStruct{
		record: &TwoFARecordStruct{
			secret:  secret,
			enabled: enabled,
			id:      record.Id,
		},
		app:        app,
		identifier: recordIdentifier,
	}, nil
}

/*
Creates a new 2FA record in the db

The record is still disabled so cannot be used to login until validated
*/
func Create(app *pocketbase.PocketBase, authRecord *models.Record) (*TwoFAStruct, error) {
	if app == nil {
		return nil, NewTwoFAError("No pocketbase app was provided")
	}

	//Check the record is an auth record, for id creation
	if authRecord == nil || !authRecord.Collection().IsAuth() {
		return nil, NewTwoFAError("The provided authRecord is not from an auth collection")
	}

	//Asign vars
	var (
		recordIdentifer = create2FAIdentifierFromRecord(authRecord)
		appName         = app.Settings().Meta.AppName
	)

	//Generate the 2FA key
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      appName,
		AccountName: recordIdentifer,
	})
	if err != nil {
		log.Println(err)
		return nil, NewTwoFAError("An error occured generating the TOTP key")
	}

	//Create the db record
	collection, err := app.Dao().FindCollectionByNameOrId("2fa_secrets")
	if err != nil {
		return nil, err
	}

	dbrecord := models.NewRecord(collection)

	dbrecord.Set("secret", key.Secret())
	dbrecord.Set("unid", recordIdentifer)
	dbrecord.Set("enabled", false)

	if err := app.Dao().SaveRecord(dbrecord); err != nil {
		log.Println(err)
		return nil, NewTwoFAError("An error occured saving the 2FA record to the db")
	}

	return &TwoFAStruct{
		record: &TwoFARecordStruct{
			secret:  key.Secret(),
			enabled: false,
			id:      dbrecord.Id,
			url:     key.URL(),
		},
		app:        app,
		identifier: recordIdentifer,
	}, nil
}

/*
Enables the 2FA record for auth, if the 2fa code is valid
*/
func (rec *TwoFAStruct) Enable(twoFACode string) error {
	if rec.record.enabled {
		return NewTwoFAError("The 2FA record is already enabled")
	}
	if len(twoFACode) != 6 {
		return NewTwoFAError("Invalid 2FA code")
	}

	valid := validate2FACode(twoFACode, rec.record.secret)
	if valid {
		//Enable the record to be used for login
		record, err := rec.app.Dao().FindRecordById("2fa_secrets", rec.record.id)
		if err != nil || record == nil {
			log.Println(err)
			return NewTwoFAError("An error occurred while getting the 2FA record")
		}
		record.Set("enabled", true)
		if err := rec.app.Dao().SaveRecord(record); err != nil {
			log.Println(err)
			return NewTwoFAError("An error occurred enabling the code")
		}
		return nil
	} else {
		//Don't enable the record
		return NewTwoFAError("Invalid 2FA code")
	}
}

/*
Deletes the 2fa record from the db (disabling it)
*/
func (rec *TwoFAStruct) Disable(twoFACode string, ignoreCode bool) error {
	if validate2FACode(twoFACode, rec.record.secret) || ignoreCode {
		record, err := rec.app.Dao().FindRecordById("2fa_secrets", rec.record.id)
		if err != nil {
			log.Println(err)
			return NewTwoFAError("An error occurred finding the 2FA record")
		}

		if err := rec.app.Dao().DeleteRecord(record); err != nil {
			log.Println(err)
			return NewTwoFAError("An error occurred deleting the 2FA record")
		}
		return nil
	} else {
		return NewTwoFAError("Invalid 2FA code")
	}
}

/*
Check that the provided 2FA code is valid and enabled

# USED FOR AUTH CHECK
*/
func (rec *TwoFAStruct) AuthWith(twoFACode string) error {
	if !rec.record.enabled {
		return NewTwoFAError("2FA not enabled")
	}

	valid := validate2FACode(twoFACode, rec.record.secret)
	if !valid {
		return NewTwoFAError("Invalid 2FA code")
	}
	return nil
}

func (rec *TwoFAStruct) IsEnabled() bool {
	return rec.record.enabled
}

//Extra helper functions:

func create2FAIdentifierFromRecord(record *models.Record) string {
	identifier := security.SHA256(record.Id + record.Email() + record.Collection().Name)
	return identifier
}

func validate2FACode(code string, secret string) bool {
	if len(code) != 6 || secret == "" {
		return false
	}

	valid := totp.Validate(code, secret)
	if !valid {
		return false
	}
	return true
}
