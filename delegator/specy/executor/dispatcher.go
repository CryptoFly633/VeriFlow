package executor

import (
	"encoding/json"
	"fmt"
	"log"

	specytypes "github.com/cosmos/relayer/v2/specy/types"
)

func ExecuteTask(task *specytypes.Task) {

	// invoke specy engine
	engineOutput, err := InvokeEngineWithTask(task)
	//engineOutput, err := mockEngine(task)
	if err != nil {
		log.Fatal(err)
		return
	}
	fmt.Printf("engineOutput: %+v \n", engineOutput)
	if !engineOutput.Status {
		fmt.Println("task is not executed: not meet checkdata condition")
		return
	}
	if len(engineOutput.ErrorInfo) != 0 {
		fmt.Errorf("engineOutput error info: %s", engineOutput.ErrorInfo)
		return
	}

	executeMsg, err := AssembleExecuteMsgWithEngineOutput(task.Msg, engineOutput.OutputData)
	if err != nil {
		fmt.Errorf("failed assemble execute msg: %s", err)
		return
	}

	// send task response to chain
	err = SendTaskResponseToChain(executeMsg, engineOutput.Cproof, task)
	if err != nil {
		fmt.Errorf("SendTaskResponseToChain error: %s", err)
		return
	}
}

func mockEngine(task *specytypes.Task) (string, error) {
	return task.Msg, nil
}

func AssembleExecuteMsgWithEngineOutput(taskMsg string, engineOutput string) (string, error) {
	var taskMsgData map[string]interface{}
	var engineOutputData map[string][]map[string]interface{}

	err := json.Unmarshal([]byte(taskMsg), &taskMsgData)
	if err != nil {
		return "", fmt.Errorf("failed parsing task msg: %s", err)
	}

	err = json.Unmarshal([]byte(engineOutput), &engineOutputData)
	if err != nil {
		return "", fmt.Errorf("failed parsing engine output: %s", err)
	}
	if len(engineOutputData["outputdata"]) != 0 {
		recursiveMerge(taskMsgData, engineOutputData["outputdata"][0])
	}

	executeMsgData, err := json.Marshal(taskMsgData)
	if err != nil {
		return "", fmt.Errorf("failed marshaling execute msg data: %s", err)
	}
	executeMsg := string(executeMsgData)

	fmt.Println("execute msg:", executeMsg)
	return executeMsg, nil
}

func recursiveMerge(target, source map[string]interface{}) {
	for key, sourceValue := range source {
		// 检查目标中是否存在相同的键
		_, exists := target[key]
		if !exists {
			for _, targetValue := range target {
				// 如果字段是 map 类型，递归合并
				if targetMap, isMap := targetValue.(map[string]interface{}); isMap {
					recursiveMerge(targetMap, source)
				}
			}
		} else {
			// 如果目标中没有这个键，直接添加
			target[key] = sourceValue
			break
		}
		sourceMap, isMap := sourceValue.(map[string]interface{})
		if isMap {
			recursiveMerge(target, sourceMap)
		}
	}
}
