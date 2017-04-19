#!/bin/bash

# Run script with the url of the GCS bucket as the first argument
# Pulls all files in the bucket and puts them into a folder and
# runs the rdjunit file
url=$1
bucket=${url#h*[e][r][/]}
echo $bucket
bucket=${bucket%/*}
filename=${bucket##**/}
datetime=$(date +"%m_%d_%Y-%H_%M_%S")
filename=$filename-$datetime
mkdir $filename
cd $filename
gslink=gs://$bucket/*
gsutil -m cp -r $gslink .
cd ..
go run rdjunit.go rdkubeapi.go rdkubelet.go $filename > output.txt
