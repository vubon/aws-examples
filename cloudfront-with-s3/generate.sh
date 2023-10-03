#!/bin/bash
echo "########### Generate Private Certificate ##########"
openssl genrsa -out private_key.pem 2048

echo "########### Generate Public Certificate ##########"

openssl rsa -pubout -in private_key.pem -out public_key.pem