package emails

import (
	"bytes"
	"html/template"
	"os"

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
