package reader

import (
	"encoding/binary"
	"io"
)

type Reader struct {
	r             io.ReaderAt
	File          []*File
	Comment       string
	decompressors map[uint16]Decompressor
}

type File struct {
	FileHeader
	Zip          *Reader
	Zipr         io.ReaderAt
	Zipsize      int64
	HeaderOffset int64
}

type FileReader struct {
	rc    io.ReadCloser
	nread uint64 // number of bytes read so far
	f     *File
	err   error // sticky error
}

// Open returns a ReadCloser that provides access to the File's contents.
// Multiple files may be read concurrently.
func (f *File) Open() (io.ReadCloser, error) {
	bodyOffset, err := f.findBodyOffset()
	if err != nil {
		return nil, err
	}
	size := int64(f.CompressedSize64)
	r := io.NewSectionReader(f.Zipr, bodyOffset, size)
	dcomp := f.Zip.decompressor(f.Method)
	if dcomp == nil {
		return nil, ErrAlgorithm
	}
	var rc io.ReadCloser = dcomp(r)
	rc = &FileReader{
		rc: rc,
		f:  f,
	}
	return rc, nil
}

func (r *FileReader) Read(b []byte) (n int, err error) {
	if r.err != nil {
		return 0, r.err
	}
	n, err = r.rc.Read(b)
	r.nread += uint64(n)
	if err == nil {
		return
	}
	if err == io.EOF {
		if r.nread != r.f.UncompressedSize64 {
			return 0, io.ErrUnexpectedEOF
		}
	}
	r.err = err
	return
}

// Close implements io.ReadCloser
func (r *FileReader) Close() error { return r.rc.Close() }

func ReadDirectoryEnd(r io.ReaderAt, bufSize, totalSize int64) (dir *DirectoryEnd, err error) {
	if bufSize > totalSize {
		bufSize = totalSize
	}
	buf := make([]byte, int(bufSize))
	var dEndOffset int64
	if _, err := r.ReadAt(buf, 0); err != nil && err != io.EOF {
		return nil, err
	}
	if p := findEOCDSignatureInBlock(buf); p >= 0 {
		buf = buf[p:]
		dEndOffset = totalSize - bufSize + int64(p)
	} else {
		return nil, ErrFormat
	}

	b := readBuf(buf[10:]) // skip signature & unncessary fields
	d := &DirectoryEnd{
		directoryRecords:   uint64(b.uint16()),
		directorySize:      uint64(b.uint32()),
		DirectoryOffset:    uint64(b.uint32()),
		DirectoryEndOffset: uint64(dEndOffset),
		commentLen:         b.uint16(),
	}

	l := int(d.commentLen)
	if l > len(b) {
		return nil, ErrCommentLength
	}

	// Make sure directoryOffset points to somewhere in our file.
	if o := int64(d.DirectoryOffset); o < 0 || o >= totalSize {
		return nil, ErrFormat
	}

	return d, nil
}

