#!/bin/bash

/geth \
    --datadir "/eth" \
	init "/eth/genesis.json"

/geth \
    --identity "$IDENTITY" \
    --rpc \
    --rpcaddr "0.0.0.0" \
    --rpcport "8545" \
    --rpccorsdomain "*" \
    --datadir "/eth" \
    --port "$PORT" \
    --rpcapi "db,eth,net,web3" \
    --networkid "20160816" \
    --nat "any" \
    --nodekeyhex "$NODEKEY" \
    --bootnodes "$BOOTNODES" \
    --targetgaslimit $GAS_LIMIT \
    --txpool.globalslots $GLOBAL_SLOTS \
    --txpool.accountslots $ACCOUNT_SLOTS \
    --txpool.globalqueue $GLOBAL_QUEUE \
    --txpool.accountqueue $ACCOUNT_QUEUE \
    --mine \
    --minerthreads 1 \
    --debug \
    --metrics \
    --promaddr "$PROMETHEUS_ADDR"
