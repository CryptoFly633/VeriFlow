# specy  manifest



## Deploy


```shell
$ yarn codegen && yarn build
$ yarn create-local && yarn deploy-local

```

For the list of supported networks, see the scripts in the [`package.json`](package.json) file.

## Querying the subgraph

### Query task
```graphql
query{
    tasks{
      hash
      id
      rule_file
      creator
      single
      calldata
      contract_address
      method
    }
  }
```
The query result

```json
{
  "data": {
    "tasks": [
      {
        "hash": "66d266f553ff18e075ad06bd6e5a905831181e9328fd4e8474cf013563b6ed4b",
        "id": "66d266f553ff18e075ad06bd6e5a905831181e9328fd4e8474cf013563b6ed4b",
        "rule_file": "fsadfsafdsafdsafsaf",
        "creator": "cosmos1zaavvzxez0elundtn32qnk9lkm8kmcszzsv80v",
        "single": true,
        "calldata": "{\"params\":[\"1686553200\",\"a7dc6c23c3d0e2f14587f2096071858c0d52957d8a2117e5dd4ada522fa742cf\"],\"index\":1}",
        "contract_address": "rewards",
        "method": "SetRewards"
      }
    ]
  }
}
```

query by task hash

```graphql
 query{
    tasks(where:{hash:"66d266f553ff18e075ad06bd6e5a905831181e9328fd4e8474cf013563b6ed4b"}){
      hash
      id
      rule_file
      creator
      single
      calldata
      contract_address
      method
    }
  }
  ```
also can query by id
  ```graphql
   query{
    tasks(where:{id:"66d266f553ff18e075ad06bd6e5a905831181e9328fd4e8474cf013563b6ed4b"}){
      hash
      id
      rule_file
      creator
      single
      calldata
      contract_address
      method
    }
  }
```
the query result
```json
{
  "data": {
    "tasks": [
      {
        "hash": "66d266f553ff18e075ad06bd6e5a905831181e9328fd4e8474cf013563b6ed4b",
        "id": "66d266f553ff18e075ad06bd6e5a905831181e9328fd4e8474cf013563b6ed4b",
        "rule_file": "fsadfsafdsafdsafsaf",
        "creator": "cosmos1zaavvzxez0elundtn32qnk9lkm8kmcszzsv80v",
        "single": true,
        "calldata": "{\"params\":[\"1686553200\",\"a7dc6c23c3d0e2f14587f2096071858c0d52957d8a2117e5dd4ada522fa742cf\"],\"index\":1}",
        "contract_address": "rewards",
        "method": "SetRewards"
      }
    ]
  }
}
```
