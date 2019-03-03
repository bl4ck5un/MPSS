#!/bin/bash

LATEST=$(docker inspect churp/mpss:latest --format "{{index .RepoDigests 0}}")

test -z $LATEST && {
	echo "error"
	exit 1
}

user=ec2-user

pssh -ih metadata/addr_list -l $user -O "IdentityFile=./mpss.pem" 'docker inspect churp/mpss:latest --format "{{index .RepoDigests 0}}"' | grep -v ${LATEST}
