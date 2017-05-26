#!/bin/bash

/geth \
    --datadir "/eth" \
	init "/eth/genesis.json"

/geth \
    --identity "$IDENTITY" \
	--rpc \
    --rpcport "8545" \
	--rpccorsdomain "*" \
	--datadir "/eth" \
	--port "$PORT" \
    --rpcapi "db,eth,net,web3" \
    --networkid "20160816" \
    --nat "any" \
    --nodekeyhex "$NODEKEY" \
    --bootnodes "$BOOTNODES" \
    --mine \
    --minerthreads 1 \
    --debug
