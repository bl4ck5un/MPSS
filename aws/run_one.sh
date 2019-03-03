#!/usr/bin/env bash

config=$1

test -f ${config} || {
	echo "$config is not a file"
	exit -1
}

./go.py -c ${config}
until ./go.py -c ${config} --ready
do
	sleep 5
done
./go.py -c ${config} --fetch
