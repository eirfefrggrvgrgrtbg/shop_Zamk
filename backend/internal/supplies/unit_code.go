package supplies

import (
	"crypto/rand"
	"math/big"
)

const unitAlphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"

func GenerateUnitCode() (string, error) {
	b := make([]byte, 16)
	alphabetLen := big.NewInt(int64(len(unitAlphabet)))
	
	for i := 0; i < 16; i++ {
		n, err := rand.Int(rand.Reader, alphabetLen)
		if err != nil {
			return "", err
		}
		b[i] = unitAlphabet[n.Int64()]
	}
	
	return "ZMU-" + string(b), nil
}
