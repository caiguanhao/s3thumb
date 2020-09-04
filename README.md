s3thumb
-------

Example AWS Lambda Function to automatically generate thumbnail for images uploaded to S3.

AWS Configs:

```
Runtime = Go 1.x
Handler = s3thumb
Timeout = 10 secs
Memory  = 128MB

Output options:
  Please set TARGET_SIZES environment variable!
  Format: %{width}x%{height}=%{suffix}
  Separate multiple versions with spaces
  For example:
    TARGET_SIZES=300x300=300px 800x800=800px

Lambda function role permission:
1. Open the role page
2. Edit the policy
3. Add additional permissions
4. Choose S3 service
5. Select PutObject and PutObjectAcl actions
6. Select All resources
7. Review and save changes

Create an S3 trigger:
Event type: ObjectCreated
Prefix: images/ (optional, for example)
```

Run `make` to make `s3thumb.zip` and upload it to AWS Lambda Function.

Fork from <https://github.com/chankh/lambda-s3-thumbnail>.
