package tokens

import (
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/tools/security"
)

type Token struct {
	Value   string
	Expires time.Time
	User    *TokenUser
	Reason  string
	App     *pocketbase.PocketBase
}

type TokenUser struct {
	Email      string
	Collection *models.Collection
	Exists     bool
}

func Initialise(email string, collection *models.Collection, exists bool) *TokenUser {
	return &TokenUser{
		Email:      email,
		Collection: collection,
		Exists:     exists,
	}
}

/*
Initalizes a new token.

This does not save it to the db yet. No expire is set yet either

Reason must match when querying for a token eg, emailauth, when quthing set as reason then when checking supply as reason.

Reason can be generic. Non unique
*/
func (user *TokenUser) CreateNewToken(reason string, app *pocketbase.PocketBase) (*Token, error) {
	record, err := user.findUserRecord(app)

	//Check to see if the user does exist else error and check to see if user doesn't exists else error
	if user.Exists && (err != nil || record == nil) {
		return nil, NewTokenError("User %s not found in collection %s. User was marked as should exist", user.Email, user.Collection.Name)
	}
	if !user.Exists && record != nil {
		return nil, NewTokenError("User %s found in collection %s. When the user is marked as should not exist", user.Email, user.Collection.Name)
	}

	randomTokenString := security.RandomString(15)

	token := &Token{
		Value:  randomTokenString,
		User:   user,
		Reason: reason,
		App:    app,
	}

	return token, nil
}

/*
Saves the token

Writes to db. Sets expirey time to 5 min from now

Returns an updated token
*/
func (token *Token) Save() (*Token, error) {

	//Check the user exists
	record, err := token.User.findUserRecord(token.App)

	//Check to see if the user does exist else error and check to see if user doesn't exists else error
	if token.User.Exists && (err != nil || record == nil) {
		return nil, NewTokenError("User %s not found in collection %s. User was marked as should exist", token.User.Email, token.User.Collection.Name)
	}
	if !token.User.Exists && record != nil {
		return nil, NewTokenError("User %s found in collection %s. When the user is marked as should not exist", token.User.Email, token.User.Collection.Name)
	}

	tokenExpiryDate := time.Now().UTC().Add(5 * time.Minute)

	collection, err := token.App.Dao().FindCollectionByNameOrId("tokens")
	if err != nil {
		return nil, NewTokenError("tokens Collection was not found. Please create it to use this feature.")
	}

	tokenCollectionRecord := models.NewRecord(collection)

	// set individual fields
	// or bulk load with record.Load(map[string]any{...})
	tokenCollectionRecord.Set("user_email", token.User.Email)
	tokenCollectionRecord.Set("auth_collection_id", token.User.Collection.Id)
	tokenCollectionRecord.Set("token", security.SHA256(token.Value))
	tokenCollectionRecord.Set("expires", tokenExpiryDate)
	tokenCollectionRecord.Set("reason", token.Reason)

	if err := token.App.Dao().SaveRecord(tokenCollectionRecord); err != nil {
		return nil, NewTokenError("Failed to create token.\n%s", err)
	}

	token.Expires = tokenExpiryDate

	return token, nil

}

func (user *TokenUser) RebuildToken(token string, reason string) *Token {
	return &Token{
		Value:  token,
		User:   user,
		Reason: reason,
	}
}

/*
Verifys a token is valid

# Does not remove it, token is stil valid
*/
func (token *Token) Verify(app *pocketbase.PocketBase) error {
	tokenRecord, err := token.FindTokenByToken(app)
	if err != nil {
		return NewTokenError("No matching request found")
	}

	if time.Now().UTC().After(tokenRecord.GetDateTime("expires").Time()) {
		token.RemoveToken(app)
		return NewTokenError("Token is expired")
	}

	/*
		The token was found because:
		- Its collection and email and reason are found and token
		- its valid
	*/
	return nil
}

/*
Remove the token from the tokens table. Invalidates it
*/
func (token *Token) RemoveToken(app *pocketbase.PocketBase) error {
	tokenRecord, err := token.FindTokenByToken(app)
	if err != nil {
		return NewTokenError("Token not found")
	}
	if err := app.Dao().DeleteRecord(tokenRecord); err != nil {
		return err
	}
	return nil
}

/*
Find a token by the user email + token + collection

If not found record will be blank and there will be an generic error
*/
func (token *Token) FindTokenByToken(app *pocketbase.PocketBase) (*models.Record, error) {
	record, err := app.Dao().FindFirstRecordByFilter(
		"tokens", "token = {:token} && user_email = {:email} && auth_collection_id = {:collectionId} && reason = {:reason}",
		dbx.Params{"token": security.SHA256(token.Value), "email": token.User.Email, "collectionId": token.User.Collection.Id, "reason": token.Reason},
	)
	return record, err
}

/*
Finds a auth collection record by the email

If not found record will be blank and there will be an generic error
*/
func (user *TokenUser) findUserRecord(app *pocketbase.PocketBase) (*models.Record, error) {
	if user.Collection.Type != "auth" {
		return nil, NewTokenError("Users collection is not an Auth collection")
	}
	record, err := app.Dao().FindAuthRecordByEmail(user.Collection.Id, user.Email)
	return record, err
}
