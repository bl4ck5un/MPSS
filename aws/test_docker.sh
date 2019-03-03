#!/bin/bash

pssh -h metadata/addr_list -l ubuntu -O "IdentityFile=./mpss.pem" "docker run hello-world"
