package main

import (
	"log"
	"os"

	twofa "suddsy.dev/m/v2/app/auth/TwoFA"
	"suddsy.dev/m/v2/app/auth/methods/emailauth"
	"suddsy.dev/m/v2/app/tools/lifetime"
	"suddsy.dev/m/v2/app/user"
	"suddsy.dev/m/v2/app/user/account"
	"suddsy.dev/m/v2/app/user/pages"

	"github.com/joho/godotenv"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/cron"
)

func main() {
	app := pocketbase.New()
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading from a .env FILE (if in docker don't worry if your using compose)")
	}

	// serves static files from the provided public dir (if exists)
	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		e.Router.GET("/*", apis.StaticDirectoryHandler(os.DirFS("./pb_public"), false))
		emailauth.RegisterEmailAuthRoutes(e, app)
		pages.RegisterAccPagesRoutes(e, app)
		account.HandleRegisterRoutes(e, app)
		twofa.Register2FARoutes(e, app)

		scheduler := cron.New()
		lifetime.EnableAutoResetCron(app, scheduler)
		scheduler.Start()

		return nil
	})

	app.OnRecordBeforeCreateRequest("files").Add(func(e *core.RecordCreateEvent) error {
		return user.HandleCreateEvent(e, app)
	})

	app.OnRecordAfterUpdateRequest("pages").Add(func(e *core.RecordUpdateEvent) error {
		go user.CheckFilesMatchBlocks(app, e)
		return nil
	})

	app.OnRecordAfterCreateRequest("users").Add(func(e *core.RecordCreateEvent) error {
		return account.NewAccountSetup(e, app)
	})

	app.OnRecordAfterUnlinkExternalAuthRequest().Add(func(e *core.RecordUnlinkExternalAuthEvent) error {
		return emailauth.EnableFromOAuthUnlink(app, e)
	})

	app.OnRecordAfterDeleteRequest().Add(func(e *core.RecordDeleteEvent) error {
		if e.Collection.Type == "auth" {
			//Make sure the flags are deleted on delete
			account.DeleteUserFlagsOnAccountDelete(e, app)
		}
		return nil
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}

}
