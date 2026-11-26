package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

type S3Store struct {
	client *s3.Client
	bucket string
}

const objectAddressPrefix = "s3://"

type Object struct {
	Body        io.ReadCloser
	Size        int64
	ContentType string
	ETag        string
}

// MultipartPart describes an uploaded part. Preserve ETag exactly as returned by S3.
// PartNumber is one-based; handlers using zero-based chunk indices must add one.
type MultipartPart struct {
	PartNumber int32  `json:"partNumber"`
	ETag       string `json:"etag"`
	Size       int64  `json:"size"`
}

var (
	ErrNotInitialized = errors.New("S3 is not initialized")
	ErrObjectNotFound = errors.New("S3 object not found")
	store             *S3Store
	storeMu           sync.RWMutex
)

// Init loads configuration without contacting the bucket. Call it at startup.
func Init() error {
	accessKey := os.Getenv("MINIO_ACCESS_KEY_ID")
	secretKey := os.Getenv("MINIO_SECRET_ACCESS_KEY")
	endpoint := os.Getenv("MINIO_ENDPOINT")
	bucket := os.Getenv("MINIO_BUCKET")
	region := os.Getenv("MINIO_REGION")

	if region == "" {
		region = "us-east-1"
	}

	if accessKey == "" || secretKey == "" || endpoint == "" || bucket == "" {
		return errors.New("set MINIO_ACCESS_KEY_ID, MINIO_SECRET_ACCESS_KEY, MINIO_ENDPOINT, and MINIO_BUCKET")
	}
	if !strings.Contains(endpoint, "://") {
		endpoint = "http://" + endpoint
	}
	endpointURL, err := url.Parse(endpoint)
	if err != nil || (endpointURL.Scheme != "https" && endpointURL.Scheme != "http") ||
		endpointURL.Hostname() == "" ||
		endpointURL.User != nil || endpointURL.RawQuery != "" || endpointURL.ForceQuery ||
		endpointURL.Fragment != "" || (endpointURL.Path != "" && endpointURL.Path != "/") {
		return errors.New("MINIO_ENDPOINT must be an HTTP or HTTPS service endpoint without a bucket path, credentials, query, or fragment")
	}

	cfg, err := config.LoadDefaultConfig(
		context.Background(),
		config.WithRegion(region),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				accessKey,
				secretKey,
				"",
			),
		),
	)
	if err != nil {
		return fmt.Errorf("load S3 config: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
		// Use only the checksums required by the S3 API.
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})

	storeMu.Lock()
	store = &S3Store{
		client: client,
		bucket: bucket,
	}
	storeMu.Unlock()

	return nil
}

func Store() *S3Store {
	storeMu.RLock()
	defer storeMu.RUnlock()
	return store
}

// Address returns the location stored in MySQL for an object in the active bucket.
func (s *S3Store) Address(key string) string {
	return objectAddressPrefix + key
}

// ObjectKey resolves a remote address or identifies a legacy filesystem path.
func (s *S3Store) ObjectKey(address string) (key string, remote bool, err error) {
	if key, ok := strings.CutPrefix(address, objectAddressPrefix); ok {
		return key, true, nil
	}
	if strings.Contains(address, "://") {
		return "", true, errors.New("stored object address must be migrated to MinIO")
	}
	return "", false, nil
}

func (s *S3Store) validate(ctx context.Context, key string) error {
	if s == nil || s.client == nil || s.bucket == "" {
		return ErrNotInitialized
	}
	if ctx == nil {
		return errors.New("S3 context must not be nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if key == "" || len(key) > 1024 || !utf8.ValidString(key) {
		return errors.New("S3 object key must contain 1 to 1024 valid UTF-8 bytes")
	}
	return nil
}

// PutObject uploads a known-length stream without buffering it in memory.
// The caller owns body and must close it. A seekable body allows SDK retries.
// Existing content at key is overwritten; callers must authorize that operation.
func (s *S3Store) PutObject(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	if err := s.validate(ctx, key); err != nil {
		return err
	}
	if body == nil || size < 0 {
		return errors.New("S3 upload requires a body and a nonnegative size")
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(key), Body: body,
		ContentLength: aws.Int64(size), ContentType: aws.String(contentType),
	}, unsignedPayload)
	if err != nil {
		return fmt.Errorf("put S3 object %q: %w", key, err)
	}
	return nil
}

