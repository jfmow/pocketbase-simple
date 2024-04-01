package emails

import (
	"bytes"
	"html/template"
	"os"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/tools/security"
)

func LoadHtmlFile(filePath string, data map[string]interface{}) (error, string) {
	htmlFile, err := os.ReadFile(filePath)
	if err != nil {
		return err, ""
	}

	// Convert the HTML file content to a string
	htmlString := string(htmlFile)
	tmpl, err := template.New(security.RandomString(12)).Parse(htmlString)
	if err != nil {
		return err, ""
	}
	var modifiedHTMLBuffer bytes.Buffer

	// Apply the dynamic data to the template and write the result to the buffer
	err = tmpl.Execute(&modifiedHTMLBuffer, data)
	if err != nil {
		return err, ""
	}

	// Get the final HTML string with dynamic content
	return nil, modifiedHTMLBuffer.String()
}

type CachedEmail struct {
	HTMLString string
	StoredAt   time.Time
}

var (
	cache      = make(map[string]CachedEmail)
	cacheMutex sync.Mutex
)

func LoadEmailDataToHTML(app *pocketbase.PocketBase, emailName string, data map[string]interface{}) (string, error) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	var htmlString string

	// Check if the email is already cached and within the 5-minute validity period
	if cachedEmail, ok := cache[emailName]; ok && time.Since(cachedEmail.StoredAt) <= 5*time.Minute {
		htmlString = cachedEmail.HTMLString
	} else {
		record, err := app.Dao().FindFirstRecordByData("custom_emails", "name", emailName)
		if err != nil {
			return "", err
		}

		htmlString = record.GetString("email")
	}

	// If not cached or cache expired, fetch from the database

	// Convert the HTML file content to a string
	tmpl, err := template.New(emailName).Parse(htmlString)
	if err != nil {
		return "", err
	}
	var modifiedHTMLBuffer bytes.Buffer

	// Apply the dynamic data to the template and write the result to the buffer
	err = tmpl.Execute(&modifiedHTMLBuffer, data)
	if err != nil {
		return "", err
	}

	// Update the cache with the new data and timestamp
	cache[emailName] = CachedEmail{
		HTMLString: htmlString,
		StoredAt:   time.Now().UTC(),
	}

	// Get the final HTML string with dynamic content
	return modifiedHTMLBuffer.String(), nil
}
