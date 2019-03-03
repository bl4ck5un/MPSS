#!/usr/bin/env bash

pscp -O 'StrictHostKeyChecking=no' -O 'IdentityFile=./mpss.pem' -h metadata/addr_list -l ec2-user -r ./scripts /home/ec2-user
