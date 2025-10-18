import { BigInt, cosmos, log, store, crypto, ByteArray, bigInt } from "@graphprotocol/graph-ts";
import { claimlist, merkellist, rewardlist } from "../generated/schema";


export function handleSetRewardList(data: cosmos.EventData): void {

  let list = new rewardlist("reward_list")
  list.list = data.event.getAttributeValue("reward_list").split(",");
  log.info("handle reward list successed ,list:{}", [data.event.getAttributeValue("reward_list")])
  list.save()
}


export function handleSetMerkelList(data: cosmos.EventData): void {

  let list = new merkellist(data.block.header.height.toString())
  list.root = data.event.getAttributeValue("merkel_root");
  list.timestamp = bigInt.fromString(data.block.header.time.seconds.toString())
  log.info("handle merker list successed ,list:{}", [data.event.getAttributeValue("merkel_root")])
  list.save()
}

export function handleClaimList(data: cosmos.EventData): void {

  let list = new claimlist(data.event.getAttributeValue("claim_address") + "-" + data.block.header.time.seconds.toString())
  list.address = data.event.getAttributeValue("claim_address");
  list.timestamp = bigInt.fromString(data.block.header.time.seconds.toString())
  list.status = data.event.getAttributeValue("claim_status");
  log.info("handle claim  successed ,address:{}", [data.event.getAttributeValue("claim_address")])
  list.save()
}