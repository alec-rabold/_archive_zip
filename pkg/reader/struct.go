package reader

import (
	"errors"
)

var (
	// ErrFormat indicates the file's format not conforming to zip specification
	ErrFormat = errors.New("zip: unable to locate end of central directory")
	// ErrCommentLength indicates an invalid comment length
	ErrCommentLength = errors.New("zip: invalid comment length")
	// ErrAlgorithm indicates an invalid/unsupported compression algorithm
	ErrAlgorithm = errors.New("zip: unsupported compression algorithm")
)

const (
	directoryEndLen    = 22
	directoryHeaderLen = 46
	fileHeaderLen      = 30 // + filename + extra

	dataDescriptorSignature  = 0x08074b50
	directoryEndSignature    = 0x06054b50
	directoryHeaderSignature = 0x02014b50
	fileHeaderSignature      = 0x04034b50

	zip64ExtraID = 0x0001 // Zip64 extended information
)

// Compression methods.
const (
	Store   uint16 = 0 // no compression
	Deflate uint16 = 8 // DEFLATE compressed
)

// FileHeader describes a file within a zip file.
// See the zip spec for details.
type FileHeader struct {
	// Name is the name of the file.
	//
	// It must be a relative path, not start with a drive letter (such as "C:"),
	// and must use forward slashes instead of back slashes. A trailing slash
	// indicates that this file is a directory and should have no data.
	Name string

	// Comment is any arbitrary user-defined string shorter than 64KiB.
	Comment string

	Flags uint16

	// Method is the compression method. If zero, Store is used.
	Method uint16

	CompressedSize     uint32 // Deprecated: Use CompressedSize64 instead.
	UncompressedSize   uint32 // Deprecated: Use UncompressedSize64 instead.
	CompressedSize64   uint64
	UncompressedSize64 uint64
	Extra              []byte
}

// DirectoryEnd descrives an EOCD record
type DirectoryEnd struct {
	directoryRecords   uint64
	directorySize      uint64
	DirectoryOffset    uint64 // relative to file
	DirectoryEndOffset uint64
	commentLen         uint16
	comment            string
}
