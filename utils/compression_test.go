package utils

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompressAndDecompressWithGzip(t *testing.T) {

	data := "SuperStreetFighter2TurboMBisonDidNothingWrong"
	var buffer bytes.Buffer

	err := CompressWithGzip([]byte(data), &buffer)
	assert.NoError(t, err)

	assert.NotEqual(t, nil, buffer)
	assert.NotEqual(t, 0, buffer.Len())

	err = DecompressWithGzip(&buffer)
	assert.NoError(t, err)
	assert.NotEqual(t, nil, buffer)
	assert.Equal(t, data, string(buffer.Bytes()))
}

func TestCompressAndDecompressWithZstd(t *testing.T) {

	data := "SuperStreetFighter2TurboMBisonDidNothingWrong"
	var buffer bytes.Buffer

	err := CompressWithZstd([]byte(data), &buffer)
	assert.NoError(t, err)

	assert.NotEqual(t, nil, buffer)
	assert.NotEqual(t, 0, buffer.Len())

	err = DecompressWithZstd(&buffer)
	assert.NoError(t, err)
	assert.NotEqual(t, nil, buffer)
	assert.Equal(t, data, string(buffer.Bytes()))
}
