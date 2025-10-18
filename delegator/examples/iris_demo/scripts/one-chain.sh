#!/bin/bash

display_usage() {
	echo "\nMissing $1 parameter. Please check if all parameters were specified."
	echo "\nUsage: ./one-chain [BINARY] [CHAIN_ID] [CHAIN_DIR] [RPC_PORT] [P2P_PORT] [PROFILING_PORT] [GRPC_PORT]"
  echo "\nExample: ./one-chain $BINARY test-chain-id ./data 26657 26656 6060 9090 \n"
  exit 1
}

KEYRING=--keyring-backend="test"
SILENT=1


redirect() {
  if [ "$SILENT" -eq 1 ]; then
    "$@" > /dev/null 2>&1
  else
    "$@"
  fi
}

BINARY=$1
CHAINID=$2
CHAINDIR=$3
RPCPORT=$4
P2PPORT=$5
PROFPORT=$6
GRPCPORT=$7
DENOM=$8
BASEDENOM=$9
EVMADDRESS=${10}
EVMWSADDRESS=${11}

if [ -z "$1" ]; then
  display_usage "[BINARY] ($BINARY|akash)"
fi

if [ -z "$2" ]; then
  display_usage "[CHAIN_ID]"
fi

if [ -z "$3" ]; then
  display_usage "[CHAIN_DIR]"
fi

if [ -z "$4" ]; then
  display_usage "[RPC_PORT]"
fi

if [ -z "$5" ]; then
  display_usage "[P2P_PORT]"
fi

if [ -z "$6" ]; then
  display_usage "[PROFILING_PORT]"
fi

if [ -z "$7" ]; then
  display_usage "[GRPC_PORT]"
fi
if [ -z "$10" ]; then
  display_usage "[EVM_ADDRESS]"
fi
if [ -z "$11" ]; then
  display_usage "[EVM_WS_ADDRESS]"
fi

echo "Creating $BINARY instance: home=$CHAINDIR | chain-id=$CHAINID | p2p=:$P2PPORT | rpc=:$RPCPORT | profiling=:$PROFPORT | grpc=:$GRPCPORT | evm=:$EVMADDRESS | evm-ws=:$EVMWSADDRESS"

# Add dir for chain, exit if error
if ! mkdir -p $CHAINDIR/$CHAINID 2>/dev/null; then
    echo "Failed to create chain folder. Aborting..."
    exit 1
fi

# Build genesis file incl account for passed address
coins="100000000000$DENOM,100000000000$BASEDENOM"
delegate="100000000000$DENOM"

redirect $BINARY --home $CHAINDIR/$CHAINID --chain-id $CHAINID init $CHAINID 
sleep 1
$BINARY --home $CHAINDIR/$CHAINID keys add validator $KEYRING --output json > $CHAINDIR/$CHAINID/validator_seed.json 2>&1
sleep 1
$BINARY --home $CHAINDIR/$CHAINID keys add user $KEYRING --output json > $CHAINDIR/$CHAINID/key_seed.json 2>&1
sleep 1
redirect $BINARY --home $CHAINDIR/$CHAINID add-genesis-account $($BINARY --home $CHAINDIR/$CHAINID keys $KEYRING show user -a) $coins 
sleep 1
redirect $BINARY --home $CHAINDIR/$CHAINID add-genesis-account $($BINARY --home $CHAINDIR/$CHAINID keys $KEYRING show validator -a) $coins 
sleep 1
redirect $BINARY --home $CHAINDIR/$CHAINID gentx validator $delegate $KEYRING --chain-id $CHAINID
sleep 1
redirect $BINARY --home $CHAINDIR/$CHAINID collect-gentxs 
sleep 1

# Check platform
platform='unknown'
unamestr=`uname`
if [ "$unamestr" = 'Linux' ]; then
   platform='linux'
fi

