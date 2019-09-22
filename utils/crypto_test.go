package utils

import (
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetHashWithArgon2(t *testing.T) {

	password := "SuperStreetFighter2Turbo"
	salt := "MBisonDidNothingWrong"

	hashy := GetHashWithArgon(password, salt, 1, 12, 64)
	assert.NotNil(t, hashy)

	t.Logf("Password: %s\tLength: %d\r\n", password, len(password))
	t.Logf("Hashed Password: %s\tLength: %d\r\n", hashy, len(hashy))

	base64Hash := make([]byte, base64.StdEncoding.EncodedLen(len(hashy)))
	base64.StdEncoding.Encode(base64Hash, hashy)

	t.Logf("Hashed As Base64: %s\tLength: %d\r\n", base64Hash, len(base64Hash))
}

func BenchmarkGetHashWithArgon2(b *testing.B) {

	password := "SuperStreetFighter2Turbo"
	salt := "MBisonDidNothingWrong"

	for i := 0; i < 1000; i++ {
		GetHashWithArgon(fmt.Sprintf(password+"-%d", i), salt, 1, 6, 64)
	}
}

func TestGetHashStringWithArgon2(t *testing.T) {

	password := "SuperStreetFighter2Turbo"
	salt := "MBisonDidNothingWrong"

	base64Hash := GetStringHashWithArgon(password, salt, 1, 12, 64)
	assert.NotNil(t, base64Hash)

	t.Logf("Password: %s\tLength: %d\r\n", password, len(password))
	t.Logf("Hash As Base64: %s\tLength: %d\r\n", base64Hash, len(base64Hash))
}

func TestHashAndAesEncrypt(t *testing.T) {

	dataPayload := []byte("\x68\x65\x6c\x6c\x6f\x20\x77\x6f\x72\x6c\x64")
	password := "SuperStreetFighter2Turbo"
	salt := "MBisonDidNothingWrong"

	hashy := GetHashWithArgon(password, salt, 1, 12, 32)
	assert.NotNil(t, hashy)

	base64Hash := make([]byte, base64.StdEncoding.EncodedLen(len(hashy)))
	base64.StdEncoding.Encode(base64Hash, hashy)

	t.Logf("Password: %s\tLength: %d\r\n", password, len(password))
	t.Logf("Password Hashed As Base64: %s\tLength: %d\r\n", base64Hash, len(base64Hash))

	encrypted, err := EncryptWithAes(dataPayload, hashy, 0)
	assert.NoError(t, err)

	base64Encrypted := make([]byte, base64.StdEncoding.EncodedLen(len(encrypted)))
	base64.StdEncoding.Encode(base64Encrypted, encrypted)

	t.Logf("Secret Payload: %s\r\n", string(dataPayload))
	t.Logf("Encrypted As Base64: %s\tLength: %d\r\n", base64Encrypted, len(base64Encrypted))
}

func TestHashAndAesEncryptAndDecrypt(t *testing.T) {

	dataPayload := []byte("\x68\x65\x6c\x6c\x6f\x20\x77\x6f\x72\x6c\x64")
	password := "SuperStreetFighter2Turbo"
	salt := "MBisonDidNothingWrong"

	hashy := GetHashWithArgon(password, salt, 1, 12, 32)
	assert.NotNil(t, hashy)

	base64Hash := make([]byte, base64.StdEncoding.EncodedLen(len(hashy)))
	base64.StdEncoding.Encode(base64Hash, hashy)

	t.Logf("Password: %s\tLength: %d\r\n", password, len(password))
	t.Logf("Password Hashed As Base64: %s\tLength: %d\r\n", base64Hash, len(base64Hash))

	encryptedPayload, err := EncryptWithAes(dataPayload, hashy, 12)
	assert.NoError(t, err)

	base64Encrypted := make([]byte, base64.StdEncoding.EncodedLen(len(encryptedPayload)))
	base64.StdEncoding.Encode(base64Encrypted, encryptedPayload)

	t.Logf("Original Payload: %s\r\n", string(dataPayload))
	t.Logf("Encrypted As Base64: %s\tLength: %d\r\n", base64Encrypted, len(base64Encrypted))

	decryptedPayload, err := DecryptWithAes(encryptedPayload, hashy, 12)
	assert.NoError(t, err)
	t.Logf("Decrypted Payload: %s\r\n", string(decryptedPayload))

	assert.Equal(t, dataPayload, decryptedPayload)
}
