package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestFishingProtection_UnknownUserAgent(t *testing.T) {
	// Setup
	handler := setupTestHandler()
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("User-Agent", "unknown-agent")

	// Execute
	handler.ServeHTTP(resp, req)

	// Assert
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.Code)
	}

	if resp.Body.String() != "passed through" {
		t.Errorf("Expected body 'passed through', got '%s'", resp.Body.String())
	}
}

func TestFishingProtection_WithConsentCookie(t *testing.T) {
	// Setup
	handler := setupTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	// Add consent cookie
	req.AddCookie(&http.Cookie{
		Name:  consentCookieName,
		Value: consentValue,
	})

	resp := httptest.NewRecorder()

	// Execute
	handler.ServeHTTP(resp, req)

	// Assert
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.Code)
	}

	if resp.Body.String() != "passed through" {
		t.Errorf("Expected body 'passed through', got '%s'", resp.Body.String())
	}
}

func TestFishingProtection_KnownBrowserNoConsent(t *testing.T) {
	// Setup
	handler := setupTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Host = "example.com"
	resp := httptest.NewRecorder()

	// Execute
	handler.ServeHTTP(resp, req)

	// Assert
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.Code)
	}

	// Check for consent form in the response
	body := resp.Body.String()
	if !strings.Contains(body, "Consent Required") {
		t.Errorf("Expected consent form in the body, got: %s", body)
	}

	// Check for CSRF token cookie
	csrfCookie := getCookie(resp, csrfTokenName)
	if csrfCookie == nil {
		t.Error("Expected CSRF token cookie to be set")
	}

	// Check for CSRF token in the form
	if !strings.Contains(body, `name="csrf_token"`) {
		t.Error("Expected CSRF token field in the form")
	}
}

func TestFishingProtection_ValidConsentSubmission(t *testing.T) {
	// Setup
	handler := setupTestHandler()

	// First get the CSRF token
	csrfToken := getCSRFToken(t, handler)

	// Now submit the consent form
	form := url.Values{}
	form.Add("original_url", "http://example.com/test")
	form.Add("consent", "true")
	form.Add("csrf_token", csrfToken)

	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	// Add CSRF cookie
	req.AddCookie(&http.Cookie{
		Name:  csrfTokenName,
		Value: csrfToken,
	})

	resp := httptest.NewRecorder()

	// Execute
	handler.ServeHTTP(resp, req)

	// Assert
	if resp.Code != http.StatusSeeOther {
		t.Errorf("Expected status code %d, got %d", http.StatusSeeOther, resp.Code)
	}

	// Check for consent cookie
	consentCookie := getCookie(resp, consentCookieName)
	if consentCookie == nil {
		t.Error("Expected consent cookie to be set")
	}

	if consentCookie != nil && consentCookie.Value != consentValue {
		t.Errorf("Expected consent cookie value '%s', got '%s'", consentValue, consentCookie.Value)
	}

	// Check that CSRF cookie is deleted (MaxAge = -1)
	csrfCookie := getCookie(resp, csrfTokenName)
	if csrfCookie == nil {
		t.Error("Expected CSRF cookie to be present (but deleted)")
	}

	if csrfCookie != nil && csrfCookie.MaxAge != -1 {
		t.Errorf("Expected CSRF cookie MaxAge -1, got %d", csrfCookie.MaxAge)
	}

	// Check redirect location
	location := resp.Header().Get("Location")
	if location == "" {
		t.Error("Expected a redirect location")
	}
}

func TestFishingProtection_InvalidCSRF(t *testing.T) {
	// Setup
	handler := setupTestHandler()

	form := url.Values{}
	form.Add("original_url", "http://example.com/test")
	form.Add("consent", "true")
	form.Add("csrf_token", "invalid-token")

	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	// Add valid CSRF cookie with different value
	req.AddCookie(&http.Cookie{
		Name:  csrfTokenName,
		Value: "different-token",
	})

	resp := httptest.NewRecorder()

	// Execute
	handler.ServeHTTP(resp, req)

	// Assert
	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, resp.Code)
	}

	// Check for error message
	body := resp.Body.String()
	if !strings.Contains(body, "CSRF validation failed") {
		t.Errorf("Expected CSRF validation error message, got: %s", body)
	}
}

func TestFishingProtection_MissingCSRFCookie(t *testing.T) {
	// Setup
	handler := setupTestHandler()

	form := url.Values{}
	form.Add("original_url", "http://example.com/test")
	form.Add("consent", "true")
	form.Add("csrf_token", "some-token")

	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	// No CSRF cookie added

	resp := httptest.NewRecorder()

	// Execute
	handler.ServeHTTP(resp, req)

	// Assert
	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, resp.Code)
	}
}

// Helper functions

func setupTestHandler() http.Handler {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "passed through")
	})

	return NewFishingProtection()(nextHandler)
}

func getCookie(recorder *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}

	return nil
}

func getCSRFToken(t *testing.T, handler http.Handler) string {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Host = "example.com"
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	// Get the CSRF token from the cookie
	csrfCookie := getCookie(resp, csrfTokenName)
	if csrfCookie == nil {
		t.Fatal("Failed to get CSRF token from cookie")
	}

	return csrfCookie.Value
}
