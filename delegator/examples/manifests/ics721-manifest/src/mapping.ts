import { BigInt, cosmos, log } from "@graphprotocol/graph-ts";
import { interchainnftreceive } from "../generated/schema";

/*
处理transfer类event
*/
export function handleTransferEvent(data: cosmos.EventData, txHash: string): void {
  const sender = data.event.getAttributeValue("sender");
  const receiver = data.event.getAttributeValue("receiver");
  let receive = new interchainnftreceive(txHash);
  receive.id = txHash;
  receive.hash = txHash;
  receive.sender = sender;
  receive.receiver = receiver;
  receive.class_id = data.event.getAttributeValue("classID");
  receive.token_id = data.event.getAttributeValue("tokenID");
  receive.timestamp = BigInt.fromString(
    data.block.header.time.seconds.toString()
  );
  receive.contract_address = data.event.getAttributeValue("contract_address");
  receive.save();
}

/*
交易处理逻辑，由于cosmos中event无法获取合约地址和交易hash等信息，故此处由txhandle统一处理所有event
之后根据event类型交由具体handle处理
*/
export function handleTx(data: cosmos.TransactionData): void {
  const txHash = data.tx.hash.toHexString();
  const events = data.tx.result.events;
  for (let index = 0; index < events.length; index++) {
    const event: cosmos.Event = events[index];
    let hob = new cosmos.HeaderOnlyBlock(data.block.header);
    let ed = new cosmos.EventData(event, hob);
    if (event.eventType == "ibc_nft_receive") {
      handleTransferEvent(ed, txHash);
      log.info("收到ibc nft 消息:{}", [txHash])
    }
  }
}

