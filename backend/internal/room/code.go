package room

import (
	"crypto/rand"
	"math/big"
)

// Excludes ambiguous chars (0, O, 1, I, L).
const codeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

func GenerateCode() string {
	b := make([]byte, 6)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(codeAlphabet))))
		b[i] = codeAlphabet[n.Int64()]
	}
	return string(b)
}