// unsignedPayload lets the S3 client stream non-seekable uploads.
func unsignedPayload(o *s3.Options) {
	o.APIOptions = append(o.APIOptions, v4.SwapComputePayloadSHA256ForUnsignedPayloadMiddleware)
}

// GetObject returns a streaming download. The caller must close Object.Body.
func (s *S3Store) GetObject(ctx context.Context, key string) (*Object, error) {
	if err := s.validate(ctx, key); err != nil {
		return nil, err
	}
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(key),
	})
	if err != nil {
		return nil, objectError("get", key, err)
	}
	return &Object{Body: out.Body, Size: aws.ToInt64(out.ContentLength),
		ContentType: aws.ToString(out.ContentType), ETag: aws.ToString(out.ETag)}, nil
}

// HeadObject retrieves metadata without downloading content; Body is nil.
func (s *S3Store) HeadObject(ctx context.Context, key string) (*Object, error) {
	if err := s.validate(ctx, key); err != nil {
		return nil, err
	}
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(key),
	})
	if err != nil {
		return nil, objectError("head", key, err)
	}
	return &Object{Size: aws.ToInt64(out.ContentLength),
		ContentType: aws.ToString(out.ContentType), ETag: aws.ToString(out.ETag)}, nil
}

// ObjectExists only treats missing objects as absent; permission/network errors survive.
func (s *S3Store) ObjectExists(ctx context.Context, key string) (bool, error) {
	_, err := s.HeadObject(ctx, key)
	if errors.Is(err, ErrObjectNotFound) {
		return false, nil
	}
	return err == nil, err
}

func objectError(operation, key string, err error) error {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchKey", "NotFound":
			err = errors.Join(ErrObjectNotFound, err)
		}
	}
	return fmt.Errorf("%s S3 object %q: %w", operation, key, err)
}

// DeleteObject is idempotent for missing keys. Only delete shared content after
// the database confirms that no user references it.
func (s *S3Store) DeleteObject(ctx context.Context, key string) error {
	if err := s.validate(ctx, key); err != nil {
		return err
	}
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete S3 object %q: %w", key, err)
	}
	return nil
}

// CreateMultipartUpload returns the S3 upload ID to persist with the user's session.
// Call AbortMultipartUpload when abandoning a session to release uploaded parts.
func (s *S3Store) CreateMultipartUpload(ctx context.Context, key, contentType string) (string, error) {
	if err := s.validate(ctx, key); err != nil {
		return "", err
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	out, err := s.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(s.bucket), Key: aws.String(key), ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("create S3 multipart upload %q: %w", key, err)
	}
	if aws.ToString(out.UploadId) == "" {
		return "", errors.New("S3 returned an empty multipart upload ID")
	}
	return *out.UploadId, nil
}

func (s *S3Store) validateUpload(ctx context.Context, key, uploadID string) error {
	if err := s.validate(ctx, key); err != nil {
		return err
	}
	if strings.TrimSpace(uploadID) == "" {
		return errors.New("S3 multipart upload ID must not be empty")
	}
	return nil
}

