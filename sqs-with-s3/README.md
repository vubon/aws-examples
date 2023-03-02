## S3 Bucket & SQS Integration
We can integrate S3 bucket and SQS by setting up event notifications on the S3 bucket. 
This way, whenever a new object is created or deleted in the bucket, an event notification is sent to SQS,
which can then trigger a message to be sent to a target system or application.
