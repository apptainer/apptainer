#!/bin/sh

# pass the path to the PEM-encoded certificate as first argument to the script, and then append to index.txt

crt=$1
exp=$(date -d "$(openssl x509 -enddate -noout -in $crt | cut -d= -f 2)" +"%y%m%d%H%M%SZ")
ser=$(openssl x509 -serial -noout -in $crt | cut -d= -f 2)
sub=$(openssl x509 -subject -noout -in $crt | cut -d= -f 2- | cut -d' ' -f 2-)
printf "V\t$exp\t\t$ser\tunknown\t$sub"
