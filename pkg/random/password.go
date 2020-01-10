package random

import (
	"math/rand"
	"time"
)

// Shamelessly stolen from stackoverflow, the slowest solution there is
// https://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-length-in-go
const passwordChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789[]{}@!$%,.-_=\\/"

func init() {
	// As much as I hate init functions, we either have this or we always get
	// the same _random_ string fpllngzieyoh43e0
	rand.Seed(time.Now().UnixNano())
}

// Password creates a random pwd of the given length
func Password(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = passwordChars[rand.Intn(len(passwordChars))]
	}
	return string(b)
}
