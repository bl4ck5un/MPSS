#!/usr/bin/env bash

for config in $(ls -1 scripts/ | sort -n -k 1.11 | grep toml); do
	./go.py -c scripts/$config
	until ./go.py -c scripts/$config --ready
	do
		sleep 10
	done
	./go.py -c scripts/$config --fetch
done
