#!/bin/bash

mkdir -p metadata

rm -rf metadata/addr_list
rm -rf metadata/id_list

filter="Name=instance-state-name,Values=running Name=key-name,Values=mpss"

list=$(aws ec2 describe-instances --filters $filter --query 'Reservations[*].Instances[*].PublicIpAddress' --output text)

for line in $(echo $list | tr "\t" "\n")
do
  echo $line >> metadata/addr_list
done

aws ec2 describe-instances --filters $filter --query 'Reservations[*].Instances[*].InstanceId' --output text | tr "\t" "\n" > metadata/id_list
