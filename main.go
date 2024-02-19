package main

import (
	"log"
	"os"

	"suddsy.dev/m/v2/app/authentication"
	"suddsy.dev/m/v2/app/user"
	"suddsy.dev/m/v2/tools"

	"github.com/joho/godotenv"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/cron"
	"github.com/spf13/cobra"
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
		user.RegisterPingRoutes(e, app)
		tools.CreateDownloadEndpoint(e, app)
		authentication.RegisterSSORoutes(e, app)

		scheduler := cron.New()
		tools.EnableAutoResetCron(app, scheduler)
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
		return user.NewAccountSetup(e, app)
	})

	app.OnRecordAfterDeleteRequest().Add(func(e *core.RecordDeleteEvent) error {
		if e.Collection.Type == "auth" {
			//Make sure the flags are deleted on delete
			user.DeleteUserFlagsOnAccountDelete(e, app)
		}
		return nil
	})

	app.RootCmd.AddCommand(&cobra.Command{
		Use: "updateme",
		Run: func(cmd *cobra.Command, args []string) {
			tools.Update()
		},
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
