package token

import (
	"fmt"
	"strings"
)

// Parser verifies and decodes ad attribution tokens.
type Parser interface {
	Parse(tokenString string) (Payload, error)
}

type parser struct {
	secret string
}

// NewParser builds a parser backed by the provided secret.
func NewParser(secret string) (Parser, error) {
	if strings.TrimSpace(secret) == "" {
		return nil, fmt.Errorf("ads token secret required")
	}
	return &parser{secret: secret}, nil
}

func (p *parser) Parse(tokenString string) (Payload, error) {
	return ParseToken(p.secret, tokenString)
}
