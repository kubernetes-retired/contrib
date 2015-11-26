#!/usr/bin/bash

echo 'This is for creating database secrets on our k8s platform! '
printf '\nEnter your dbname: (please use with specific meaning, eg. yinnut-msql-dev, yinnut-mysql-prod, etc)'
read db_name
printf '\nEnter your username: '
read username
encode_user=$(echo $username | base64)
printf '\nEnter your password: '
read password
encode_pwd=$(echo $password | base64)

echo ""

if ! kubectl get secrets/${db_name} > /dev/null 2>&1; then
  (cat <<EOF
  apiVersion: v1
  kind: Secret
  metadata:
    name: ${db_name}
  data:
    username: ${encode_user}
    password: ${encode_pwd}
EOF
  )| kubectl create -f -
  echo "Created db [${db_name}] secret!"
else
  echo "db [${db_name}] secret already exist!"
fi

kubectl get secrets/${db_name}
