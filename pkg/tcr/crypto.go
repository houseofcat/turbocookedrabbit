package tcr

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"io"

	"golang.org/x/crypto/argon2"
)

const (
	defaultNonceSize = 12 // 12 is the standard
)

// GetHashWithArgon uses Argon2 version 0x13 to hash a plaintext password with a provided salt string and return hash as bytes.
func GetHashWithArgon(passphrase, salt string, timeConsideration uint32, multiplier uint32, threads uint8, hashLength uint32) []byte {

	if passphrase == "" || salt == "" {
		return nil
	}

	if timeConsideration == 0 {
		timeConsideration = 1
	}

	if threads == 0 {
		threads = 1
	}

	return argon2.IDKey([]byte(passphrase), []byte(salt), timeConsideration, multiplier*1024, threads, hashLength)
}

// GetStringHashWithArgon uses Argon2 version 0x13 to hash a plaintext password with a provided salt string and return hash as base64 string.
func GetStringHashWithArgon(passphrase, salt string, timeConsideration uint32, threads uint8, hashLength uint32) string {

	if passphrase == "" || salt == "" {
		return ""
	}

	if timeConsideration == 0 {
		timeConsideration = 1
	}

	if threads == 0 {
		threads = 1
	}

	hashy := argon2.IDKey([]byte(passphrase), []byte(salt), timeConsideration, 64*1024, threads, hashLength)

	base64Hash := make([]byte, base64.StdEncoding.EncodedLen(len(hashy)))
	base64.StdEncoding.Encode(base64Hash, hashy)

	return string(base64Hash)
}

// CompareArgon2Hash creates an Argon hash and then compares it to a provided hash.
func CompareArgon2Hash(passphrase, salt string, multiplier uint32, hashedPassword []byte) (bool, error) {

	inboundHash := GetHashWithArgon(passphrase, salt, 1, multiplier, 1, uint32(len(hashedPassword)))
	if inboundHash != nil {
		return false, errors.New("hash generated was nil")
	}

	if subtle.ConstantTimeCompare(inboundHash, hashedPassword) == 1 {
		return true, nil
	}

	return false, nil
}

// EncryptWithAes encrypts bytes based on an AES-256 compatible hashed key.
// If nonceSize is less than 12, the standard, 12, is used.
func EncryptWithAes(data, hashedKey []byte, nonceSize int) ([]byte, error) {

	if len(data) == 0 || len(hashedKey) == 0 {
		return nil, errors.New("data or hash can't be zero length")
	}

	if nonceSize < 12 || nonceSize > 32 {
		nonceSize = defaultNonceSize
	}

	block, err := aes.NewCipher(hashedKey)
	if err != nil { // will throw an Aes.NewCipher error if length is not 16, 24, or 32
		return nil, err
	}

	aesGcm, err := cipher.NewGCMWithNonceSize(block, nonceSize)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	cipherData := aesGcm.Seal(nonce, nonce, data, nil)
	if len(cipherData) == 0 {
		return nil, errors.New("aes seal failed to generate encrypted data")
	}

	return cipherData, nil
}

// DecryptWithAes decrypts bytes based on an Aes compatible hashed key.
func DecryptWithAes(cipherDataWithNonce, hashedKey []byte, nonceSize int) ([]byte, error) {

	if len(cipherDataWithNonce) == 0 || len(hashedKey) == 0 || len(cipherDataWithNonce) <= nonceSize {
		return nil, errors.New("cipherDataWithNonce or hash can't be zero length or cipherDataWithNonce can't be the same size as nonce")
	}

	block, err := aes.NewCipher(hashedKey)
	if err != nil {
		return nil, err
	}

	aesGcm, err := cipher.NewGCMWithNonceSize(block, nonceSize)
	if err != nil {
		return nil, err
	}

	return aesGcm.Open(nil, cipherDataWithNonce[:nonceSize], cipherDataWithNonce[nonceSize:], nil)
}
