package token

import (
	"bytes"
	"encoding/base64"
	"fmt"
)

type Token struct {
	ID     string
	Secret string
}

func NewToken(id, secret string) *Token {
	return &Token{
		ID:     id,
		Secret: secret,
	}
}

func (t *Token) Encode() string {
	return base64.StdEncoding.EncodeToString([]byte(t.ID + ":" + t.Secret))
}

func Decode(encoded string) (*Token, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}

	parts := bytes.SplitN(data, []byte(":"), 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid token format")
	}

	return &Token{
		ID:     string(parts[0]),
		Secret: string(parts[1]),
	}, nil
}
