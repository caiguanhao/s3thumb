s3thumb.zip: s3thumb
	zip s3thumb.zip s3thumb

s3thumb: s3thumb.go
	GOOS=linux GOARCH=amd64 go build -v

upload: s3thumb.zip
	aws lambda update-function-code \
		--function-name s3thumb \
		--zip-file fileb://s3thumb.zip

update:
	aws lambda update-function-configuration \
		--function-name s3thumb \
		--handler s3thumb \
		--timeout 10 \
		--memory-size 128 \
		--environment "Variables={TARGET_SIZES=300x300=300px 800x800=800px}"

.PHONY: upload update
