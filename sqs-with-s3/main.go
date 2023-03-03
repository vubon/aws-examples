package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/vubon/aws-examples/sqs-with-s3/sqs"
)

func main() {
	mux := http.NewServeMux()
	go func() {
		defer func() {
			if err := recover(); err != nil {
				fmt.Println("SQS recover from panic attack ", err)
			}
		}()
		sqs.SQS()
	}()
	err := http.ListenAndServe(":8080", mux)
	if err != nil {
		os.Exit(1)
	}

}
