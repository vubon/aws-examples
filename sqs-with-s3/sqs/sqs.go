package sqs

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
)

const QueueName = "<Your SQS Name>"

var (
	svc      *sqs.SQS
	queueURL *string
)

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
}

func MessageHandler(msg *sqs.Message) {
	fmt.Println("RECEIVING MESSAGE >>> ")
	fmt.Println(*msg.Body)
}

func SQS() {
	// Create a session that gets credential values from ~/.aws/credentials
	// and the default region from ~/.aws/config
	sess := session.Must(session.NewSessionWithOptions(session.Options{
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
		DeleteMessage(message)
	}
}
