# S3 File Upload and Pre-Signed URL Generation with AWS SDK in Golang

## Prerequisites

You need to set up your AWS credentials. If the `.aws` folder does not exist in your home directory,
please create it first and add the following two files inside the folder:

`config` file should have the following data. Feel free to change the AWS region as needed:

```markdown
[default]
region = ap-southeast-1
output = json
```

`credentials` file should be updated with your AWS credentials:

```markdown
[default]
aws_access_key_id = <Use your aws_access_key_id>
aws_secret_access_key = <Use your aws_secret_access_key>
```

## Steps for Generating Pre-Signed URLs

Please follow the above steps for generating pre-signed URLs.

1. **Generate Certificates**:

    ```
    sh generate.sh
    ```

2. **Create S3 Bucket**:

    ```
    go run main.go cloudfront.go json.go s3.go --new-bucket=<bucket name>
    ```

3. **Upload a Public Certificate and Create CloudFront Public Key and Group Key**:

    ```
    go run main.go cloudfront.go json.go s3.go --upload-cert
    ```

4. **Create a CloudFront Distribution and Update S3 Bucket Policy**:

    ```
    go run main.go cloudfront.go json.go s3.go --create-distribution
    ```

5. **Upload Files and Create Pre-Signed URLs**:

    ```
    go run main.go cloudfront.go json.go s3.go --dir-name=<File directory path>
    ```

6. **Clean Old Data**: Optional

    ```
    go run main.go cloudfront.go json.go s3.go --clean
    ```
