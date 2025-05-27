package token

type TokenType string

const (
	WebToken TokenType = "web"
)

func (t TokenType) Prefix() string {
	switch t {
	case WebToken:
		return "w"
	}

	panic("unknown token type")
}
