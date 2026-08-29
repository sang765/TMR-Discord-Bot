#!/bin/bash

cd /home/container

if [ ! -f "./tmr-bot" ]; then
    echo "Binary not found. Compiling..."

    export GOROOT=/home/container/go
    export PATH=$GOROOT/bin:$PATH

    if [ ! -d "/home/container/go" ]; then
        echo "Installing Go..."
        mkdir -p /home/container/go
        wget -q https://go.dev/dl/go1.23.6.linux-amd64.tar.gz -O /tmp/go.tar.gz
        tar -C /home/container -xzf /tmp/go.tar.gz
        rm /tmp/go.tar.gz
    fi

    CGO_ENABLED=0 go build -o tmr-bot .
    if [ $? -ne 0 ]; then
        echo "Build failed!"
        exit 1
    fi
    echo "Build successful!"
fi

if [ ! -f "./.env" ]; then
    echo ".env file not found!"
    exit 1
fi

chmod +x tmr-bot
./tmr-bot
