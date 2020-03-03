package zipfile

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"github.com/alec-rabold/zipspy/pkg/aws"
	"github.com/alec-rabold/zipspy/pkg/reader"
	"io"
	"io/ioutil"
	"strings"

	log "github.com/sirupsen/logrus"
)

// FileExtractor extracts & decompresses files from a zip archive in S3
type FileExtractor struct {
	aws    *aws.Client
	ctx    context.Context
	bucket string
	key    string
	size   int64
	reader.DirectoryEnd
}

// File represents a decompressed, extracted file
type File struct {
	reader.FileHeader
	contents bytes.Buffer
}

// NewFileExtractor creates a new instance of FileExtractor
func NewFileExtractor(bucket, key string) *FileExtractor {
	x := &FileExtractor{
		aws:    aws.NewClient(),
		ctx:    context.Background(),
		bucket: bucket,
		key:    key,
	}
	x.init()
	return x
}

// init sets the extraction metadata
func (x *FileExtractor) init() {
	head := x.aws.GetHeadObject(x.ctx, x.bucket, x.key)
	x.size = *head.ContentLength
}

// ExtractFiles retrieves the desired files from S3 (compressed), then
// returns a slice a decompressed File objevts
func (x *FileExtractor) ExtractFiles(files []string) ([]*File, error) {
	dir, err := x.getEOCDRecord()
	if err != nil {
		return nil, err
	}
	x.DirectoryEnd = dir
	zFiles, err := x.getLocalDirectoryFiles()
	if err != nil {
		return nil, err
	}

	x.extractAndDecompressFiles(zFiles, files)
	return nil, nil
}

// EOCDR stands for End of Central Directory\
func (x *FileExtractor) getEOCDRecord() (reader.DirectoryEnd, error) {
	var dir *reader.DirectoryEnd
	// look for directoryEndSignature in the last 1k, then in the last 65k
	for i, bLen := range []int64{1024, 65 * 1024} {
		if bLen > x.size {
			bLen = x.size
		}
		byteRange := fmt.Sprintf("bytes=%v-%v", x.size-bLen, x.size)
		response := x.aws.GetS3ObjectWithRange(x.ctx, x.bucket, x.key, byteRange)
		bodyBytes, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return reader.DirectoryEnd{}, err
		}
		r := bytes.NewReader(bodyBytes)
		dir, err = reader.ReadDirectoryEnd(r, bLen, x.size)
		if dir != nil {
			break
		}
		if i == 1 || bLen == x.size {
			return reader.DirectoryEnd{}, reader.ErrFormat
		}

	}
	return *dir, nil
}

func (x *FileExtractor) getLocalDirectoryFiles() ([]*reader.File, error) {
	var zFiles []*reader.File
	byteRange := fmt.Sprintf("bytes=%v-%v", x.DirectoryOffset, x.DirectoryEndOffset)
	response := x.aws.GetS3ObjectWithRange(x.ctx, x.bucket, x.key, byteRange)
	bodyBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Error(err)
	}

	r := bytes.NewReader(bodyBytes)
	buf := bufio.NewReader(r)
	// The count of files inside a zip is truncated to fit in a uint16.
	// Gloss over this by reading headers until we encounter
	// a bad one, and then only report an ErrFormat or UnexpectedEOF if
	// the file count modulo 65536 is incorrect
	for {
		f := &reader.File{
			Zip:     new(reader.Reader),
			Zipr:    r,
			Zipsize: *response.ContentLength}
		err = reader.ReadDirectoryHeader(f, buf)
		if err == reader.ErrFormat || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return nil, err
		}
		zFiles = append(zFiles, f)
	}
	return zFiles, nil
}

func (x *FileExtractor) extractAndDecompressFiles(zFiles []*reader.File, filesToExtract []string) ([]*File, error) {
	var files []*File
	for _, file := range zFiles {
		if contains(filesToExtract, file.Name) {
			// Get S3 object w/ range, then decode
			byteRange := fmt.Sprintf("bytes=%v-%v", file.HeaderOffset, file.HeaderOffset+int64(file.CompressedSize64)+100)
			response := x.aws.GetS3ObjectWithRange(x.ctx, x.bucket, x.key, byteRange)
			bodyBytes, err := ioutil.ReadAll(response.Body)
			if err != nil {
				return nil, err
			}
			fileReader := bytes.NewReader(bodyBytes)
			file.Zipr = fileReader

			rc, err := file.Open()
			if err != nil {
				return nil, err
			}
			var buf bytes.Buffer
			_, err = io.Copy(&buf, rc)
			if err != nil {
				return nil, err
			}
			rc.Close()

			files = append(files, &File{
				FileHeader: file.FileHeader,
				contents:   buf,
			})

			fmt.Println(buf.String())
		}
	}
	return files, nil
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if strings.Contains(e, a) {
			return true
		}
	}
	return false
}
