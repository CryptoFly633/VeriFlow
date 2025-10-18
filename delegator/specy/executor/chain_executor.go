package executor

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"time"

	specyconfig "github.com/cosmos/relayer/v2/specy/config"
	specytypes "github.com/cosmos/relayer/v2/specy/types"
)

func CreateExecutorOnChain(iasReport string, enclavePK string) error {
	cmd := exec.Command("specyd", "tx", "specy", "create-executor",
		iasReport, enclavePK,
		"--from", specyconfig.Config.ChainInfo.ValidatorWalletAddress,
		"--chain-id", specyconfig.Config.ChainInfo.ChainId,
		"--home", specyconfig.Config.ChainInfo.HomeDir,
		"--node", specyconfig.Config.ChainInfo.NodeAddress,
		"--keyring-backend", "test", "--yes")

	fmt.Printf("---------------cmd--------------: %+v \n", cmd)

	_, err := executeCmd(cmd)
	return err
}

func SendTaskResponseToChain(msg string, cproof string, task *specytypes.Task) error {
	//taskResult := string(specyResp.Result.TaskResult)
	//taskResult := "FM2vKqiPHN0XCQ=="
	//taskResult, _ = decodeTaskResult(taskResult)
	//completeCalldata, err := assembleCalldata(task.RuleFile, taskResult)

	// 从 task 中获取 target chain name
	//targetChainName := task.xxxx
	targetChainName := "osmo-test-5"
	targetChainBinaryInfo, err := specyconfig.GetTargetChainConfig(targetChainName)
	if targetChainBinaryInfo == nil {
		return fmt.Errorf("target chain binary info is nil")
	}
	fmt.Printf("===========begin generate packet data============= target chain: %s binary location: %s \n", targetChainName, targetChainBinaryInfo.BinaryLocation)

	cmd := exec.Command(targetChainBinaryInfo.BinaryLocation, "tx", "interchain-accounts", "host",
		"generate-packet-data", msg, "--memo", "executing-task")

	output, err := executeCmd(cmd)
	if err != nil {
		return err
	}
	packetData, err := retrievePacketDataFromCmdOutput(output)
	fmt.Printf("===========finish generate packet data============= PacketDataOutput: %s \n", output)

	if err != nil {
		return err
	}

	//msg, err := specyChain.ChainProvider.MsgExecuteTask(task.Creator, task.Creator, task.TaskName, "cproofstring", packetData)
	//if err != nil {
	//	return err
	//}
	//specyChain.ChainProvider.SendMessage(ctx, msg, "")

	cmd = exec.Command("specyd", "tx", "specy", "execute-task",
		task.Creator, task.TaskName, cproof, packetData,
		"--from", specyconfig.Config.ChainInfo.ValidatorWalletAddress,
		"--chain-id", specyconfig.Config.ChainInfo.ChainId,
		"--home", specyconfig.Config.ChainInfo.HomeDir,
		"--node", specyconfig.Config.ChainInfo.NodeAddress,
		"--keyring-backend", "test", "--yes")

	fmt.Printf("===========begin execute on specy chain============= cmd: %+v \n", cmd)

	_, err = executeCmd(cmd)
	fmt.Println("===========finish execute on specy chain=============")

	return err
}

func retrievePacketDataFromCmdOutput(output string) (string, error) {
	return output, nil
}

//func SendTaskResponseToChainLegacy(specyResp specytypes.TaskResponse, task *specytypes.Task) error {
//	taskResult := string(specyResp.Result.TaskResult)
//	//taskResult := "FM2vKqiPHN0XCQ=="
//	//taskResult, _ = decodeTaskResult(taskResult)
//	//completeCalldata, err := assembleCalldata(task.RuleFile, taskResult)
//	_, err := assembleCalldata(task.RuleFile, taskResult)
//	if err != nil {
//		return err
//	}
//
//	cmd := exec.Command("specyd", "tx", "specy", "execute-task",
//		task.Creator, task.TaskName, string(specyResp.Signature), string(specyResp.Result.TaskResult),
//		"--from", task.Creator,
//		"--chain_id", specyconfig.Config.ChainInfo.ChainId,
//		//"--home", specyconfig.Config.SpecyInfo,
//		"--keyring-backend", "test", "--yes")
//
//	_, err = executeCmd(cmd)
//	return err
//}

func executeCmd(cmd *exec.Cmd) (string, error) {
	// 创建缓冲区来存储标准输出和标准错误输出
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// 将标准输出和标准错误输出连接到缓冲区
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// 执行命令
	err := cmd.Run()

	result := stdout.String()
	if err != nil {
		// 捕获到错误
		fmt.Println("执行命令出错:", err)
		if len(result) == 0 {
			result = stderr.String()
		}
		fmt.Println("执行结果:", result)
		return "", err
	}

	// 执行成功
	fmt.Println("命令执行完成")
	if len(result) == 0 {
		result = stderr.String()
	}
	fmt.Println("执行结果:", result)
	return result, nil
}

func assembleCalldata(calldata string, taskResult string) (string, error) {
	// 解析 JSON 字符串
	var data Data
	err := json.Unmarshal([]byte(calldata), &data)
	if err != nil {
		fmt.Println("解析 JSON 失败:", err)
		return "", err
	}

	// 根据 index 赋值
	if data.Index >= 0 && data.Index < len(data.Params) {
		data.Params[data.Index] = taskResult
	}

	// 提取 value 值
	values := make([]string, len(data.Params))
	currentTime := time.Now()
	truncatedTime := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 0, 0, 0, 0, currentTime.Location())
	timestamp := truncatedTime.Unix()
	values[0] = strconv.FormatInt(timestamp, 10)
	values[1] = taskResult

	var newData ExecuteData
	newData.Params = values
	newData.Index = data.Index

	jsonStr, err := json.Marshal(newData)
	fmt.Printf("jsonStr: %+v\n", string(jsonStr))

	return string(jsonStr), err
}

type Data struct {
	Params []string `json:"params"`
	Index  int      `json:"index"`
}

type ExecuteData struct {
	Params []string `json:"params"`
	Index  int      `json:"index"`
}

func decodeTaskResult(taskResult string) (string, error) {
	// 解码 Base64 字符串
	decoded, err := base64.StdEncoding.DecodeString(taskResult)
	if err != nil {
		fmt.Println("Base64 解码失败:", err)
		return "", err
	}

	// 将解码后的字节切片转换为十六进制字符串
	hexString := hex.EncodeToString(decoded)
	fmt.Println("转换后的十六进制字符串:", hexString)
	return hexString, err
}

func SendProofResponseToChain(txSpecResp specytypes.ProofResponse) error {
	jsonData, err := json.Marshal(txSpecResp.Proofs)
	if err != nil {
		fmt.Println("JSON encoding error:", err)
		return err
	}
	cmd := exec.Command("specyd", "tx", "regulatory", "submit-spec-value", string(txSpecResp.TxHash), string(jsonData), string(txSpecResp.ProofsHash), string(txSpecResp.TeeSignature))
	// 执行命令并获取输出
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	// 输出结果
	fmt.Println(string(output))
	return nil
}
