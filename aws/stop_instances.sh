#!/bin/bash

list=$(cat ./metadata/id_list | tr "\n" " ")

aws ec2 stop-instances --instance-ids $list --output text
