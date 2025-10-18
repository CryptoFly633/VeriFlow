#!/bin/bash -e
RELAYER_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
RELAYER_CONF="$HOME/.relayer"
GAIA_CONF="$(pwd)/data"

# Ensure user understands what will be deleted
if ([[ -d $RELAYER_CONF ]] || [[ -d $GAIA_CONF ]]) && [[ ! "$1" == "skip" ]]; then
  read -p "$0 will delete \$HOME/.relayer and \$(pwd)/data folder. Do you wish to continue? (y/n): " -n 1 -r
  echo
  if [[ ! $REPLY =~ ^[Yy]$ ]]; then
      exit 1
  fi
fi

cd $RELAYER_DIR
rm -rf $RELAYER_CONF &> /dev/null
pwd
# spin up two ibc enabled chains and add relevant config info for relaying
bash "scripts/two-chainz.sh" "skip"

echo "waiting for blocks..."
sleep 3

# creates clients, connections and channels (all this happens in rly tx command)
rly tx link demo --src-port nft-transfer --dst-port nft-transfer -v ics721-1 -d -t 3s
sleep 2
echo "--  Initial balances --"
echo "balance 0 $(rly q bal ibc-2)"
echo "balance 1 $(rly q bal ibc-1)"

rly start demo