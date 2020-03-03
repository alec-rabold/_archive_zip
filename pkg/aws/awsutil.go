package aws

import (
	"context"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	log "github.com/sirupsen/logrus"
)

// Client is an abstraction layer for interacting with AWS services.
type Client struct {
	s3 s3.S3
}

// NewClient creates a new AWS client, expecting that the environment variables configure the settings.
func NewClient() *Client {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	return &Client{
		s3: *s3.New(sess),
	}
}

// GetHeadObject implements the AWS interface
func (c *Client) GetHeadObject(ctx context.Context, bucket, key string) *s3.HeadObjectOutput {
	output, err := c.s3.HeadObjectWithContext(ctx, &s3.HeadObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		log.Errorf("error getting S3 head object (bucket: %s)(key: %s), err: %v", bucket, key, err)
	}
	return output
}

// GetS3ObjectWithRange implements the AWS interface
func (c *Client) GetS3ObjectWithRange(ctx context.Context, bucket, key, byteRange string) *s3.GetObjectOutput {
	output, err := c.s3.GetObjectWithContext(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
		Range:  &byteRange,
	})
	if err != nil {
		log.Errorf("error getting S3 object (bucket: %s)(key: %s)(range: %s), err: %v", bucket, key, byteRange, err)
	}
	return output
}
