package middleware

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/mileusna/useragent"
)

const (
	consentCookieName = "consent"
	consentValue      = "approved"
	csrfTokenName     = "csrf_token"
	csrfTokenLength   = 32
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
            <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
            <button type="submit" class="button">I understand, proceed to the site</button>
        </form>
    </div>
</body>
</html>
`

type templateData struct {
	OriginalURL string
	CurrentURL  string
	CSRFToken   string
}

// generateCSRFToken creates a new CSRF token as a random 32-byte string encoded in base64.
// It returns the generated token and an error if secure random data generation fails.
func generateCSRFToken() (string, error) {
	bytes := make([]byte, csrfTokenLength)

	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(bytes), nil
}

// NewFishingProtection creates a middleware to enforce user consent for accessing proxies or private sites.
// It validates user requests based on user-agent parsing, CSRF tokens, and consent cookies.
// Returns a middleware function wrapping an HTTP handler to enforce protection rules. Errors from CSRF token generation or template execution are returned as HTTP 500 responses.
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

			// Check if consent cookie exists
			cookie, err := r.Cookie(consentCookieName)
			if err == nil && cookie.Value == consentValue {
				// User has already consented, proceed with the request
				next.ServeHTTP(w, r)
				return
			}

			// Handle consent submission
			if r.Method == http.MethodPost && r.FormValue("consent") == "true" {
				handleConsentFormSubmission(w, r)
				return
			}

			renderConsentForm(w, r, tmpl)
		})
	}
}

// renderConsentForm renders an HTTP consent form to request user acknowledgement for proxy access.
// It generates a CSRF token to ensure secure interaction, sets a CSRF token cookie, and populates the form with dynamic data.
// Errors occur during CSRF token generation or template execution, responding with HTTP 500 in these cases.
func renderConsentForm(w http.ResponseWriter, r *http.Request, tmpl *template.Template) {
	// For known browsers without consent, show the consent form
	currentPath := r.URL.String()
	if currentPath == "" {
		currentPath = "/"
	}

	// Construct absolute URL
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}

	host := r.Host
	absoluteURL := fmt.Sprintf("%s://%s%s", scheme, host, currentPath)

	// Generate CSRF token
	csrfToken, err := generateCSRFToken()
	if err != nil {
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	// Set CSRF token cookie
	csrfCookie := http.Cookie{
		Name:     csrfTokenName,
		Value:    csrfToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, &csrfCookie)

	data := templateData{
		OriginalURL: absoluteURL,
		CurrentURL:  currentPath,
		CSRFToken:   csrfToken,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	if err = tmpl.Execute(w, data); err != nil {
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}
}

// handleConsentFormSubmission processes a user's form submission, validating CSRF tokens and setting consent cookies.
// It ensures CSRF token validity by comparing form data and cookie values, deletes the CSRF token cookie if valid,
// and redirects users to the original requested URL. Errors occur on invalid CSRF tokens or request parsing.
func handleConsentFormSubmission(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
	}

	// Verify CSRF token
	formToken := r.FormValue("csrf_token")
	csrfCookie, csrfErr := r.Cookie(csrfTokenName)

	if csrfErr != nil || formToken == "" || formToken != csrfCookie.Value {
		// CSRF validation failed, show the consent form again
		http.Error(w, "Invalid request: CSRF validation failed", http.StatusBadRequest)
		return
	}

	// Delete the CSRF token cookie since it's no longer needed
	http.SetCookie(w, &http.Cookie{
		Name:     csrfTokenName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	originalURL := r.FormValue("original_url")
	if originalURL == "" {
		originalURL = "/"
	} else {
		// Replace backslashes with forward slashes
		originalURL = strings.ReplaceAll(originalURL, "\\", "/")

		// Parse the URL to validate it
		parsedURL, err := url.Parse(originalURL)
		if err != nil || parsedURL.Hostname() != "" {
			// If invalid or not a relative URL, redirect to the default safe URL
			originalURL = "/"
		}
	}

	// Set consent cookie
	cookie := http.Cookie{
		Name:     consentCookieName,
		Value:    consentValue,
		Path:     "/",
		MaxAge:   3600 * 24, // 24 hours
		HttpOnly: true,
		SameSite: http.SameSiteNoneMode,
	}
	http.SetCookie(w, &cookie)

	// Redirect to the originally requested URL
	http.Redirect(w, r, originalURL, http.StatusSeeOther)
}
