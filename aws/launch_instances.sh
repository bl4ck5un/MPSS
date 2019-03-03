#!/bin/bash

# ami-0ac019f4fcb7cb7e6 is Ubuntu Server 18.04 LTS

aws ec2 run-instances --image-id ami-0ac019f4fcb7cb7e6 \
	--subnet-id subnet-034102a5710c6337a \
	--security-group-ids sg-0bcb62ba07503348c \
	--count 101 \
	--instance-type c5.large \
	--key-name mpss \
	--associate-public-ip-address