// UploadPart uploads a one-based part and returns its completion token.
// All nonfinal parts must have the same size (at least 5 MiB); the final part
// may be smaller. The caller owns body and must persist the returned ETag.
func (s *S3Store) UploadPart(ctx context.Context, key, uploadID string, partNumber int32, body io.Reader, size int64) (*MultipartPart, error) {
	if err := s.validateUpload(ctx, key, uploadID); err != nil {
		return nil, err
	}
	if partNumber < 1 || partNumber > 10000 || body == nil || size <= 0 || size > 5*1024*1024*1024 {
		return nil, errors.New("S3 part requires a number from 1 to 10000, a body, and a size from 1 byte to 5 GiB")
	}
	out, err := s.client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket: aws.String(s.bucket), Key: aws.String(key), UploadId: aws.String(uploadID),
		PartNumber: aws.Int32(partNumber), Body: body, ContentLength: aws.Int64(size),
	}, unsignedPayload)
	if err != nil {
		return nil, fmt.Errorf("upload S3 part %d for %q: %w", partNumber, key, err)
	}
	if aws.ToString(out.ETag) == "" {
		return nil, errors.New("S3 returned an empty part ETag")
	}
	return &MultipartPart{PartNumber: partNumber, ETag: *out.ETag, Size: size}, nil
}

// ListParts retrieves every page of uploaded parts for resumable upload status.
func (s *S3Store) ListParts(ctx context.Context, key, uploadID string) ([]MultipartPart, error) {
	if err := s.validateUpload(ctx, key, uploadID); err != nil {
		return nil, err
	}
	pages := s3.NewListPartsPaginator(s.client, &s3.ListPartsInput{
		Bucket: aws.String(s.bucket), Key: aws.String(key), UploadId: aws.String(uploadID),
	})
	parts := make([]MultipartPart, 0)
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list S3 parts for %q: %w", key, err)
		}
		for _, part := range page.Parts {
			parts = append(parts, MultipartPart{PartNumber: aws.ToInt32(part.PartNumber),
				ETag: aws.ToString(part.ETag), Size: aws.ToInt64(part.Size)})
		}
	}
	return parts, nil
}

// CompleteMultipartUpload sorts a copy of parts and commits exactly that list.
// Callers must verify the expected part count, total size, and content integrity
// before completion. An ETag is not the application's SHA-256 file hash.
// On error the session remains available for inspection, retry, or explicit abort.
func (s *S3Store) CompleteMultipartUpload(ctx context.Context, key, uploadID string, parts []MultipartPart) error {
	if err := s.validateUpload(ctx, key, uploadID); err != nil {
		return err
	}
	if len(parts) == 0 || len(parts) > 10000 {
		return errors.New("S3 multipart completion requires 1 to 10000 parts")
	}
	ordered := append([]MultipartPart(nil), parts...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].PartNumber < ordered[j].PartNumber })
	completed := make([]types.CompletedPart, len(ordered))
	for i, part := range ordered {
		if part.PartNumber < 1 || part.PartNumber > 10000 || strings.TrimSpace(part.ETag) == "" ||
			(i > 0 && ordered[i-1].PartNumber == part.PartNumber) {
			return errors.New("S3 completion requires unique part numbers from 1 to 10000 and nonempty ETags")
		}
		completed[i] = types.CompletedPart{PartNumber: aws.Int32(part.PartNumber), ETag: aws.String(part.ETag)}
	}
	_, err := s.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket: aws.String(s.bucket), Key: aws.String(key), UploadId: aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{Parts: completed},
	})
	if err != nil {
		return fmt.Errorf("complete S3 multipart upload %q: %w", key, err)
	}
	return nil
}

// AbortMultipartUpload discards uploaded parts. An already absent upload is a success.
func (s *S3Store) AbortMultipartUpload(ctx context.Context, key, uploadID string) error {
	if err := s.validateUpload(ctx, key, uploadID); err != nil {
		return err
	}
	_, err := s.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket: aws.String(s.bucket), Key: aws.String(key), UploadId: aws.String(uploadID),
	})
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchUpload" {
			return nil
		}
		return fmt.Errorf("abort S3 multipart upload %q: %w", key, err)
	}
	return nil
}
