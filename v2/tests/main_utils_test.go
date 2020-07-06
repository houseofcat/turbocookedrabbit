package main_test

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/houseofcat/turbocookedrabbit/v2/pkg/tcr"
	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/assert"
)

func TestCompressAndDecompressWithGzip(t *testing.T) {

	data := "SuperStreetFighter2TurboMBisonDidNothingWrong"
	buffer := &bytes.Buffer{}

	err := tcr.CompressWithGzip([]byte(data), buffer)
	assert.NoError(t, err)

	assert.NotEqual(t, nil, buffer)
	assert.NotEqual(t, 0, buffer.Len())

	err = tcr.DecompressWithGzip(buffer)
	assert.NoError(t, err)
	assert.NotEqual(t, nil, buffer)
	assert.Equal(t, data, buffer.String())
}

func TestCompressAndDecompressWithZstd(t *testing.T) {

	data := "SuperStreetFighter2TurboMBisonDidNothingWrong"
	buffer := &bytes.Buffer{}

	err := tcr.CompressWithZstd([]byte(data), buffer)
	assert.NoError(t, err)

	assert.NotEqual(t, nil, buffer)
	assert.NotEqual(t, 0, buffer.Len())

	err = tcr.DecompressWithZstd(buffer)
	assert.NoError(t, err)
	assert.NotEqual(t, nil, buffer)
	assert.Equal(t, data, buffer.String())
}

func TestGetHashWithArgon2(t *testing.T) {

	password := "SuperStreetFighter2Turbo"
	salt := "MBisonDidNothingWrong"

	hashy := tcr.GetHashWithArgon(password, salt, 1, 12, 64, 64)
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
		tcr.GetHashWithArgon(fmt.Sprintf(password+"-%d", i), salt, 1, 12, 64, 32)
	}
}

func TestGetHashStringWithArgon2(t *testing.T) {

	password := "SuperStreetFighter2Turbo"
	salt := "MBisonDidNothingWrong"

	base64Hash := tcr.GetStringHashWithArgon(password, salt, 1, 12, 64)
	assert.NotNil(t, base64Hash)

	t.Logf("Password: %s\tLength: %d\r\n", password, len(password))
	t.Logf("Hash As Base64: %s\tLength: %d\r\n", base64Hash, len(base64Hash))
}

func TestHashAndAesEncrypt(t *testing.T) {

	dataPayload := []byte("\x68\x65\x6c\x6c\x6f\x20\x77\x6f\x72\x6c\x64")
	password := "SuperStreetFighter2Turbo"
	salt := "MBisonDidNothingWrong"

	hashy := tcr.GetHashWithArgon(password, salt, 1, 12, 64, 32)
	assert.NotNil(t, hashy)

	base64Hash := make([]byte, base64.StdEncoding.EncodedLen(len(hashy)))
	base64.StdEncoding.Encode(base64Hash, hashy)

	t.Logf("Password: %s\tLength: %d\r\n", password, len(password))
	t.Logf("Password Hashed As Base64: %s\tLength: %d\r\n", base64Hash, len(base64Hash))

	encrypted, err := tcr.EncryptWithAes(dataPayload, hashy, 0)
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

	hashy := tcr.GetHashWithArgon(password, salt, 1, 12, 64, 32)
	assert.NotNil(t, hashy)

	base64Hash := make([]byte, base64.StdEncoding.EncodedLen(len(hashy)))
	base64.StdEncoding.Encode(base64Hash, hashy)

	t.Logf("Password: %s\tLength: %d\r\n", password, len(password))
	t.Logf("Password Hashed As Base64: %s\tLength: %d\r\n", base64Hash, len(base64Hash))

	encryptedPayload, err := tcr.EncryptWithAes(dataPayload, hashy, 12)
	assert.NoError(t, err)

	base64Encrypted := make([]byte, base64.StdEncoding.EncodedLen(len(encryptedPayload)))
	base64.StdEncoding.Encode(base64Encrypted, encryptedPayload)

	t.Logf("Original Payload: %s\r\n", string(dataPayload))
	t.Logf("Encrypted As Base64: %s\tLength: %d\r\n", base64Encrypted, len(base64Encrypted))

	decryptedPayload, err := tcr.DecryptWithAes(encryptedPayload, hashy, 12)
	assert.NoError(t, err)
	t.Logf("Decrypted Payload: %s\r\n", string(decryptedPayload))

	assert.Equal(t, dataPayload, decryptedPayload)
}

