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

const (
	letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
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
	options = []Option{}
)

type (
	File struct {
		Bucket string
		Key    string
		Format string
	}

	Option struct {
		Width  int
		Height int
		Suffix string
	}
)

func (f *File) Process() {
	if f.Download() != nil {
		return
	}

	if f.GetFormat() != nil {
		return
	}

	for i := 0; i < len(options); i++ {
		f.ResizeAndUpload(options[i].Width, options[i].Height, options[i].Suffix)
	}
}

func (f *File) LocalFile() string {
	return filepath.Join(os.TempDir(), f.Bucket, f.Key)
}

func (f *File) GetFormat() (err error) {
	localFile := f.LocalFile()
	var file *os.File
	file, err = os.Open(localFile)
	if err != nil {
		log.WithError(err).WithField("filename", localFile).Error("failed to open file")
		return
	}
	bytes := make([]byte, 4)
	file.ReadAt(bytes, 0)
	if bytes[0] == 0x89 && bytes[1] == 0x50 && bytes[2] == 0x4E && bytes[3] == 0x47 {
		f.Format = "png"
	} else if bytes[0] == 0xFF && bytes[1] == 0xD8 {
		f.Format = "jpg"
	} else if bytes[0] == 0x47 && bytes[1] == 0x49 && bytes[2] == 0x46 && bytes[3] == 0x38 {
		f.Format = "gif"
	}
	if f.Format == "" {
		err = errors.New("no format")
		return
	}
	return
}

func (f *File) Download() (err error) {
	localFile := f.LocalFile()

	dir := filepath.Dir(localFile)
	if err = os.MkdirAll(dir, os.ModePerm); err != nil {
		log.WithError(err).WithField("path", dir).Error("failed to create tmp directory")
		return
	}

	var file *os.File
	file, err = os.Create(localFile)
	if err != nil {
		log.WithError(err).WithField("filename", localFile).Error("failed to create file")
		return
	}

	var n int64
	n, err = downloader.Download(file, &s3.GetObjectInput{
		Bucket: aws.String(f.Bucket),
		Key:    aws.String(f.Key),
	})
	if err != nil {
		log.WithError(err).WithFields(log.Fields{"bucket": f.Bucket, "key": f.Key, "filename": localFile}).Error("failed to download file")
		return
	}

	log.WithFields(log.Fields{"filename": localFile, "bytes": n}).Info("file downloaded")
	return
}

func (f *File) ResizeAndUpload(width, height int, suffix string) (err error) {
	localFile := f.LocalFile()

	var img image.Image
	img, err = imaging.Open(localFile)
	if err != nil {
		log.WithError(err).WithField("filename", localFile).Error("failed to open file")
		return
	}

	tmpFile := filepath.Join(os.TempDir(), randomString(10)+"."+f.Format)

	thumb := imaging.Thumbnail(img, width, height, imaging.CatmullRom)
	dst := imaging.New(width, height, color.NRGBA{0, 0, 0, 0})
	dst = imaging.Paste(dst, thumb, image.Pt(0, 0))

	err = imaging.Save(dst, tmpFile)
	if err != nil {
		log.WithError(err).WithField("filename", tmpFile).Error("failed to generate thumbnail")
		return
	}

	var file *os.File
	file, err = os.Open(tmpFile)
	if err != nil {
		log.WithError(err).WithField("filename", tmpFile).Error("failed to open file")
		return
	}

	newKey := f.Key + "/" + suffix
	var result *s3manager.UploadOutput
	result, err = uploader.Upload(&s3manager.UploadInput{
		ACL:         aws.String("public-read"),
		Bucket:      aws.String(f.Bucket),
		ContentType: aws.String(contentTypes[f.Format]),
		Key:         aws.String(newKey),
		Body:        file,
	})

	if err != nil {
		log.WithError(err).WithFields(log.Fields{"bucket": f.Bucket, "key": newKey}).Error("failed to upload file")
		return
	}

	log.WithField("location", result.Location).Info("file uploaded")
	return
}

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

	sizes := strings.Fields(os.Getenv("TARGET_SIZES"))
	for i := 0; i < len(sizes); i++ {
		var width, height int
		var suffix string
		fmt.Sscanf(sizes[i], "%dx%d=%s", &width, &height, &suffix)
		if width > 0 && height > 0 && suffix != "" {
			options = append(options, Option{
				Width:  width,
				Height: height,
				Suffix: suffix,
			})
		}
	}
	if len(options) < 1 {
		return "", errors.New("no options")
	}

outer:
	for _, r := range req.Records {
		key := r.S3.Object.Key
		for i := 0; i < len(options); i++ {
			if strings.HasSuffix(key, options[i].Suffix) {
				log.WithField("key", key).Info("object ignored")
				continue outer
			}
		}
		file := File{
			Bucket: r.S3.Bucket.Name,
			Key:    key,
		}
		file.Process()
	}
	return fmt.Sprintf("%d records processed", len(req.Records)), nil
}

func main() {
	lambda.Start(handle)
}
