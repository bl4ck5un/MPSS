#!/bin/bash

primary=$(head -n 1 metadata/addr_list)

user=ec2-user


ssh -i mpss.pem $user@${primary} docker stop primary
pssh -ih metadata/addr_list -l $user -O "IdentityFile=./mpss.pem" docker stop node

