#!/usr/bin/env bash

set -e

apt-get update && apt-get install -y wget gcc libpcap-dev \
&& wget https://dl.google.com/go/go1.20.4.linux-amd64.tar.gz \
&& tar -C /usr/local -xzf go1.20.4.linux-amd64.tar.gz \
&& rm go1.20.4.linux-amd64.tar.gz

# add this to your .bashrc
export GOROOT=/usr/local/go
export GOPATH=/go
export PATH=$GOPATH/bin:$GOROOT/bin:$PATH
