#!/bin/bash

bucket=<> # bucket name
profile=<> # aws profile name

aws --profile ${profile} s3 cp ./pf9-2.0.yml s3://${bucket}/tmp/
curl -o pf9-3.0-tmp.json https://converter.swagger.io/api/convert?url=https://${bucket}.s3.us-east-2.amazonaws.com/tmp/pf9-2.0.yml
aws --profile ${profile} s3 rm s3://${bucket}/tmp/pf9-2.0.yml

# output file pf9-3.0-tmp.json still needs to be reviewed manually and corrected, there are several mistakes
