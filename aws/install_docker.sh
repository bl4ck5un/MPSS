#!/bin/bash

pssh -ih metadata/addr_list -l ec2-user -O "IdentityFile=./mpss.pem" "sh docker_setup.sh"
