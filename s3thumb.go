package main

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/disintegration/imaging"
	log "github.com/sirupsen/logrus"
)

var (
	sess         = session.Must(session.NewSession())
	uploader     = s3manager.NewUploader(sess)
	downloader   = s3manager.NewDownloader(sess)
	contentTypes = map[string]string{
		"jpg": "image/jpeg",
		"png": "image/png",
		"gif": "image/gif",
	}
)

const (
	letterBytes     = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	targetImageSize = 200
)

func randomString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func handle(ctx context.Context, req events.S3Event) (string, error) {
	log.SetOutput(os.Stdout)
	log.Infof("%v", req)
	for _, r := range req.Records {
		key := r.S3.Object.Key
		process(r.S3.Bucket.Name, key)
	}
	return fmt.Sprintf("%d records processed", len(req.Records)), nil
}

func process(bucket, key string) {
	localFile, err := downloadFile(bucket, key)
	if err != nil {
		return
	}

	newFile, newKey, format, err := resizeImage(localFile, key)
	if err != nil {
		return
	}

	err = uploadFile(newFile, bucket, newKey, format)
	if err != nil {
		return
	}
}

func downloadFile(bucket, key string) (localFile string, err error) {
	localFile = filepath.Join(os.TempDir(), bucket, key)

	dir := filepath.Dir(localFile)
	if err = os.MkdirAll(dir, os.ModePerm); err != nil {
		log.WithError(err).WithField("path", dir).Error("failed to create tmp directory")
		return
	}

	var f *os.File
	f, err = os.Create(localFile)
	if err != nil {
		log.WithError(err).WithField("filename", localFile).Error("failed to create file")
		return
	}

	var n int64
	n, err = downloader.Download(f, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		log.WithError(err).WithFields(log.Fields{"bucket": bucket, "key": key, "filename": localFile}).Error("failed to download file")
		return
	}

	log.WithFields(log.Fields{"filename": localFile, "bytes": n}).Info("file downloaded")
	return
}

func resizeImage(localFile, key string) (newFile, newKey, format string, err error) {
	var file *os.File
	file, err = os.Open(localFile)
	if err != nil {
		log.WithError(err).WithField("filename", localFile).Error("failed to open file")
		return
	}
	bytes := make([]byte, 4)
	file.ReadAt(bytes, 0)
	if bytes[0] == 0x89 && bytes[1] == 0x50 && bytes[2] == 0x4E && bytes[3] == 0x47 {
		format = "png"
	} else if bytes[0] == 0xFF && bytes[1] == 0xD8 {
		format = "jpg"
	} else if bytes[0] == 0x47 && bytes[1] == 0x49 && bytes[2] == 0x46 && bytes[3] == 0x38 {
		format = "gif"
	}
	if format == "" {
		err = errors.New("no format")
		return
	}

	var img image.Image
	img, err = imaging.Open(localFile)
	if err != nil {
		log.WithError(err).WithField("filename", localFile).Error("failed to open file")
		return
	}

	newKey = strings.TrimSuffix(key, "/original") + fmt.Sprintf("/%d", targetImageSize)
	newFile = filepath.Join(os.TempDir(), randomString(10)+"."+format)

	thumb := imaging.Thumbnail(img, targetImageSize, targetImageSize, imaging.CatmullRom)
	dst := imaging.New(targetImageSize, targetImageSize, color.NRGBA{0, 0, 0, 0})
	dst = imaging.Paste(dst, thumb, image.Pt(0, 0))

	err = imaging.Save(dst, newFile)
	if err != nil {
		log.WithError(err).WithField("filename", newFile).Error("failed to generate thumbnail")
		return
	}
	return
}

func uploadFile(localFile, bucket, key, format string) (err error) {
	var file *os.File
	file, err = os.Open(localFile)
	if err != nil {
		log.WithError(err).WithField("filename", localFile).Error("failed to open file")
		return
	}

	var result *s3manager.UploadOutput
	result, err = uploader.Upload(&s3manager.UploadInput{
		ACL:         aws.String("public-read"),
		Bucket:      aws.String(bucket),
		ContentType: aws.String(contentTypes[format]),
		Key:         aws.String(key),
		Body:        file,
	})

	if err != nil {
		log.WithError(err).WithFields(log.Fields{"bucket": bucket, "key": key}).Error("failed to upload file")
		return
	}

	log.WithField("location", result.Location).Info("file uploaded")
	return
}

func main() {
	lambda.Start(handle)
}
