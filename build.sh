#!/bin/bash
rm -rf tcpF.*
CGO_ENABLED=0 go build  -buildmode=exe -o Forward.run .
CGO_ENABLED=0 GOOS=windows go build -buildmode=exe -o Forward.exe .
echo ok