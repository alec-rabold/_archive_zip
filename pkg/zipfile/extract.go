package zipfile

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"strings"

	"github.com/alec-rabold/zipspy/pkg/aws"
	"github.com/alec-rabold/zipspy/pkg/reader"
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
	fileMap map[string][]*File
}

// ExtractFilesOutput is the response objection from calling Extract()
type ExtractFilesOutput struct {
	FileMap map[string][]*File
}

// File represents a decompressed, extracted file
type File struct {
	reader.FileHeader
	Contents bytes.Buffer
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
	x.fileMap = make(map[string][]*File)
}

// ExtractFiles retrieves the desired files from S3 (compressed), then
// returns a slice a decompressed File objevts
func (x *FileExtractor) ExtractFiles(files []string) (*ExtractFilesOutput, error) {
	dir, err := x.getEOCDRecord()
	if err != nil {
		return nil, err
	}

	x.DirectoryEnd = dir
	zFiles, err := x.getLocalDirectoryFiles()
	if err != nil {
		return nil, err
	}

	_, err = x.extractAndDecompressFiles(zFiles, files)
	if err != nil {
		return nil, err
	}

	return &ExtractFilesOutput{x.fileMap}, nil
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
		if str := contains(filesToExtract, file.Name); str != nil {
			// Get S3 object w/ range, then decode
			rangeEnd := int64(math.Min(float64(file.HeaderOffset+int64(file.CompressedSize64)+200), float64(x.size)))
			byteRange := fmt.Sprintf("bytes=%v-%v", file.HeaderOffset, rangeEnd)
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

			if _, ok := x.fileMap[*str]; !ok {
				x.fileMap[*str] = make([]*File, 0)
			}

			x.fileMap[*str] = append(x.fileMap[*str], &File{
				FileHeader: file.FileHeader,
				Contents:   buf,
			})
		}
	}
	return files, nil
}

// checks if a string (e) contains any substrings of those in a slice (s)
// returns the matched string from slice (s)[n]
func contains(s []string, e string) *string {
	for _, a := range s {
		if strings.Contains(e, a) {
			return &a
		}
	}
	return nil
}
