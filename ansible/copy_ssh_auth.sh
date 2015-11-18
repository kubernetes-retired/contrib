#!/bin/bash -x

ssh_auth=$(cat ./private_keys/ansible_private_key.pub)

for i in {232..242}; do
  sshpass -p 123456 ssh root@192.168.1."$i" "(mkdir ~/.ssh && touch ~/.ssh/authorized_keys) && (echo $ssh_auth >> ~/.ssh/authorized_keys)"
done
