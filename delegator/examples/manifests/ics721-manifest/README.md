# ics 721  manifest



## Deploy


```shell
$ yarn codegen && yarn build
$ yarn create-local && yarn deploy-local

```

For the list of supported networks, see the scripts in the [`package.json`](package.json) file.

## Querying the subgraph

### Query transfers
```
query{
  interchainnftreceives{
    id
    hash
    sender
    receiver
    class_id
    token_id
    timestamp
    contract_address
  }
}
```
The query result

```
{
  "data": {
    "interchainnftreceives": [
      {
        "id": "0x36c94b4355d27e8656ec587c73bcfb691484e3378f61d1098eb87af8609bbb79",
        "Sender": "reg1zaavvzxez0elundtn32qnk9lkm8kmcszydc6a7",
        "Receiver": "reg185fflsvwrz0cx46w6qada7mdy92m6kx4wajjpk",
        "Value": "1000",
        "TokenName": "stake",
        "ContractAddress": "bank",
        "Timestamp": "1672992870"
      },
      {
        "id": "0x49a3a3053f88a9d66e17a5c52d47bf0a1e86bf6cecc7b124d108b09bb88ff9ff",
        "Sender": "reg1zaavvzxez0elundtn32qnk9lkm8kmcszydc6a7",
        "Receiver": "reg185fflsvwrz0cx46w6qada7mdy92m6kx4wajjpk",
        "Value": "1000",
        "TokenName": "stake",
        "ContractAddress": "bank",
        "Timestamp": "1672993040"
      }
    ]
  }
}
```

query by transaction hash

```
query{
  transfers(where:{Hash:"0x36c94b4355d27e8656ec587c73bcfb691484e3378f61d1098eb87af8609bbb79"}){
    id
    Sender
    Receiver
    Value
    TokenName
    ContractAddress
    Timestamp
  }
}
```
the query result
```
{
  "data": {
    "transfers": [
      {
        "id": "0x36c94b4355d27e8656ec587c73bcfb691484e3378f61d1098eb87af8609bbb79",
        "Sender": "reg1zaavvzxez0elundtn32qnk9lkm8kmcszydc6a7",
        "Receiver": "reg185fflsvwrz0cx46w6qada7mdy92m6kx4wajjpk",
        "Value": "1000",
        "TokenName": "stake",
        "ContractAddress": "bank",
        "Timestamp": "1672992870"
      }
    ]
  }
}
```

query one  by id

Notice：use transfer not transfers
```
query{
  transfer(id:"0x36c94b4355d27e8656ec587c73bcfb691484e3378f61d1098eb87af8609bbb79"){
    id
    Sender
    Receiver
    Value
    TokenName
    ContractAddress
    Timestamp
  }
}
```

query SuspiciousAccountList by contract_address and account address

```
query{
  suspiciousAccountLists(where:{contractaddress:"ERC20",account:"address2"}){
    contractaddress
    account
  }
}
```