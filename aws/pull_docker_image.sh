#!/bin/bash

user=ec2-user
pssh -h metadata/addr_list -l $user -O "IdentityFile=./mpss.pem" 'docker pull churp/mpss'
