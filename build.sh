#!/bin/bash
if git diff-index --quiet HEAD --; then
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -v -o main .
  image="docker.io/bcxq/report-server:preview-v0.0.1"
  docker build . -t $image
  docker push $image
else
  echo git not clean, abort.
fi
