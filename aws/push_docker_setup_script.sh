#!/usr/bin/env bash

cat << EOF > docker_setup.sh
sudo apt-get update && \
	sudo apt-get install --yes apt-transport-https ca-certificates curl software-properties-common && \
	curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add - && \
	sudo add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu bionic stable" && \
	sudo apt-get update && \
	sudo apt-get install --yes docker-ce

sudo groupadd docker
sudo usermod -aG docker \$USER
EOF

pscp -O 'IdentityFile=./mpss.pem' -h metadata/addr_list -l ubuntu ./docker_setup.sh /home/ubuntu

rm -rf docker_setup.sh
