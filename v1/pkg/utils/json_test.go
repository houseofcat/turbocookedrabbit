package utils

import (
	"bytes"
	"testing"

	"github.com/houseofcat/turbocookedrabbit/v1/pkg/models"
	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/assert"
)

type TestStruct struct {
	PropertyString1 string `json:"PropertyString1"`
	PropertyString2 string `json:"PropertyString2"`
	PropertyString3 string `json:"PropertyString3"`
	PropertyString4 string `json:"PropertyString4"`
}

func TestCreateAndReadCompressedPayload(t *testing.T) {

	password := "SuperStreetFighter2Turbo"
	salt := "MBisonDidNothingWrong"

	hashy := GetHashWithArgon(password, salt, 1, 12, 64, 32)

	encrypt := &models.EncryptionConfig{
		Enabled:           false,
		Hashkey:           hashy,
		Type:              aesSymmetricType,
		TimeConsideration: 1,
		Threads:           6,
	}

	compression := &models.CompressionConfig{
		Enabled: true,
		Type:    gzipCompressionType,
	}

	test := &TestStruct{
		PropertyString1: RandomString(5000),
		PropertyString2: RandomString(5000),
		PropertyString3: RandomString(5000),
		PropertyString4: RandomString(5000),
	}

	t.Logf("Test Data Size: ~ %d letters", 20000)
	data, err := CreatePayload(test, compression, encrypt)
	assert.NoError(t, err)
	assert.NotEqual(t, 0, len(data))
	t.Logf("Compressed Payload Size: ~ %d bytes", len(data))

	buffer := bytes.NewBuffer(data)
	err = ReadPayload(buffer, compression, encrypt)
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

	hashy := GetHashWithArgon(password, salt, 1, 12, 64, 32)

	encrypt := &models.EncryptionConfig{
		Enabled:           true,
		Hashkey:           hashy,
		Type:              aesSymmetricType,
		TimeConsideration: 1,
		Threads:           6,
	}

	compression := &models.CompressionConfig{
		Enabled: false,
		Type:    gzipCompressionType,
	}

	test := &TestStruct{
		PropertyString1: RandomString(5000),
		PropertyString2: RandomString(5000),
		PropertyString3: RandomString(5000),
		PropertyString4: RandomString(5000),
	}

	t.Logf("Test Data Size: ~ %d letters", 20000)
	data, err := CreatePayload(test, compression, encrypt)
	assert.NoError(t, err)
	assert.NotEqual(t, 0, len(data))
	t.Logf("Encrypted Payload Size: ~ %d bytes", len(data))

	buffer := bytes.NewBuffer(data)
	err = ReadPayload(buffer, compression, encrypt)
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

	hashy := GetHashWithArgon(password, salt, 1, 12, 64, 32)

	encrypt := &models.EncryptionConfig{
		Enabled:           true,
		Hashkey:           hashy,
		Type:              aesSymmetricType,
		TimeConsideration: 1,
		Threads:           6,
	}

	compression := &models.CompressionConfig{
		Enabled: true,
		Type:    gzipCompressionType,
	}

	test := &TestStruct{
		PropertyString1: RandomString(5000),
		PropertyString2: RandomString(5000),
		PropertyString3: RandomString(5000),
		PropertyString4: RandomString(5000),
	}

	t.Logf("Test Data Size: ~ %d letters", 20000)
	data, err := CreatePayload(test, compression, encrypt)
	assert.NoError(t, err)
	assert.NotEqual(t, 0, len(data))
	t.Logf("Compressed & Encrypted Payload Size: ~ %d bytes", len(data))

	buffer := bytes.NewBuffer(data)
	err = ReadPayload(buffer, compression, encrypt)
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

	hashy := GetHashWithArgon(password, salt, 1, 12, 64, 32)

	encrypt := &models.EncryptionConfig{
		Enabled:           true,
		Hashkey:           hashy,
		Type:              aesSymmetricType,
		TimeConsideration: 1,
		Threads:           6,
	}

	compression := &models.CompressionConfig{
		Enabled: true,
		Type:    zstdCompressionType,
	}

	test := &TestStruct{
		PropertyString1: RandomString(5000),
		PropertyString2: RandomString(5000),
		PropertyString3: RandomString(5000),
		PropertyString4: RandomString(5000),
	}

	t.Logf("Test Data Size: ~ %d letters", 20000)
	data, err := CreatePayload(test, compression, encrypt)
	assert.NoError(t, err)
	assert.NotEqual(t, 0, len(data))
	t.Logf("Compressed & Encrypted Payload Size: ~ %d bytes", len(data))

	buffer := bytes.NewBuffer(data)
	err = ReadPayload(buffer, compression, encrypt)
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
