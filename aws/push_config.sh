#!/usr/bin/env bash

pscp -e conf.stderr -O 'StrictHostKeyChecking=no' -O 'IdentityFile=./mpss.pem' -h metadata/addr_list -l ubuntu ./config-*.toml /home/ubuntu