type TestStruct struct {
	PropertyString1 string `json:"PropertyString1"`
	PropertyString2 string `json:"PropertyString2"`
	PropertyString3 string `json:"PropertyString3"`
	PropertyString4 string `json:"PropertyString4"`
}

func TestCreateAndReadCompressedPayload(t *testing.T) {

	password := "SuperStreetFighter2Turbo"
	salt := "MBisonDidNothingWrong"

	hashy := tcr.GetHashWithArgon(password, salt, 1, 12, 64, 32)

	encrypt := &tcr.EncryptionConfig{
		Enabled:           false,
		Hashkey:           hashy,
		Type:              tcr.AesSymmetricType,
		TimeConsideration: 1,
		Threads:           6,
	}

	compression := &tcr.CompressionConfig{
		Enabled: true,
		Type:    tcr.GzipCompressionType,
	}

	test := &TestStruct{
		PropertyString1: tcr.RandomString(5000),
		PropertyString2: tcr.RandomString(5000),
		PropertyString3: tcr.RandomString(5000),
		PropertyString4: tcr.RandomString(5000),
	}

	t.Logf("Test Data Size: ~ %d letters", 20000)
	data, err := tcr.CreatePayload(test, compression, encrypt)
	assert.NoError(t, err)
	assert.NotEqual(t, 0, len(data))
	t.Logf("Compressed Payload Size: ~ %d bytes", len(data))

	buffer := bytes.NewBuffer(data)
	err = tcr.ReadPayload(buffer, compression, encrypt)
	assert.NoError(t, err)
	assert.NotEqual(t, 0, buffer.Len())

	var json = jsoniter.ConfigFastest
	outputData := &TestStruct{}
	err = json.Unmarshal(buffer.Bytes(), outputData)
	assert.NoError(t, err)
	assert.Equal(t, test.PropertyString1, outputData.PropertyString1)
	assert.Equal(t, test.PropertyString2, outputData.PropertyString2)
	assert.Equal(t, test.PropertyString3, outputData.PropertyString3)
	assert.Equal(t, test.PropertyString4, outputData.PropertyString4)
}

func TestCreateAndReadEncryptedPayload(t *testing.T) {

	password := "SuperStreetFighter2Turbo"
	salt := "MBisonDidNothingWrong"

	hashy := tcr.GetHashWithArgon(password, salt, 1, 12, 64, 32)

	encrypt := &tcr.EncryptionConfig{
		Enabled:           true,
		Hashkey:           hashy,
		Type:              tcr.AesSymmetricType,
		TimeConsideration: 1,
		Threads:           6,
	}

	compression := &tcr.CompressionConfig{
		Enabled: false,
		Type:    tcr.GzipCompressionType,
	}

	test := &TestStruct{
		PropertyString1: tcr.RandomString(5000),
		PropertyString2: tcr.RandomString(5000),
		PropertyString3: tcr.RandomString(5000),
		PropertyString4: tcr.RandomString(5000),
	}

	t.Logf("Test Data Size: ~ %d letters", 20000)
	data, err := tcr.CreatePayload(test, compression, encrypt)
	assert.NoError(t, err)
	assert.NotEqual(t, 0, len(data))
	t.Logf("Encrypted Payload Size: ~ %d bytes", len(data))

	buffer := bytes.NewBuffer(data)
	err = tcr.ReadPayload(buffer, compression, encrypt)
	assert.NoError(t, err)
	assert.NotEqual(t, 0, buffer.Len())

	var json = jsoniter.ConfigFastest
	outputData := &TestStruct{}
	err = json.Unmarshal(buffer.Bytes(), outputData)
	assert.NoError(t, err)
	assert.Equal(t, test.PropertyString1, outputData.PropertyString1)
	assert.Equal(t, test.PropertyString2, outputData.PropertyString2)
	assert.Equal(t, test.PropertyString3, outputData.PropertyString3)
	assert.Equal(t, test.PropertyString4, outputData.PropertyString4)
}

