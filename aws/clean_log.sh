#!/bin/bash

user=ec2-user

pssh -ih metadata/addr_list -l $user -O "IdentityFile=./mpss.pem" "sudo rm -rf scripts/log* scripts/*.log"