# Set proper defaults and change ports (use a different sed for Mac or Linux)
echo "Change settings in config.toml and genesis.json files..."
if [ $platform = 'linux' ]; then
  sed -i 's#"tcp://127.0.0.1:26657"#"tcp://0.0.0.0:'"$RPCPORT"'"#g' $CHAINDIR/$CHAINID/config/config.toml
  sed -i 's#"tcp://0.0.0.0:26656"#"tcp://0.0.0.0:'"$P2PPORT"'"#g' $CHAINDIR/$CHAINID/config/config.toml
  sed -i 's#"0.0.0.0:8545"#"0.0.0.0:'"$EVMADDRESS"'"#g' $CHAINDIR/$CHAINID/config/app.toml
    sed -i 's#"0.0.0.0:8546"#"0.0.0.0:'"$EVMWSADDRESS"'"#g' $CHAINDIR/$CHAINID/config/app.toml
  sed -i 's#"localhost:6060"#"localhost:'"$P2PPORT"'"#g' $CHAINDIR/$CHAINID/config/config.toml
  sed -i 's/timeout_commit = "5s"/timeout_commit = "1s"/g' $CHAINDIR/$CHAINID/config/config.toml
  sed -i 's/timeout_propose = "3s"/timeout_propose = "1s"/g' $CHAINDIR/$CHAINID/config/config.toml
  sed -i 's/index_all_keys = false/index_all_keys = true/g' $CHAINDIR/$CHAINID/config/config.toml
  sed -i 's/"bond_denom": "stake"/"bond_denom": "'"$DENOM"'"/g' $CHAINDIR/$CHAINID/config/genesis.json
  sed -i 's/"voting_period": "172800s"/"voting_period": "180s"/g' $CHAINDIR/$CHAINID/config/genesis.json
  sed -i 's/"stake"/"uiris"/g' $CHAINDIR/$CHAINID/config/genesis.json
  # sed -i '' 's#index-events = \[\]#index-events = \["message.action","send_packet.packet_src_channel","send_packet.packet_sequence"\]#g' $CHAINDIR/$CHAINID/config/app.toml
else
  sed -i '' 's#"tcp://127.0.0.1:26657"#"tcp://0.0.0.0:'"$RPCPORT"'"#g' $CHAINDIR/$CHAINID/config/config.toml
  sed -i '' 's#"tcp://0.0.0.0:26656"#"tcp://0.0.0.0:'"$P2PPORT"'"#g' $CHAINDIR/$CHAINID/config/config.toml
  sed -i '' 's#"localhost:6060"#"localhost:'"$P2PPORT"'"#g' $CHAINDIR/$CHAINID/config/config.toml
  sed -i '' 's/timeout_commit = "5s"/timeout_commit = "1s"/g' $CHAINDIR/$CHAINID/config/config.toml
  sed -i '' 's/timeout_propose = "3s"/timeout_propose = "1s"/g' $CHAINDIR/$CHAINID/config/config.toml
  sed -i '' 's/index_all_keys = false/index_all_keys = true/g' $CHAINDIR/$CHAINID/config/config.toml
  sed -i '' 's/"bond_denom": "stake"/"bond_denom": "'"$DENOM"'"/g' $CHAINDIR/$CHAINID/config/genesis.json
  # sed -i '' 's#index-events = \[\]#index-events = \["message.action","send_packet.packet_src_channel","send_packet.packet_sequence"\]#g' $CHAINDIR/$CHAINID/config/app.toml
fi

if [ $CHAINID = 'ibc-2' ]; then
cat << END >> $CHAINDIR/$CHAINID/config/config.toml

#######################################################
###       Extractor Configuration Options     ###
#######################################################
[extractor]
enabled = true
output_file = "stdout"
END


REGULATORYD=`which iris`

if [ ! -f "firehose.yml" ]; then
  cat << END >> $CHAINDIR/firehose.yml
start:
  args:
    - ingestor
    - merger
    - firehose
  flags:
    common-first-streamable-block: 1
    common-blockstream-addr: localhost:9000
    ingestor-mode: node
    ingestor-node-path: $REGULATORYD
    ingestor-node-args: start --x-crisis-skip-assert-invariants --home $CHAINID start --pruning=nothing --grpc-web.enable=false "
    ingestor-node-logs-filter: "module=(p2p|pex|consensus|x/bank)"
    firehose-real-time-tolerance: 99999h
    relayer-max-source-latency: 99999h
    verbose: 1
END
fi

fh_bin="firehose-cosmos"

if [[ -z $(which $fh_bin || true) ]]; then
  echo "You must install the firehose-binary first. See README for instructions"
  exit 1
fi
pushd data
echo "Starting firehose"

$fh_bin start > $CHAINID.log 2>&1 &
popd

else

# Start the gaia
$BINARY --home $CHAINDIR/$CHAINID start --pruning=nothing --grpc-web.enable=false --grpc.address="0.0.0.0:$GRPCPORT" > $CHAINDIR/$CHAINID.log 2>&1 &
fi