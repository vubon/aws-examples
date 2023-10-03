package main

import (
	"encoding/json"
	"log"
	"os"
)

const filePath = "data.json"

type Memory struct {
	BucketName       string
	BucketDomain     string
	CFGroupId        string
	CFKeyId          string
	CloudFrontDomain string
	DistributionArn  string
}

func NewMemory() *Memory {
	return &Memory{}
}

func (m *Memory) getMemory() *Memory {
	// Create or open the output file
	_, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		_ = writeMemory(m)
		return m
	}

	data, _ := os.ReadFile(filePath)
	err = json.Unmarshal(data, m)
	if err != nil {
		log.Println("Memory read error ", err)
		return m
	}
	return m
}

func writeMemory(m *Memory) error {
	// Marshal the data into JSON format
	jsonData, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	err = os.WriteFile(filePath, jsonData, 0644)
	if err != nil {
		return err
	}
	return nil
}

func (m *Memory) cleanMemory() {
	err := os.Remove(filePath)
	if err != nil {
		return
	}
}
