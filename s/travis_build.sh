#!/bin/bash
set -u -e -o pipefail -o verbose

cd cmd/gen-books
go build -o gen-books
cd ../..

./cmd/gen-books/gen-books -analytics UA-113489735-1

if [ -z ${NETLIFY_TOKEN+x} ]
then
    echo "Skipping upload because NETLIFY_TOKEN not set"
else
    ./netlifyctl -A $NETLIFY_TOKEN deploy || true
    cat netlifyctl-debug.log || true
fi
