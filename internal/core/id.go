package core

import "crypto/rand"

// An id is four base36 characters with at least one digit. Short enough to type and to
// sit in a bullet on a phone, and never a word: `done` and `take` cannot be one, so a
// bare token on the command line is an id or a filter and never both.
const (
	idLen      = 4
	idAlphabet = "0123456789abcdefghijklmnopqrstuvwxyz"
)

// NewID is a fresh id. The store checks it against the rows it has and asks again on
// the one-in-a-million collision; permanence is the store's promise, this is only shape.
func NewID() string {
	for {
		raw := make([]byte, idLen)
		rand.Read(raw)
		out := make([]byte, idLen)
		for i, b := range raw {
			out[i] = idAlphabet[int(b)%len(idAlphabet)]
		}
		if id := string(out); IsID(id) {
			return id
		}
	}
}

// IsID is the shape check every surface shares: the CLI's head tokens, the mirror's
// parentheses, TRAY_IDS.
func IsID(s string) bool {
	if len(s) != idLen {
		return false
	}
	digit := false
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			digit = true
		case r >= 'a' && r <= 'z':
		default:
			return false
		}
	}
	return digit
}
