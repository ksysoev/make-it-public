package middleware

import (
	"html/template"
	"net/http"

	"github.com/mileusna/useragent"
)

const (
	consentCookieName = "consent"
	consentValue      = "approved"
)

var consentFormTemplate = `
<!DOCTYPE html>
<html>
<head>
    <title>Access Request</title>
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        body {
            font-family: Arial, sans-serif;
            margin: 0;
            padding: 20px;
            background-color: #f5f5f5;
            color: #333;
        }
        .container {
            max-width: 600px;
            margin: 40px auto;
            background-color: white;
            border-radius: 8px;
            padding: 20px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        }
        h1 {
            color: #333;
            font-size: 24px;
            margin-top: 0;
        }
        p {
            line-height: 1.6;
        }
        .warning {
            background-color: #fff8e1;
            border-left: 4px solid #ffc107;
            padding: 12px;
            margin-bottom: 20px;
        }
        .button {
            background-color: #4CAF50;
            border: none;
            color: white;
            padding: 10px 20px;
            text-align: center;
            text-decoration: none;
            display: inline-block;
            font-size: 16px;
            margin-top: 10px;
            cursor: pointer;
            border-radius: 4px;
        }
        .button:hover {
            background-color: #45a049;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="warning">
            <p><strong>Attention:</strong> You're accessing a site via MakeItPublic proxy service.</p>
        </div>
        <h1>Consent Required</h1>
        <p>You are attempting to access a page that is being served through the MakeItPublic proxy service. This service makes local or private servers temporarily accessible to the public.</p>
        <p>To continue, please confirm that you want to proceed to:</p>
        <p><strong>{{.OriginalURL}}</strong></p>
        <form method="POST" action="{{.CurrentURL}}">
            <input type="hidden" name="original_url" value="{{.OriginalURL}}">
            <input type="hidden" name="consent" value="true">
            <button type="submit" class="button">I understand, proceed to the site</button>
        </form>
    </div>
</body>
</html>
`

type templateData struct {
	OriginalURL string
	CurrentURL  string
}

func NewFishingProtection() func(next http.Handler) http.Handler {
	tmpl := template.Must(template.New("consent").Parse(consentFormTemplate))

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Parse user agent
			ua := useragent.Parse(r.UserAgent())

			// If unknown user agent, assume it's not a real browser and allow the request
			if ua.IsUnknown() {
				next.ServeHTTP(w, r)
				return
			}

			// Handle consent submission
			if r.Method == http.MethodPost && r.FormValue("consent") == "true" {
				if err := r.ParseForm(); err == nil {
					originalURL := r.FormValue("original_url")
					if originalURL == "" {
						originalURL = "/"
					}

					// Set consent cookie
					cookie := http.Cookie{
						Name:     consentCookieName,
						Value:    consentValue,
						Path:     "/",
						MaxAge:   3600 * 24, // 24 hours
						HttpOnly: true,
						SameSite: http.SameSiteStrictMode,
					}
					http.SetCookie(w, &cookie)

					// Redirect to the originally requested URL
					http.Redirect(w, r, originalURL, http.StatusSeeOther)
					return
				}
			}

			// Check if consent cookie exists
			cookie, err := r.Cookie(consentCookieName)
			if err == nil && cookie.Value == consentValue {
				// User has already consented, proceed with the request
				next.ServeHTTP(w, r)
				return
			}

			// For known browsers without consent, show the consent form
			currentURL := r.URL.String()
			if currentURL == "" {
				currentURL = "/"
			}

			data := templateData{
				OriginalURL: currentURL,
				CurrentURL:  currentURL,
			}

			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			tmpl.Execute(w, data)
		})
	}
}
