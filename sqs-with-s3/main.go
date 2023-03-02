package main

import (
	"net/http"
	"os"

	"github.com/vubon/aws-examples/sqs-with-s3/sqs"
)

func main() {
	mux := http.NewServeMux()
	go sqs.SQS()
	err := http.ListenAndServe(":8080", mux)
	if err != nil {
		os.Exit(1)
	}

}
