package tcr

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"

	"github.com/klauspost/compress/zstd"
)

// CompressWithZstd uses an external dependency for Zstd to compress data and places data in the supplied buffer.
func CompressWithZstd(data []byte, buffer *bytes.Buffer) error {

	zstdWriter, err := zstd.NewWriter(buffer)
	if err != nil {
		return err
	}

	_, err = io.Copy(zstdWriter, bytes.NewReader(data))
	if err != nil {

		closeErr := zstdWriter.Close()
		if closeErr != nil {
			return closeErr
		}

		return err
	}

	return zstdWriter.Close()
}

// DecompressWithZstd uses an external dependency for Zstd to decompress data and replaces the supplied buffer with a new buffer with data in it.
func DecompressWithZstd(buffer *bytes.Buffer) error {

	zstdReader, err := zstd.NewReader(buffer)
	if err != nil {
		return err
	}
	defer zstdReader.Close()

	data, err := ioutil.ReadAll(zstdReader)
	if err != nil {
		return err
	}

	*buffer = *bytes.NewBuffer(data)

	return nil
}

// CompressWithGzip uses the standard Gzip Writer to compress data and places data in the supplied buffer.
func CompressWithGzip(data []byte, buffer *bytes.Buffer) error {

	gzipWriter := gzip.NewWriter(buffer)

	_, err := gzipWriter.Write(data)
	if err != nil {
		return err
	}

	if err := gzipWriter.Close(); err != nil {
		return err
	}

	return nil
}

// DecompressWithGzip uses the standard Gzip Reader to decompress data and replaces the supplied buffer with a new buffer with data in it.
func DecompressWithGzip(buffer *bytes.Buffer) error {

	gzipReader, err := gzip.NewReader(buffer)
	if err != nil {
		return err
	}

	data, err := ioutil.ReadAll(gzipReader)
	if err != nil {
		return err
	}

	if err := gzipReader.Close(); err != nil {
		return err
	}

	*buffer = *bytes.NewBuffer(data)

	return nil
}
