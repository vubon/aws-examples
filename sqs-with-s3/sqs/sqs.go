package sqs

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/vubon/aws-examples/sqs-with-s3/s3"
	"time"
)

const QueueName = "<Your SQS Name>"

var (
	svc      *sqs.SQS
	queueURL *string
	sess     *session.Session
)

type Response struct {
	Records []struct {
		EventVersion string    `json:"eventVersion"`
		EventSource  string    `json:"eventSource"`
		AwsRegion    string    `json:"awsRegion"`
		EventTime    time.Time `json:"eventTime"`
		EventName    string    `json:"eventName"`
		UserIdentity struct {
			PrincipalId string `json:"principalId"`
		} `json:"userIdentity"`
		RequestParameters struct {
			SourceIPAddress string `json:"sourceIPAddress"`
		} `json:"requestParameters"`
		ResponseElements struct {
			XAmzRequestId string `json:"x-amz-request-id"`
			XAmzId2       string `json:"x-amz-id-2"`
		} `json:"responseElements"`
		S3 struct {
			S3SchemaVersion string `json:"s3SchemaVersion"`
			ConfigurationId string `json:"configurationId"`
			Bucket          struct {
				Name          string `json:"name"`
				OwnerIdentity struct {
					PrincipalId string `json:"principalId"`
				} `json:"ownerIdentity"`
				Arn string `json:"arn"`
			} `json:"bucket"`
			Object struct {
				Key       string `json:"key"`
				Size      int    `json:"size"`
				ETag      string `json:"eTag"`
				Sequencer string `json:"sequencer"`
			} `json:"object"`
		} `json:"s3"`
	} `json:"Records"`
}

func GetQueueURL(queue string) (*sqs.GetQueueUrlOutput, error) {
	urlResult, err := svc.GetQueueUrl(&sqs.GetQueueUrlInput{
		QueueName: &queue,
	})
	// snippet-end:[sqs.go.receive_messages.queue_url]
	if err != nil {
		return nil, err
	}

	return urlResult, nil
}

func pullMessages(chn chan<- *sqs.Message) {
	for {
		output, err := svc.ReceiveMessage(&sqs.ReceiveMessageInput{
			AttributeNames: []*string{
				aws.String(sqs.MessageSystemAttributeNameSentTimestamp),
			},
			MessageAttributeNames: []*string{
				aws.String(sqs.QueueAttributeNameAll),
			},
			QueueUrl:            queueURL,
			MaxNumberOfMessages: aws.Int64(2),
			WaitTimeSeconds:     aws.Int64(15),
		})

		if err != nil {
			fmt.Printf("failed to fetch sqs message %v\n", err)
		}

		for _, message := range output.Messages {
			chn <- message
		}

	}
}

func DeleteMessage(msg *sqs.Message) {
	_, err := svc.DeleteMessage(&sqs.DeleteMessageInput{
		QueueUrl:      queueURL,
		ReceiptHandle: msg.ReceiptHandle,
	})
	if err != nil {
		fmt.Println("Delete error", err)
	}
	fmt.Println("Delete Queue message: ", msg.MessageId)
}

func MessageHandler(msg *sqs.Message) {
	fmt.Println("RECEIVING MESSAGE >>> ")
	//fmt.Println(*msg.Body)
	var resp Response
	errJSON := json.Unmarshal([]byte(*msg.Body), &resp)
	if errJSON != nil {
		fmt.Println("JSON Unmarshal error", errJSON)
	}
	bucketName := resp.Records[0].S3.Bucket.Name
	fileName := resp.Records[0].S3.Object.Key
	fmt.Println("Bucket name:  ", bucketName, "File Name: ", fileName)
	err := s3.DownloadObject(sess, fileName, bucketName)
	if err != nil {
		fmt.Println("Got error ", err)
	}
	// If everything is okay, then delete the Queue message.
	DeleteMessage(msg)
}

func SQS() {
	// Create a session that gets credential values from ~/.aws/credentials
	// and the default region from ~/.aws/config
	sess = session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	// Create an SQS service client
	svc = sqs.New(sess)

	// Get URL of queue
	urlResult, err := GetQueueURL(QueueName)
	if err != nil {
		fmt.Println("Got an error getting the queue URL:", err)
	}
	queueURL = urlResult.QueueUrl

	chnMessages := make(chan *sqs.Message, 2)
	go func() {
		defer func() {
			if err := recover(); err != nil {
				fmt.Println("Recover from second layer of concurrency", err)
			}
		}()
		pullMessages(chnMessages)
	}()

	for message := range chnMessages {
		MessageHandler(message)
	}
}
