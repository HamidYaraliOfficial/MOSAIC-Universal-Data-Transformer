// Package expression implements MOSAIC's lightweight Transformation
// Expression Language: the formula syntax used by Filter Rows, Generate
// Column, Map Values and the visual Formula Builder. Expressions look like
// `age > 18`, `price * quantity`, or `contains(name, "AI") && status == "active"`.
package expression

import (
	"fmt"
	"strings"
	"unicode"
)

type tokenKind int

const (
	tokEOF tokenKind = iota
	tokNumber
	tokString
	tokIdent
	tokLParen
	tokRParen
	tokComma
	tokOp
	tokDot
)

type token struct {
	kind tokenKind
	text string
}

// lex tokenizes an expression source string into a flat token stream.
func lex(src string) ([]token, error) {
	var toks []token
	runes := []rune(src)
	i, n := 0, len(runes)

	for i < n {
		c := runes[i]
		switch {
		case unicode.IsSpace(c):
			i++
		case c == '(':
			toks = append(toks, token{tokLParen, "("})
			i++
		case c == ')':
			toks = append(toks, token{tokRParen, ")"})
			i++
		case c == ',':
			toks = append(toks, token{tokComma, ","})
			i++
		case c == '.':
			toks = append(toks, token{tokDot, "."})
			i++
		case c == '"' || c == '\'':
			quote := c
			j := i + 1
			var b strings.Builder
			for j < n && runes[j] != quote {
				if runes[j] == '\\' && j+1 < n {
					j++
				}
				b.WriteRune(runes[j])
				j++
			}
			if j >= n {
				return nil, fmt.Errorf("expression: unterminated string literal")
			}
			toks = append(toks, token{tokString, b.String()})
			i = j + 1
		case unicode.IsDigit(c):
			j := i
			for j < n && (unicode.IsDigit(runes[j]) || runes[j] == '.') {
				j++
			}
			toks = append(toks, token{tokNumber, string(runes[i:j])})
			i = j
		case unicode.IsLetter(c) || c == '_':
			j := i
			for j < n && (unicode.IsLetter(runes[j]) || unicode.IsDigit(runes[j]) || runes[j] == '_') {
				j++
			}
			toks = append(toks, token{tokIdent, string(runes[i:j])})
			i = j
		default:
			op, width := matchOperator(runes[i:])
			if op == "" {
				return nil, fmt.Errorf("expression: unexpected character %q at %d", c, i)
			}
			toks = append(toks, token{tokOp, op})
			i += width
		}
	}
	toks = append(toks, token{tokEOF, ""})
	return toks, nil
}

var multiCharOps = []string{"&&", "||", "==", "!=", ">=", "<=", "??"}

func matchOperator(rest []rune) (string, int) {
	for _, op := range multiCharOps {
		if len(rest) >= len(op) && string(rest[:len(op)]) == op {
			return op, len(op)
		}
	}
	switch rest[0] {
	case '+', '-', '*', '/', '%', '>', '<', '!', '=':
		return string(rest[0]), 1
	}
	return "", 0
}
