package url

import (
	"errors"
	"fmt"
	"net/url"
)

// NewEndpointGenerator creates a function to generate URLs for a given schema and domain.
// It returns an error if schema or domain is empty. The returned function generates URLs based on a keyID.
// Accepts schema, the URL scheme (e.g., "http" or "https"), and domain, the domain to construct URLs for.
// Returns a function that builds URLs using the keyID or an error if parameters are invalid.
func NewEndpointGenerator(schema, domain string, port int) (func(string) (string, error), error) {
	if schema == "" {
		return nil, errors.New("schema is empty")
	}

	if domain == "" {
		return nil, errors.New("domain is empty")
	}

	return func(keyID string) (string, error) {
		if keyID == "" {
			return "", errors.New("keyID is empty")
		}

		host := fmt.Sprintf("%s.%s", keyID, domain)

		if port > 0 {
			host = fmt.Sprintf("%s:%d", host, port)
		}

		u := url.URL{
			Scheme: schema,
			Host:   host,
		}

		return u.String(), nil
	}, nil
}
