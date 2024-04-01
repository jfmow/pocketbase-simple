package lifetime

import (
	"log"
	"os"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/tools/cron"
	"suddsy.dev/m/v2/app/user/account"
)

/*
Automaticly resets the database every 6 hours

This is only for the demo website and even then doesn't really need to be here

  - Will be removed in future
*/
func EnableAutoResetCron(app *pocketbase.PocketBase, scheduler *cron.Cron) error {
	// prints "Hello!" every 2 minutes
	autoReset, found := os.LookupEnv("AUTO_RESET")

	if found && autoReset == "true" {
		scheduler.MustAdd("Reset", "0 */6 * * *", func() {
			baseCollections, err := app.Dao().FindCollectionsByType(models.CollectionTypeBase)
			if err != nil {
				panic(err)
			}
			for _, table := range baseCollections {
				if table.Name != "pocketbases" {
					_, err = app.Dao().DB().
						NewQuery("DELETE FROM " + table.Name + ";").
						Execute()
					if err != nil {
						// Handle error
						log.Panicln(err)
					}
				}

			}
			authCollections, err := app.Dao().FindCollectionsByType(models.CollectionTypeAuth)
			if err != nil {
				panic(err)
			}
			for _, table := range authCollections {

				_, err = app.Dao().DB().
					NewQuery("DELETE FROM " + table.Name + ";").
					Execute()
				if err != nil {
					// Handle error
					log.Panicln(err)
				}

			}
			account.CreatePreviewPage(app, "")
		})

	}

	return nil
}
