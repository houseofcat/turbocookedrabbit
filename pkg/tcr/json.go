package tcr

import (
	"bytes"
	"io/ioutil"
	"time"

	jsoniter "github.com/json-iterator/go"
)

const (
	gzipCompressionType = "gzip"
	zstdCompressionType = "zstd"

	aesSymmetricType = "aes"
)

// ConvertJSONFileToConfig opens a file.json and converts to RabbitSeasoning.
func ConvertJSONFileToConfig(fileNamePath string) (*RabbitSeasoning, error) {

	byteValue, err := ioutil.ReadFile(fileNamePath)
	if err != nil {
		return nil, err
	}

	config := &RabbitSeasoning{}
	var json = jsoniter.ConfigFastest
	err = json.Unmarshal(byteValue, config)

	return config, err
}

// ConvertJSONFileToTopologyConfig opens a file.json and converts to Topology.
func ConvertJSONFileToTopologyConfig(fileNamePath string) (*TopologyConfig, error) {

	byteValue, err := ioutil.ReadFile(fileNamePath)
	if err != nil {
		return nil, err
	}

	config := &TopologyConfig{}
	var json = jsoniter.ConfigFastest
	err = json.Unmarshal(byteValue, config)

	return config, err
}

// ReadJSONFileToInterface opens a file.json and converts to interface{}.
func ReadJSONFileToInterface(fileNamePath string) (interface{}, error) {

	byteValue, err := ioutil.ReadFile(fileNamePath)
	if err != nil {
		return nil, err
	}

	var data interface{}
	var json = jsoniter.ConfigFastest
	err = json.Unmarshal(byteValue, data)

	return &data, err
}

// CreatePayload creates a JSON marshal and optionally compresses and encrypts the bytes.
func CreatePayload(
	input interface{},
	compression *CompressionConfig,
	encryption *EncryptionConfig) ([]byte, error) {

	var json = jsoniter.ConfigFastest
	data, err := json.Marshal(&input)
	if err != nil {
		return nil, err
	}

	buffer := &bytes.Buffer{}
	if compression.Enabled {
		err := handleCompression(compression, data, buffer)
		if err != nil {
			return nil, err
		}

		// Update data - data is now compressed
		data = buffer.Bytes()
	}

	if encryption.Enabled {
		err := handleEncryption(encryption, data, buffer)
		if err != nil {
			return nil, err
		}

		// Update data - data is now encrypted
		data = buffer.Bytes()
	}

	return data, nil
}

// CreateWrappedPayload wraps your data in a plaintext wrapper called ModdedLetter and performs the selected modifications to data.
func CreateWrappedPayload(
	input interface{},
	letterID uint64,
	metadata string,
	compression *CompressionConfig,
	encryption *EncryptionConfig) ([]byte, error) {

	moddedLetter := &ModdedLetter{
		LetterID:       letterID,
		LetterMetadata: metadata,
		Body:           &ModdedBody{},
	}

	var json = jsoniter.ConfigFastest
	var err error
	var innerData []byte
	innerData, err = json.Marshal(&input)
	if err != nil {
		return nil, err
	}

	buffer := &bytes.Buffer{}
	if compression.Enabled {
		err := handleCompression(compression, innerData, buffer)
		if err != nil {
			return nil, err
		}

		// Data is now compressed
		moddedLetter.Body.Compressed = true
		moddedLetter.Body.CType = compression.Type
		innerData = buffer.Bytes()
	}

	if encryption.Enabled {
		err := handleEncryption(encryption, innerData, buffer)
		if err != nil {
			return nil, err
		}

		// Data is now encrypted
		moddedLetter.Body.Encrypted = true
		moddedLetter.Body.EType = encryption.Type
		innerData = buffer.Bytes()
	}

	moddedLetter.Body.UTCDateTime = time.Now().UTC().Format(time.RFC3339)
	moddedLetter.Body.Data = innerData

	data, err := json.Marshal(&moddedLetter)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func handleCompression(compression *CompressionConfig, data []byte, buffer *bytes.Buffer) error {

	switch compression.Type {
	case zstdCompressionType:
		return CompressWithZstd(data, buffer)
	case gzipCompressionType:
		fallthrough
	default:
		return CompressWithGzip(data, buffer)
	}
}

func handleEncryption(encryption *EncryptionConfig, data []byte, buffer *bytes.Buffer) error {

	switch encryption.Type {
	case aesSymmetricType:
		fallthrough
	default:
		data, err := EncryptWithAes(data, encryption.Hashkey, 12)

		if err != nil {
			return err
		}

		*buffer = *bytes.NewBuffer(data)

		return nil
	}
}

// ReadPayload unencrypts and uncompresses payloads
func ReadPayload(buffer *bytes.Buffer, compression *CompressionConfig, encryption *EncryptionConfig) error {

	if encryption.Enabled {
		if err := handleDecryption(encryption, buffer); err != nil {
			return err
		}
	}

	if compression.Enabled {
		if err := handleDecompression(compression, buffer); err != nil {
			return err
		}
	}

	return nil
}

func handleDecompression(compression *CompressionConfig, buffer *bytes.Buffer) error {

	switch compression.Type {
	case zstdCompressionType:
		return DecompressWithZstd(buffer)
	case gzipCompressionType:
		fallthrough
	default:
		return DecompressWithGzip(buffer)
	}
}

func handleDecryption(encryption *EncryptionConfig, buffer *bytes.Buffer) error {

	switch encryption.Type {
	case aesSymmetricType:
		fallthrough
	default:
		data, err := DecryptWithAes(buffer.Bytes(), encryption.Hashkey, 12)

		if err != nil {
			return err
		}

		*buffer = *bytes.NewBuffer(data)

		return nil
	}
}
