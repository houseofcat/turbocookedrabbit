package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"

	"golang.org/x/crypto/argon2"
)

const (
	defaultNonceSize = 12 // 12 is the standard
)

// HashWithArgon uses Argon2 to hash a plaintext password with a provided salt string.
func HashWithArgon(password string, salt string, timeConsideration uint32, threads uint8) []byte {

	if password == "" || salt == "" {
		return nil
	}

	if timeConsideration == 0 {
		timeConsideration = 1
	}

	if threads == 0 {
		threads = 1
	}

	return argon2.IDKey([]byte(password), []byte(salt), timeConsideration, 64*1024, threads, 32)
}

// EncryptWithAes encrypts bytes based on an Aes compatible hashed key.
// If nonceSize is less than 12, the standard, 12, is used.
func EncryptWithAes(data, hashedKey []byte, nonceSize int) ([]byte, error) {

	if len(data) == 0 || len(hashedKey) == 0 {
		return nil, errors.New("data or hash can't be zero length")
	}

	if nonceSize < 12 {
		nonceSize = defaultNonceSize
	}

	block, err := aes.NewCipher(hashedKey)
	if err != nil {
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

	nonce, cipherData := cipherDataWithNonce[:nonceSize], cipherDataWithNonce[nonceSize:]
	data, err := aesGcm.Open(nil, nonce, cipherData, nil)
	if err != nil {
		return nil, err
	}

	return data, nil
}
