package emails

import (
	"bytes"
	"html/template"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase"
)

type CachedEmail struct {
	Template *template.Template
	StoredAt time.Time
}

var (
	cache      = make(map[string]CachedEmail)
	cacheMutex sync.Mutex
)

func LoadEmailDataToHTML(app *pocketbase.PocketBase, emailName string, data map[string]interface{}) (string, error) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	var emailTemplate *template.Template

	// Check if the email template is already cached and within the 5-minute validity period
	if cachedEmail, ok := cache[emailName]; ok && time.Since(cachedEmail.StoredAt) <= 1*time.Minute {
		emailTemplate = cachedEmail.Template
	} else {
		// If not cached or cache expired, fetch from the database
		record, err := app.Dao().FindFirstRecordByData("custom_emails", "name", emailName)
		if err != nil {
			return "", err
		}

		emailHtmlString := record.GetString("email_rich")

		// Parse the HTML string as a template
		emailTemplate, err = template.New(emailName).Parse(emailHtmlString)
		if err != nil {
			return "", err
		}

		// Update the cache with the new template and HTML string
		cache[emailName] = CachedEmail{
			Template: emailTemplate,
			StoredAt: time.Now().UTC(),
		}
	}

	var modifiedHTMLBuffer bytes.Buffer

	// Apply the dynamic data to the template and write the result to the buffer
	err := emailTemplate.Execute(&modifiedHTMLBuffer, data)
	if err != nil {
		return "", err
	}

	// Get the final HTML string with dynamic content
	return centerEmailContent(modifiedHTMLBuffer.String()), nil
}

/*
Center an email horizontaly using a table

Sets max width to 600px also
*/
func centerEmailContent(htmlString string) string {
	centeredTable := `
<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">
<html dir="ltr" lang="en">
<head>
    <meta content="width=device-width" name="viewport" />
    <link rel="preload" as="image" href="https://note.suddsy.dev/logo-small-email.png" />
    <meta content="text/html; charset=UTF-8" http-equiv="Content-Type" />
    <meta content="IE=edge" http-equiv="X-UA-Compatible" />
    <meta name="x-apple-disable-message-reformatting" />
    <meta content="telephone=no,address=no,email=no,date=no,url=no" name="format-detection" />
    <meta content="light" name="color-scheme" />
    <meta content="light" name="supported-color-schemes" />
    <style>
        @font-face {
            font-family: 'Inter';
            font-style: normal;
            font-weight: 400;
            mso-font-alt: 'sans-serif';
            src: url(https://rsms.me/inter/font-files/Inter-Regular.woff2?v=3.19) format('woff2');
        }
        * {
            font-family: 'Inter', sans-serif;
        }
    </style>
    <style>
        blockquote, h1, h2, h3, img, li, ol, p, ul {
            margin-top: 1em;
            margin-bottom: 1em;
        }
    </style>
</head>
<body style="margin: 0; padding: 0; background-color: #ffffff;">
    <table role="presentation" style="width: 100%; border-collapse: collapse; margin: 0; padding: 0; background-color: #ffffff;">
        <tr>
            <td align="center" style="padding: 20px 0;">
                <table role="presentation" style="max-width: 540px; width: 100%; border-collapse: collapse; background-color: #ffffff; margin: 0 auto;">
                    <tr>
                        <td style="padding: 20px; text-align: left; font-family: Arial, sans-serif; font-size: 16px; color: #333333;">
                            ` + htmlString + `
                        </td>
                    </tr>
                </table>
            </td>
        </tr>
    </table>
</body>
</html>`

	// Output or use the final HTML content
	return centeredTable
}
