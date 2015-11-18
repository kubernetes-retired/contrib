#!/bin/bash

alias ansible='ansible -i inventory.dev'
alias ssh='ssh -i private_keys/ansible_private_key -o "StrictHostKeyChecking no"'
