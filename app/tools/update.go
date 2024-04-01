package tools

import (
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

/*
Logic for updating the app
*/

func CreateDownloadEndpoint(e *core.ServeEvent, app *pocketbase.PocketBase) error {
	e.Router.GET("/update/latest", func(c echo.Context) error {
		return HandleServeFileDownload(c, app)
	} /* optional middlewares */)
	e.Router.GET("/update/done", func(c echo.Context) error {
		return c.String(200, "AHHHH")
	} /* optional middlewares */)
	return nil
}

func HandleServeFileDownload(c echo.Context, app *pocketbase.PocketBase) error {
	updateApiKey := c.QueryParam("auth")

	storedKey, found := os.LookupEnv("UpdateToken")

	if !found || updateApiKey != storedKey {
		return c.JSON(http.StatusForbidden, "")
	}

	collId, err := app.Dao().FindCollectionByNameOrId("pocketbases")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, "")
	}

	type Update struct {
		Id   string `db:"id" json:"id"`
		Base string `db:"base" json:"base"`
	}
	result := Update{}

	app.Dao().DB().
		Select("pocketbases.*").
		From("pocketbases").
		OrderBy("updated DESC").One(&result)

	newFs, err := app.NewFilesystem()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, "")
	}
	return newFs.Serve(c.Response().Writer, c.Request(), collId.Id+"/"+result.Id+"/"+result.Base, "base")
}

// Update logic
func Update() error {
	var UpdateFailed bool

	if runtime.GOOS != "linux" {
		log.Panic("Update cancled. This tool only works on linux systems :(")
		return nil
	}

	WorkingDir := GetWorkDir()

	NewBaseFile := filepath.Join(WorkingDir, "base")
	OldBaseFile := filepath.Join(WorkingDir, "old-base")

	UniqueDownloadURL, found := os.LookupEnv("UpdateURL")
	if !found || UniqueDownloadURL == "" {
		log.Println("Missing enviroment vars")
		return nil
	}

	UniqueDownloadToken, found := os.LookupEnv("UpdateToken")
	if !found || UniqueDownloadToken == "" {
		log.Println("Missing enviroment vars")
		return nil
	}

	defer func() {
		if UpdateFailed {
			os.Rename(OldBaseFile, NewBaseFile)
			os.Remove(OldBaseFile)
		}
	}()

	DownloadURL := UniqueDownloadURL + "?auth=" + UniqueDownloadToken

	//Rename the current base
	os.Rename(NewBaseFile, OldBaseFile)

	//Download the new file
	log.Println("Downloading updated file")
	err := downloadTheFile(NewBaseFile, DownloadURL)
	if err != nil {
		UpdateFailed = true
		log.Println("Failed to download the new file")
		return nil
	}
	log.Println("File downloaded")

	//Give the new file executable permisions, then reboot the system to start up fresh and start cleanup
	log.Println("Giving executable permsision")
	err = os.Chmod(NewBaseFile, 0755)
	if err != nil {
		UpdateFailed = true
		log.Println("Failed to give the downloaded file executable permision")
		return err
	}
	log.Println("Executable permision given")

	time.Sleep(5 * time.Second)

	cmd := exec.Command("reboot")
	log.Println("Rebooting")
	err = cmd.Run()
	if err != nil {
		UpdateFailed = true
		log.Println("Error executing command, if you have sudo enabled, reboot your system to finish the update.")
		return err
	}

	//This will probaly not return if it's successful :), but it might if reboot takes forever ;)
	return nil
}

func downloadTheFile(filepath string, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		log.Println("Download err:", err)
		return err
	}
	defer resp.Body.Close()

	file, err := os.Create(filepath)
	if err != nil {
		log.Println("Download err 2:", err)
		return err
	}
	defer file.Close()

	size, err := io.Copy(file, resp.Body)
	if err != nil {
		return err
	}

	log.Printf("Downloaded file size: %d bytes", size)
	log.Println(filepath)
	return err
}
