#!/bin/bash -e
sudo docker build -t thrawler_build .
rm -rf target
mkdir target
docker run --rm --name thrawler_build thrawler_build:latest | tar xvC target/ --strip-components 1
docker rmi thrawler_build
