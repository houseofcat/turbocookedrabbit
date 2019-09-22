package utils

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRandomString(t *testing.T) {

	randoString := RandomString(20)
	assert.NotEqual(t, "", randoString)
	t.Logf("RandoString1: %s", randoString)

	time.Sleep(1 * time.Nanosecond)

	anotherRandoString := RandomString(20)
	assert.NotEqual(t, "", anotherRandoString)
	t.Logf("RandoString2: %s", anotherRandoString)

	assert.NotEqual(t, randoString, anotherRandoString)
}

func TestRandomStringFromSource(t *testing.T) {

	src := rand.NewSource(time.Now().UnixNano())

	randoString := RandomStringFromSource(10, src)
	assert.NotEqual(t, "", randoString)
	t.Logf("RandoString1: %s", randoString)

	anotherRandoString := RandomStringFromSource(10, src)
	assert.NotEqual(t, "", anotherRandoString)
	t.Logf("RandoString2: %s", anotherRandoString)

	assert.NotEqual(t, randoString, anotherRandoString)
}
