#!/bin/bash

cd /home/container

if [ ! -f "./tmr-bot" ]; then
    echo "Binary not found. Compiling..."

    if [ ! -d "./go" ]; then
        echo "Installing Go..."
        mkdir -p /home/container/go-install
        wget -q https://go.dev/dl/go1.23.6.linux-amd64.tar.gz -O /home/container/go-install/go.tar.gz
        tar -C /home/container/go-install -xzf /home/container/go-install/go.tar.gz
        export GOROOT=/home/container/go-install/go
        export PATH=$PATH:/home/container/go-install/go/bin
    else
        export GOROOT=/home/container/go/bin/go
        export PATH=$PATH:/home/container/go/bin
    fi

    CGO_ENABLED=0 go build -o tmr-bot .
    if [ $? -ne 0 ]; then
        echo "Build failed!"
        exit 1
    fi
    rm -rf /home/container/go-install
    echo "Build successful!"
fi

if [ ! -f "./.env" ]; then
    echo ".env file not found!"
    exit 1
fi

chmod +x tmr-bot
./tmr-bot
