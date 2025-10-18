# rewards  manifest



## Deploy


```shell
$ yarn codegen && yarn build
$ yarn create-local && yarn deploy-local

```

For the list of supported networks, see the scripts in the [`package.json`](package.json) file.

## Querying the subgraph

### Query reward list
```graphql
 query{
    rewardlists{
      id
      list
    }
  }
```
The query result

```json
{
  "data": {
    "rewardlists": [
      {
        "id": "reward_list",
        "list": [
          "token1",
          "token2"
        ]
      }
    ]
  }
}
```

query by task id
  ```graphql
   query{
    rewardlists(where:{id:"reward_list"}){
      id
      list
    }
  }
```
the query result
```json
{
  "data": {
    "rewardlists": [
      {
        "id": "reward_list",
        "list": [
          "token1",
          "token2"
        ]
      }
    ]
  }
}
```