func ReadDirectoryHeader(f *File, r io.Reader) error {
	var buf [directoryHeaderLen]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return err
	}
	b := readBuf(buf[:])
	if sig := b.uint32(); sig != directoryHeaderSignature {
		return ErrFormat
	}

	f.Flags = b.skip(4).uint16()
	f.Method = b.uint16()
	f.CompressedSize = b.skip(8).uint32()
	f.UncompressedSize = b.uint32()
	f.CompressedSize64 = uint64(f.CompressedSize)
	f.UncompressedSize64 = uint64(f.UncompressedSize)
	filenameLen := int(b.uint16())
	extraLen := int(b.uint16())
	commentLen := int(b.uint16())
	f.HeaderOffset = int64(b.skip(8).uint32())

	d := make([]byte, filenameLen+extraLen+commentLen)
	if _, err := io.ReadFull(r, d); err != nil {
		return err
	}
	f.Name = string(d[:filenameLen])
	f.Extra = d[filenameLen : filenameLen+extraLen]
	f.Comment = string(d[filenameLen+extraLen:])

	needUSize := f.UncompressedSize == ^uint32(0)
	needCSize := f.CompressedSize == ^uint32(0)
	needHeaderOffset := f.HeaderOffset == int64(^uint32(0))

	for extra := readBuf(f.Extra); len(extra) >= 4; {
		fieldTag := extra.uint16()
		fieldSize := int(extra.uint16())
		if len(extra) < fieldSize {
			break
		}
		fieldBuf := extra.sub(fieldSize)

		switch fieldTag {
		case zip64ExtraID:
			// update directory values from the zip64 extra block.
			// They should only be consulted if the sizes read earlier
			// are maxed out.
			if needUSize {
				needUSize = false
				if len(fieldBuf) < 8 {
					return ErrFormat
				}
				f.UncompressedSize64 = fieldBuf.uint64()
			}
			if needCSize {
				needCSize = false
				if len(fieldBuf) < 8 {
					return ErrFormat
				}
				f.CompressedSize64 = fieldBuf.uint64()
			}
			if needHeaderOffset {
				needHeaderOffset = false
				if len(fieldBuf) < 8 {
					return ErrFormat
				}
				f.HeaderOffset = int64(fieldBuf.uint64())
			}
		}
	}
	return nil
}

func findEOCDSignatureInBlock(b []byte) int {
	for i := len(b) - directoryEndLen; i >= 0; i-- {
		if binary.LittleEndian.Uint32(b[i:i+4]) == directoryEndSignature {
			commentLength := int(b[i+directoryEndLen-2]) | int(b[i+directoryEndLen-1])<<8
			if commentLength+directoryEndLen+i <= len(b) {
				return i
			}
		}
	}
	return -1
}

// findBodyOffset does the minimum work to verify the file has a header
// and returns the file body offset.
func (f *File) findBodyOffset() (int64, error) {
	var buf [fileHeaderLen]byte
	if _, err := f.Zipr.ReadAt(buf[:], 0); err != nil {
		return 0, err
	}
	b := readBuf(buf[:])
	if sig := b.uint32(); sig != fileHeaderSignature {
		return 0, ErrFormat
	}
	b = b[22:] // skip over most of the header
	filenameLen := int(b.uint16())
	extraLen := int(b.uint16())
	return int64(fileHeaderLen + filenameLen + extraLen), nil
}

// RegisterDecompressor registers or overrides a custom decompressor for a
// specific method ID. If a decompressor for a given method is not found,
// Reader will default to looking up the decompressor at the package level.
func (z *Reader) RegisterDecompressor(method uint16, dcomp Decompressor) {
	if z.decompressors == nil {
		z.decompressors = make(map[uint16]Decompressor)
	}
	z.decompressors[method] = dcomp
}

func (z *Reader) decompressor(method uint16) Decompressor {
	dcomp := z.decompressors[method]
	if dcomp == nil {
		dcomp = decompressor(method)
	}
	return dcomp
}

type readBuf []byte

func (b *readBuf) uint8() uint8 {
	v := (*b)[0]
	*b = (*b)[1:]
	return v
}

func (b *readBuf) uint16() uint16 {
	v := binary.LittleEndian.Uint16(*b)
	*b = (*b)[2:]
	return v
}

func (b *readBuf) uint32() uint32 {
	v := binary.LittleEndian.Uint32(*b)
	*b = (*b)[4:]
	return v
}

func (b *readBuf) uint64() uint64 {
	v := binary.LittleEndian.Uint64(*b)
	*b = (*b)[8:]
	return v
}

func (b *readBuf) sub(n int) readBuf {
	b2 := (*b)[:n]
	*b = (*b)[n:]
	return b2
}

func (b *readBuf) skip(n int) *readBuf {
	*b = (*b)[n:]
	return b
}
