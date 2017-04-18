#!/bin/bash

find . -type d | while read line; do
	( echo Building in the $line ....; cd $line && go build .)
done
