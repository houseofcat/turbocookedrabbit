package tcr

import (
	"bytes"
	"io/ioutil"
	"time"

	"github.com/google/uuid"
	jsoniter "github.com/json-iterator/go"
)

const (
	// GzipCompressionType helps identify which compression/decompression to use.
	GzipCompressionType = "gzip"

	// ZstdCompressionType helps identify which compression/decompression to use.
	ZstdCompressionType = "zstd"

	//AesSymmetricType helps identity which encryption/decryption to use.
	AesSymmetricType = "aes"
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

// ReadWrappedBodyFromJSONBytes simply read the bytes as a Letter.
func ReadWrappedBodyFromJSONBytes(data []byte) (*WrappedBody, error) {

	var json = jsoniter.ConfigFastest
	body := &WrappedBody{}
	err := json.Unmarshal(data, body)
	if err != nil {
		return nil, err
	}

	return body, nil
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
	letterID uuid.UUID,
	metadata string,
	compression *CompressionConfig,
	encryption *EncryptionConfig) ([]byte, error) {

	wrappedBody := &WrappedBody{
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
		wrappedBody.Body.Compressed = true
		wrappedBody.Body.CType = compression.Type
		innerData = buffer.Bytes()
	}

	if encryption.Enabled {
		err := handleEncryption(encryption, innerData, buffer)
		if err != nil {
			return nil, err
		}

		// Data is now encrypted
		wrappedBody.Body.Encrypted = true
		wrappedBody.Body.EType = encryption.Type
		innerData = buffer.Bytes()
	}

	wrappedBody.Body.UTCDateTime = time.Now().UTC().Format(time.RFC3339)
	wrappedBody.Body.Data = innerData

	data, err := json.Marshal(&wrappedBody)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func handleCompression(compression *CompressionConfig, data []byte, buffer *bytes.Buffer) error {

	switch compression.Type {
	case ZstdCompressionType:
		return CompressWithZstd(data, buffer)
	case GzipCompressionType:
		fallthrough
	default:
		return CompressWithGzip(data, buffer)
	}
}

func handleEncryption(encryption *EncryptionConfig, data []byte, buffer *bytes.Buffer) error {

	switch encryption.Type {
	case AesSymmetricType:
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

	if encryption != nil && encryption.Enabled {
		if err := handleDecryption(encryption, buffer); err != nil {
			return err
		}
	}

	if compression != nil && compression.Enabled {
		if err := handleDecompression(compression, buffer); err != nil {
			return err
		}
	}

	return nil
}

func handleDecompression(compression *CompressionConfig, buffer *bytes.Buffer) error {

	switch compression.Type {
	case ZstdCompressionType:
		return DecompressWithZstd(buffer)
	case GzipCompressionType:
		fallthrough
	default:
		return DecompressWithGzip(buffer)
	}
}

func handleDecryption(encryption *EncryptionConfig, buffer *bytes.Buffer) error {

	switch encryption.Type {
	case AesSymmetricType:
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
