#!/bin/bash

if [ -n "$1" ]; then
	cd "$1" || exit 1
fi

# Step 1: Get the leaf certificate
openssl s_client -connect www.akamai.com:443 < /dev/null 2>&1 |  sed -n '/-----BEGIN/,/-----END/p' > leaf.pem

# Step 2: Get the intermediate certificate
openssl s_client -showcerts -connect www.akamai.com:443 < /dev/null 2>&1 |  sed -n '/-----BEGIN/,/-----END/p' > intermediate.pem
