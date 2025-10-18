#!/bin/bash
# two-chainz creates two iris chains and configures the relayer to 

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
CHAIN_DATA="$(pwd)/data"
RELAYER_DIR="$(dirname $SCRIPTDIR)"
RELAYER_CONF="$HOME/.relayer"

# Ensure iris is installed
if ! [ -x "$(which iris)" ]; then
  echo "Error: iris is not installed. Try running 'make build-gaia'" >&2
  exit 1
fi

# Display software version for testers
echo "IRIS VERSION INFO:"
iris version --long

# Ensure jq is installed
if [[ ! -x "$(which jq)" ]]; then
  echo "jq (a tool for parsing json in the command line) is required..."
  echo "https://stedolan.github.io/jq/download/"
  exit 1
fi

# Ensure user understands what will be deleted
if [[ -d $CHAIN_DATA ]] && [[ ! "$1" == "skip" ]]; then
  read -p "$(basename $0) will delete \$(pwd)/data and \$HOME/.relayer folders. Do you wish to continue? (y/n): " -n 1 -r
  echo
  if [[ ! $REPLY =~ ^[Yy]$ ]]; then
      exit 1
  fi
fi

# Delete data from old runs
rm -rf $CHAIN_DATA &> /dev/null
rm -rf $RELAYER_CONF &> /dev/null

# Stop existing iris processes
killall iris &> /dev/null

set -e

chainid0=ibc-2
chainid1=ibc-1

echo "Generating gaia configurations..."
mkdir -p $CHAIN_DATA && cd $CHAIN_DATA && cd ../
bash "./scripts/one-chain.sh" iris $chainid0 ./data 26657 26656 6060 9090 iris uiris 8545 8546
bash "./scripts/one-chain.sh" iris $chainid1 ./data 26557 26556 6061 9091 iris uiris 8535 8536

[ -f $CHAIN_DATA/$chainid0.log ] && echo "$chainid0 initialized. Watch file $CHAIN_DATA/$chainid0.log to see its execution."
[ -f $CHAIN_DATA/$chainid1.log ] && echo "$chainid1 initialized. Watch file $CHAIN_DATA/$chainid1.log to see its execution."

# cd $RELAYER_DIR

# echo "Building Relayer..."
# make -C ../../ install

pwd

echo "Generating rly configurations..."
rly config init
rly chains add-dir configs/chains

SEED0=$(jq -r '.mnemonic' $CHAIN_DATA/ibc-2/key_seed.json)
SEED1=$(jq -r '.mnemonic' $CHAIN_DATA/ibc-1/key_seed.json)
echo "Key $(rly keys restore ibc-2 testkey "$SEED0") imported from ibc-2 to relayer..."
echo "Key $(rly keys restore ibc-1 testkey "$SEED1") imported from ibc-1 to relayer..."

rly paths add-dir configs/paths