func TestCreateAndReadCompressedEncryptedPayload(t *testing.T) {

	password := "SuperStreetFighter2Turbo"
	salt := "MBisonDidNothingWrong"

	hashy := tcr.GetHashWithArgon(password, salt, 1, 12, 64, 32)

	encrypt := &tcr.EncryptionConfig{
		Enabled:           true,
		Hashkey:           hashy,
		Type:              tcr.AesSymmetricType,
		TimeConsideration: 1,
		Threads:           6,
	}

	compression := &tcr.CompressionConfig{
		Enabled: true,
		Type:    tcr.GzipCompressionType,
	}

	test := &TestStruct{
		PropertyString1: tcr.RandomString(5000),
		PropertyString2: tcr.RandomString(5000),
		PropertyString3: tcr.RandomString(5000),
		PropertyString4: tcr.RandomString(5000),
	}

	t.Logf("Test Data Size: ~ %d letters", 20000)
	data, err := tcr.CreatePayload(test, compression, encrypt)
	assert.NoError(t, err)
	assert.NotEqual(t, 0, len(data))
	t.Logf("Compressed & Encrypted Payload Size: ~ %d bytes", len(data))

	buffer := bytes.NewBuffer(data)
	err = tcr.ReadPayload(buffer, compression, encrypt)
	assert.NoError(t, err)
	assert.NotEqual(t, 0, buffer.Len())

	var json = jsoniter.ConfigFastest
	outputData := &TestStruct{}
	err = json.Unmarshal(buffer.Bytes(), outputData)
	assert.NoError(t, err)
	assert.Equal(t, test.PropertyString1, outputData.PropertyString1)
	assert.Equal(t, test.PropertyString2, outputData.PropertyString2)
	assert.Equal(t, test.PropertyString3, outputData.PropertyString3)
	assert.Equal(t, test.PropertyString4, outputData.PropertyString4)
}

func TestCreateAndReadLZCompressedEncryptedPayload(t *testing.T) {

	password := "SuperStreetFighter2Turbo"
	salt := "MBisonDidNothingWrong"

	hashy := tcr.GetHashWithArgon(password, salt, 1, 12, 64, 32)

	encrypt := &tcr.EncryptionConfig{
		Enabled:           true,
		Hashkey:           hashy,
		Type:              tcr.AesSymmetricType,
		TimeConsideration: 1,
		Threads:           6,
	}

	compression := &tcr.CompressionConfig{
		Enabled: true,
		Type:    tcr.ZstdCompressionType,
	}

	test := &TestStruct{
		PropertyString1: tcr.RandomString(5000),
		PropertyString2: tcr.RandomString(5000),
		PropertyString3: tcr.RandomString(5000),
		PropertyString4: tcr.RandomString(5000),
	}

	t.Logf("Test Data Size: ~ %d letters", 20000)
	data, err := tcr.CreatePayload(test, compression, encrypt)
	assert.NoError(t, err)
	assert.NotEqual(t, 0, len(data))
	t.Logf("Compressed & Encrypted Payload Size: ~ %d bytes", len(data))

	buffer := bytes.NewBuffer(data)
	err = tcr.ReadPayload(buffer, compression, encrypt)
	assert.NoError(t, err)
	assert.NotEqual(t, 0, buffer.Len())

	var json = jsoniter.ConfigFastest
	outputData := &TestStruct{}
	err = json.Unmarshal(buffer.Bytes(), outputData)
	assert.NoError(t, err)
	assert.Equal(t, test.PropertyString1, outputData.PropertyString1)
	assert.Equal(t, test.PropertyString2, outputData.PropertyString2)
	assert.Equal(t, test.PropertyString3, outputData.PropertyString3)
	assert.Equal(t, test.PropertyString4, outputData.PropertyString4)
}

func TestRandomString(t *testing.T) {

	randoString := tcr.RandomString(20)
	assert.NotEqual(t, "", randoString)
	t.Logf("RandoString1: %s", randoString)

	time.Sleep(1 * time.Nanosecond)

	anotherRandoString := tcr.RandomString(20)
	assert.NotEqual(t, "", anotherRandoString)
	t.Logf("RandoString2: %s", anotherRandoString)

	assert.NotEqual(t, randoString, anotherRandoString)
}

func TestRandomStringFromSource(t *testing.T) {

	src := rand.NewSource(time.Now().UnixNano())

	randoString := tcr.RandomStringFromSource(10, src)
	assert.NotEqual(t, "", randoString)
	t.Logf("RandoString1: %s", randoString)

	anotherRandoString := tcr.RandomStringFromSource(10, src)
	assert.NotEqual(t, "", anotherRandoString)
	t.Logf("RandoString2: %s", anotherRandoString)

	assert.NotEqual(t, randoString, anotherRandoString)
}
