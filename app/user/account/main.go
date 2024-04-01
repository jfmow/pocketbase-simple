package account

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
	"suddsy.dev/m/v2/tools"
)

func NewAccountSetup(e *core.RecordCreateEvent, app *pocketbase.PocketBase) error {
	user := e.Record.Id

	if runtime.GOOS != "linux" {
		log.Panic("Update cancled. This tool only works on linux systems :(")
		return nil
	}

	err, _ := CreatePreviewPage(app, user)
	if err != nil {
		return err
	}

	userFlagsCollection, err := app.Dao().FindCollectionByNameOrId("user_flags")
	if err != nil {
		return err
	}

	newUserFlagsRecord := models.NewRecord(userFlagsCollection)

	newUserFlagsRecord.Set("user", user)
	newUserFlagsRecord.Set("collection", e.Record.Collection().Id)

	// In bytes
	newUserFlagsRecord.Set("maxUploadSize", 5485760)
	newUserFlagsRecord.Set("quota", 10485760)

	if sso, ok := e.HttpContext.Get("sso").(bool); ok {
		// Value exists, proceed with your logic
		if sso {
			newUserFlagsRecord.Set("sso", true)
		}
	}

	if err := app.Dao().SaveRecord(newUserFlagsRecord); err != nil {
		return err
	}

	return nil
}

func DeleteUserFlagsOnAccountDelete(e *core.RecordDeleteEvent, app *pocketbase.PocketBase) error {
	// Make sure the flags are deleted on delete
	record, err := app.Dao().FindFirstRecordByData("user_flags", "user", e.Record.Id)
	if err != nil {
		return err
	}

	if err := app.Dao().DeleteRecord(record); err != nil {
		return err
	}
	return nil
}

func CreatePreviewPage(app *pocketbase.PocketBase, user string) (error, string) {
	WorkingDir := tools.GetWorkDir()
	type Page struct {
		Content  json.RawMessage `json:"content"`
		Shared   bool            `json:"shared"`
		Id       string          `json:"id"`
		Title    string          `json:"title"`
		Icon     string          `json:"icon"`
		Unsplash string          `json:"unsplash"`
	}
	var previewPage Page

	PreviewPageFile, err := os.ReadFile(filepath.Join(WorkingDir, "preview_page.json"))
	if err != nil {
		log.Println("Failed to read preview_page file/find it")
		return nil, ""
	} else {
		err = json.Unmarshal(PreviewPageFile, &previewPage)
		if err != nil {
			return err, ""
		}

		collection, err := app.Dao().FindCollectionByNameOrId("pages")
		if err != nil {
			return err, ""
		}

		record := models.NewRecord(collection)

		if user == "" {
			record.Set("id", previewPage.Id)
		} else {
			record.Set("owner", user)
		}

		record.Set("content", previewPage.Content)
		record.Set("shared", previewPage.Shared)
		record.Set("title", previewPage.Title)
		record.Set("icon", previewPage.Icon)
		record.Set("unsplash", previewPage.Unsplash)

		if err := app.Dao().SaveRecord(record); err != nil {
			return err, ""
		}
		return nil, record.Id
	}

}
