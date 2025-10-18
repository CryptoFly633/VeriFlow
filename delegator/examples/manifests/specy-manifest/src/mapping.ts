import { cosmos, log } from "@graphprotocol/graph-ts";
import { task } from "../generated/schema";

/*
handle create-task event
*/
export function handleTaskEvent(data: cosmos.EventData): void {
  let newTask = new task(data.event.getAttributeValue("task_hash"));
  newTask.hash = data.event.getAttributeValue("task_hash")
  newTask.contract_address = data.event.getAttributeValue("task_contract_address")
  newTask.method = data.event.getAttributeValue("task_method")
  newTask.calldata = data.event.getAttributeValue("task_calldata")
  if (data.event.getAttributeValue("task_single") == "true") {
    newTask.single = true
  } else {
    newTask.single = false
  }
  newTask.rule_file = data.event.getAttributeValue("task_rule_file")
  newTask.creator = data.event.getAttributeValue("task_creator")
  log.info("Task handle success,hash:{}", [newTask.hash])
  newTask.save();
}
