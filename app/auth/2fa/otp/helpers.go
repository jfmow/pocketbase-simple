package otp

import (
	"log"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

var (
	otpCollectionName = "otp_secrets"
)

type OTPKey struct {
	Secret string
	Url    string
	Key    *otp.Key
}

func generateOTPSecretKey(app *pocketbase.PocketBase, uniqueUserIdentifer string) (*OTPKey, error) {

	appName := app.Settings().Meta.AppName

	if len(uniqueUserIdentifer) < 5 {
		return nil, NewOTPError("User indetifer too short or missing")
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      appName,
		AccountName: uniqueUserIdentifer,
	})
	if err != nil {
		log.Fatal(err)
	}

	keyData := &OTPKey{
		Secret: key.Secret(),
		Url:    key.URL(),
		Key:    key,
	}

	return keyData, nil
}

func verifyOTP(uniqueUserIdentifer string, secret string, code string) (bool, error) {
	if len(code) != 6 {
		return false, NewOTPError("Invalid code")
	}
	if uniqueUserIdentifer == "" || secret == "" {
		return false, NewOTPError("Missing identifer or secret")
	}
	valid := totp.Validate(code, secret)
	return valid, nil
}

func CreateNewOTPRecord(app *pocketbase.PocketBase, uniqueUserIdentifer string) (string, error) {

	otp, err := generateOTPSecretKey(app, uniqueUserIdentifer)
	if err != nil {
		return "", apis.NewApiError(500, err.Error(), nil)
	}

	collection, err := app.Dao().FindCollectionByNameOrId(otpCollectionName)
	if err != nil {
		return "", err
	}

	record := models.NewRecord(collection)

	record.Set("otp_secret", otp.Secret)
	record.Set("user_idenity", uniqueUserIdentifer)

	if err := app.Dao().SaveRecord(record); err != nil {
		return "", err
	}

	return otp.Url, nil
}

func RemoveOTPRecord(app *pocketbase.PocketBase, uniqueUserIdentifer string) error {
	record, err := app.Dao().FindFirstRecordByData(otpCollectionName, "user_idenity", uniqueUserIdentifer)

	if err != nil || record == nil {
		return NewOTPError("An error occured finding the otp record")
	}

	if err := app.Dao().DeleteRecord(record); err != nil {
		return err
	}

	return nil

}

func AuthWithOTP(app *pocketbase.PocketBase, uniqueUserIdentifer string, otp string) (bool, error) {
	record, err := app.Dao().FindFirstRecordByData(otpCollectionName, "user_idenity", uniqueUserIdentifer)

	if err != nil {
		return false, err
	}

	secret := record.GetString("otp_secret")

	isValid, err := verifyOTP(uniqueUserIdentifer, secret, otp)
	if err != nil {
		return false, apis.NewApiError(500, err.Error(), nil)
	}

	if !isValid {
		return false, nil
	} else {
		return true, nil
	}
}

func CheckIfOTPUserExists(app *pocketbase.PocketBase, uniqueUserIdentifer string) bool {
	record, err := app.Dao().FindFirstRecordByData(otpCollectionName, "user_idenity", uniqueUserIdentifer)

	if err != nil || record == nil {
		return false
	}

	return true
}
