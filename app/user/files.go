package user

import (
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func HandleCreateEvent(e *core.RecordCreateEvent, app *pocketbase.PocketBase) error {
	//Check if the user is part of the default user group, and if so find there flags, else return nil
	if e.Record.Collection().Name != "users" {
		return nil
	}

	record, err := app.Dao().FindFirstRecordByData("user_flags", "user", e.Record.Id)
	if err != nil || record.Id == "" {
		return apis.NewUnauthorizedError("User does not have correct permisions", nil)
	}

	_, fs, _ := e.HttpContext.Request().FormFile("file_data")
	uploadedFileSize := int(fs.Size)

	if uploadedFileSize > record.GetInt("maxUploadSize") {
		return apis.NewBadRequestError("File too large!", nil)
	}

	return nil
}

func CheckFilesMatchBlocks(app *pocketbase.PocketBase, c *core.RecordUpdateEvent) error {
	/**
	This function checks to see if the images (igms) or files (simpleEmbed) are stored in the db tables but are also actually in the stored page.
	If they aren't, then they are deleted from the db.

	To prevent deletion of files by mistake it checks the data that was just sent to the server rather than getting it from the server and potentialy being ahed of the servers db saving
	*/
	if c.Record.GetTime("last_file_check").After(time.Now().Add(-1 * time.Hour)) {
		// Return because the record has been checked less than 1 hour ago
		//This reduces the overall pressure on the db as it finds all the records for a page, it now only has to do this when a page is updated and once per hour. (This time may be extended in high traffic enviroments but it doesn't really make a difference)
		return nil
	}

	jsonData := c.Record.GetString("content")

	type BlockData struct {
		Text   string `json:"text"`
		FileId string `json:"fileId"`
	}

	type Block struct {
		Data BlockData `json:"data"`
		ID   string    `json:"id"`
		Type string    `json:"type"`
	}

	type Record struct {
		Blocks  []Block `json:"blocks"`
		Time    int64   `json:"time"`
		Version string  `json:"version"`
	}

	var record Record

	err := json.Unmarshal([]byte(jsonData), &record)
	if err != nil {
		log.Fatal(err)
	}

	imageBlocks := []Block{}
	fileBlocks := []Block{}
	for _, block := range record.Blocks {
		if block.Type == "image" {
			imageBlocks = append(imageBlocks, block)
		} else if block.Type == "simpleEmbeds" {
			fileBlocks = append(fileBlocks, block)
		}
	}

	imgsRecords, err := app.Dao().FindRecordsByExpr("imgs",
		dbx.NewExp("page = {:pageId}", dbx.Params{"pageId": c.Record.Id}),
	)
	if err != nil {
		return err
	}
	filesRecords, err := app.Dao().FindRecordsByExpr("files",
		dbx.NewExp("page = {:pageId}", dbx.Params{"pageId": c.Record.Id}),
	)
	if err != nil {
		return err
	}

	isStringInImageBlocks := func(imageBlocks []Block, searchString string) bool {
		for _, block := range imageBlocks {
			// Assuming you want to check the Text field for the presence of the searchString
			if strings.Contains(block.Data.FileId, searchString) {
				return true
			}
		}
		return false
	}

	for _, record := range imgsRecords {
		if !isStringInImageBlocks(imageBlocks, record.Id) {
			if err := app.Dao().DeleteRecord(record); err != nil {
				log.Println("Error deleting an img that has no page: " + record.Id)
			}
		}
	}

	for _, record := range filesRecords {
		if !isStringInImageBlocks(fileBlocks, record.Id) {
			if err := app.Dao().DeleteRecord(record); err != nil {
				log.Println("Error deleting an file that has no page: " + record.Id)
			}
		}
	}
	c.Record.Set("last_file_check", time.Now().UTC())
	if err := app.Dao().SaveRecord(c.Record); err != nil {
		return err
	}
	return nil
}
