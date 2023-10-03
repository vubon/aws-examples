package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"flag"
	"log"
	"net/url"
	"os"
	"strings"
)

var (
	dirName      = flag.String("dir-name", "", "Files directory")
	newBucket    = flag.String("new-bucket", "", "For creating new AWS S3 bucket name")
	uploadCert   = flag.Bool("upload-cert", false, "Upload Certificate into Cloudfront")
	distribution = flag.Bool("create-distribution", false, "Create a new distribution at Cloudfront")
	clean        = flag.Bool("clean", false, "Clean Old memory")
)

type FileWriter struct {
	*csv.Writer
}

type Client struct {
	S3C *S3Client
	CFC *CFClient
	MC  *Memory
}

type Statement struct {
	Sid       string `json:"Sid"`
	Effect    string `json:"Effect"`
	Principal struct {
		Service string `json:"Service"`
	} `json:"Principal"`
	Action    string `json:"Action"`
	Resource  string `json:"Resource"`
	Condition struct {
		StringEquals struct {
			AWSSourceArn string `json:"AWS:SourceArn"`
		} `json:"StringEquals"`
	} `json:"Condition"`
}

type PolicyDocument struct {
	Version   string    `json:"Version"`
	Id        string    `json:"Id"`
	Statement Statement `json:"Statement"`
}

func generateCSVFile() (*FileWriter, *os.File) {
	// Create or open the output file
	outputFile, _ := os.OpenFile("output.csv", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)

	// Create a CSV writer for the output file
	fileWriter := &FileWriter{csv.NewWriter(outputFile)}
	headers := []string{"File Name", "URL"}

	err := fileWriter.Write(headers)
	if err != nil {
		return nil, nil
	}
	return fileWriter, outputFile
}

func formatMerchantId(key string) string {
	return strings.Split(key, ".")[0]
}

func (c *Client) CreateBucket(bucketName string) {
	memory := c.MC.getMemory()
	if memory.BucketDomain != "" {
		return
	}
	location := c.S3C.CreateBucket(bucketName)
	parsedURL, err := url.Parse(location)
	if err != nil {
		log.Println("Error parsing URL:", err)
	}
	memory.BucketName = bucketName
	memory.BucketDomain = parsedURL.Host
	_ = writeMemory(memory)
}

func (c *Client) BucketPolicyUpdate() {
	mem := c.MC.getMemory()
	policy := PolicyDocument{
		Version: "2012-10-17",
		Id:      "PolicyForCloudFrontPrivateContent",
		Statement: Statement{
			Sid:    "AllowCloudFrontServicePrincipalReadOnly",
			Effect: "Allow",
			Principal: struct {
				Service string `json:"Service"`
			}(struct{ Service string }{Service: "cloudfront.amazonaws.com"}),
			Action:   "s3:GetObject",
			Resource: "arn:aws:s3:::" + mem.BucketName + "/*",
			Condition: struct {
				StringEquals struct {
					AWSSourceArn string `json:"AWS:SourceArn"`
				} `json:"StringEquals"`
			}{
				StringEquals: struct {
					AWSSourceArn string `json:"AWS:SourceArn"`
				}{
					AWSSourceArn: mem.DistributionArn,
				},
			},
		},
	}
	jsonData, err := json.Marshal(policy)
	if err != nil {
		log.Println("Policy update marshal error ", err)
		return
	}
	err = c.S3C.PolicyUpdate(mem.BucketName, string(jsonData))
	if err != nil {
		log.Println("Policy update error ", err)
		return
	}
}

func (c *Client) WithoutUpload() {
	writer, _ := generateCSVFile()
	defer writer.Flush()

	mem := c.MC.getMemory()
	objects := c.S3C.GetFileList(mem.BucketName)

	for _, object := range objects {
		if strings.HasSuffix(*object.Key, ".csv") {
			newUrl := c.PreSignedURL(mem.CFKeyId, mem.CloudFrontDomain+"/"+*object.Key)

			// Write the record to the file
			err := writer.Write([]string{formatMerchantId(*object.Key), newUrl})
			if err != nil {
				log.Println(err)
			}
		}
	}
}

func (c *Client) WithUpload(dir string) {
	// CSV Generator
	writer, _ := generateCSVFile()
	defer writer.Flush()

	// Read files from the directory
	files, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	mem := c.MC.getMemory()

	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".csv") {
			content, err := os.ReadFile(dir + "/" + file.Name())
			if err != nil {
				log.Println("Could not find the file: ", file.Name(), " ", err)
			}
			c.S3C.Upload(file.Name(), mem.BucketName, bytes.NewReader(content))
			newUrl := c.PreSignedURL(mem.CFKeyId, mem.CloudFrontDomain+"/"+file.Name())
			// Write the record to the file
			err = writer.Write([]string{formatMerchantId(file.Name()), newUrl})
			if err != nil {
				log.Println(err)
			}
		}
	}
}

func (c *Client) PreSignedURL(keyId, url string) string {
	newUrl, err := c.CFC.CreatePreSignedURl(url, keyId, "private_key.pem")
	if err != nil {
		log.Println("URL generate URL ", err)
		return ""
	}
	return newUrl
}

func (c *Client) UploadCertificate() (string, string) {
	memory := c.MC.getMemory()
	// If already Upload the certificate and created the group
	if memory.CFKeyId != "" {
		return memory.CFKeyId, memory.CFGroupId
	}
	file, err := os.ReadFile("public_key.pem")
	if err != nil {
		log.Println("public key read error ", err)
		return "", ""
	}

	comment := "Server S3 object Presigned URL"
	keyId, err := c.CFC.CreatePublicKey("bucket", comment, string(file))
	if err != nil {
		log.Println("Public key upload error ", err)
		return "", ""
	}
	// Write memory
	memory.CFKeyId = keyId
	_ = writeMemory(memory)

	groupId, err := c.CFC.CreateKeyGroup("bucket", comment, []string{keyId})
	if err != nil {
		log.Println("public key group error", err)
		return "", ""
	}
	// Write memory
	memory.CFGroupId = groupId
	_ = writeMemory(memory)

	return keyId, groupId
}

func (c *Client) Distribution() {
	memory := c.MC.getMemory()
	comment := "Server S3 object Presigned URL"
	created, err := c.CFC.CreateDistributions(comment, memory.BucketDomain, []string{memory.CFGroupId})
	if err != nil {
		log.Println("Creation Distribution Error ", err)
		return
	}
	log.Println(*created.DomainName, *created.Status, *created.Id, *created.ARN)

	// Write memory
	memory.CloudFrontDomain = "https://" + *created.DomainName
	memory.DistributionArn = *created.ARN
	_ = writeMemory(memory)
}

func main() {
	flag.Parse()
	s3c, err := New()
	cfc, err := NewCFClient()
	if err != nil {
		log.Println("Client init error ", err)
		return
	}
	mc := NewMemory()
	client := Client{S3C: s3c, CFC: cfc, MC: mc}

	switch {
	case *dirName != "":
		client.WithUpload(*dirName)
	case *newBucket != "":
		client.CreateBucket(*newBucket)
	case *uploadCert:
		client.UploadCertificate()
	case *distribution:
		client.Distribution()
		client.BucketPolicyUpdate()
	case *clean:
		client.MC.cleanMemory()
	default:
		client.WithoutUpload()
	}
}
