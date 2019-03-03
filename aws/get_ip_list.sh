#!/usr/bin/env bash

list=$(cat ./metadata/id_list | tr "\n" " ")

aws ec2 describe-instances --instance-ids $list --query 'Reservations[*].Instances[*].PublicIpAddress' --output text | tr "\t" "\n"
