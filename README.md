s3thumb
-------

Example AWS Lambda Function to automatically generate thumbnail for images uploaded to S3.

AWS Configs:

```
Runtime = Go 1.x
Handler = s3thumb
Timeout = 10 secs

S3 trigger:
Suffix: images/original
Prefix: users/
Event type: ObjectCreated

S3 Permission:
Allow: s3:PutObject
Allow: s3:PutObjectAcl
```

Build and make zip file, and upload the zip file to AWS Lambda Function:

```
GOOS=linux GOARCH=amd64 go build -v
zip s3thumb.zip s3thumb
```

Fork from <https://github.com/chankh/lambda-s3-thumbnail>.
