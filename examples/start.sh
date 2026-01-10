#!/bin/bash

export CHAIN_NETWORK="localhost"
export CHAIN_ID="1337"
export CHAIN_RPC="http://127.0.0.1:8545"
export TOKEN_CONTRACT="0xBA32c2Ee180e743cCe34CbbC86cb79278C116CEb"
export TOKEN_NAME="MyToken"
export TOKEN_VERSION="1"
export GATEWAY_URL="http://localhost:8080"
export RESOURCE_PATH="/premium-data"

export BUYER_PRIVATE_KEY=""

go run examples/buyer.go
