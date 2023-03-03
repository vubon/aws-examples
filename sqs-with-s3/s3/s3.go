package s3

import (
	"bytes"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

func DownloadObject(sess *session.Session, filename string, bucket string) error {
	svc := s3.New(sess)
	rawObject, err := svc.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(filename),
	})
	if err != nil {
		return err
	}
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(rawObject.Body)
	if err != nil {
		return err
	}
	fmt.Println(buf.String())
	return nil
}
