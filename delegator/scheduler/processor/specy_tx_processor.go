package processor

import (
	"context"
	"encoding/json"
	"fmt"
	specydispatcher "github.com/cosmos/relayer/v2/specy/executor"

	"log"
	"os/exec"
	"time"

	"github.com/cosmos/relayer/v2/utils"

	abci "github.com/cometbft/cometbft/abci/types"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	specytypes "github.com/cosmos/relayer/v2/specy/types"
)

func HandleTxWithTxSpec(
	ctx context.Context,
	tx *abci.ResponseDeliverTx,
	chainID string,
	height uint64,
	rpcclient rpcclient.Client,
	txIndex int,
) {

	events := utils.ParseBase64Events(tx.Events)
	fmt.Printf("-------------events: %+v \n", events)
	// check whether it's a regulatory contract. if yes, classify events
	cts, regFlag := classifyEventsByContract(ctx, events)
	if !regFlag {
		return
	}

	// query txHash
	txHash, err := queryTxHash(ctx, rpcclient, height, txIndex)
	if err != nil {
		log.Fatal(err)
		return
	}

	// find msgSender from events
	msgSender, err := findEventMsgSender(events)
	if err != nil {
		log.Fatal(err)
		return
	}

	// invoke specy
	txSpecResp, err := specydispatcher.InvokeEngineWithTx(ctx, cts, txHash, msgSender, chainID, height)
	if err != nil {
		log.Fatal(err)
		return
	}

	// invoke chain
	invokeChainWithTxSpecResponse(txSpecResp, cts[0].ContractID)
	if err != nil {
		log.Fatal(err)
		return
	}
}

func queryTxHash(ctx context.Context, client rpcclient.Client, height uint64, txIndex int) ([]byte, error) {
	i := int64(height)

	queryCtx, cancelQueryCtx := context.WithTimeout(ctx, 2*time.Minute)
	defer cancelQueryCtx()

	blockRes, err := client.Block(queryCtx, &i)
	txHash := blockRes.Block.Txs[txIndex].Hash()
	fmt.Printf("-------------txHash: %+v \n", txHash)

	if txHash == nil {
		return txHash, specytypes.ErrNotFindTxHash
	}
	return txHash, err
}

func invokeChainWithTxSpecResponse(txSpecResp specytypes.ProofResponse, contractAddress string) error {
	jsonData, err := json.Marshal(txSpecResp.Proofs)
	if err != nil {
		fmt.Println("JSON encoding error:", err)
		return err
	}
	cmd := exec.Command("specyd", "tx", "regulatory", "submit-spec-value", string(txSpecResp.TxHash), string(jsonData), string(txSpecResp.ProofsHash), string(txSpecResp.TeeSignature), contractAddress)
	// 执行命令并获取输出
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	// 输出结果
	fmt.Println(string(output))
	return nil
}

// 根据event 的contract进行分类
func classifyEventsByContract(ctx context.Context, events []sdk.StringEvent) ([]*specytypes.ContractEvent, bool) {
	contractEventMap := make(map[string]specytypes.ContractEvent)
	regFlag := false
	for _, event := range events {
		// 1.查找event的contract attribute，如果找不到，暂且认为该event不需要监管
		contractName := findEventContractName(event)
		if contractName == "" {
			continue
		}
		fmt.Printf("-------------contractName: %s \n", contractName)

		// 2.check regulatory relation contract name
		if !checkRegulatoryContractName(ctx, contractName) {
			continue
		}
		regFlag = true
		// 3.判断map中是否存在该contract对应的events对象
		contractEvent, ok := contractEventMap[contractName]
		if !ok {
			//不存在，创建新的
			contractEvent = specytypes.ContractEvent{ContractID: contractName, Events: []*specytypes.Event{}}
			contractEventMap[contractName] = contractEvent
		}
		contractEventItem := &specytypes.Event{EventName: event.Type}

		for _, attribute := range event.Attributes {
			contractEventItemAttribute := &specytypes.Attribute{
				Key:   attribute.Key,
				Value: attribute.Value,
			}
			contractEventItem.Attributes = append(contractEventItem.Attributes, contractEventItemAttribute)
		}
		contractEvent.Events = append(contractEvent.Events, contractEventItem)
		contractEventMap[contractName] = contractEvent
	}

	cts := make([]*specytypes.ContractEvent, len(contractEventMap))

	index := 0
	//map 转为数组
	for _, ce := range contractEventMap {
		cts[index] = &ce
		index++
	}

	return cts, regFlag
}

func checkRegulatoryContractName(ctx context.Context, contractName string) bool {
	fmt.Printf("-------------ctx: %+v \n", ctx.Value(specytypes.SpecyInfoKey))

	specyInfoMap := ctx.Value(specytypes.SpecyInfoKey).(map[string]any)
	contractNameDict := specyInfoMap[specytypes.ContractNameDictKey].(map[string]bool)
	return contractNameDict[contractName]
}

// 查找event的contract name
func findEventContractName(event sdk.StringEvent) string {
	contractName := ""
	for _, attr := range event.Attributes {
		if string(attr.Key) == specytypes.AttributeKeyContractAddress {
			contractName = attr.Value
		}
	}
	return contractName
}

func findEventMsgSender(events []sdk.StringEvent) ([]byte, error) {
	msgSender := ""

OuterLoop:
	for _, event := range events {
		for _, attr := range event.Attributes {
			switch attr.Key {
			case "message.sender":
				msgSender = attr.Value
				break OuterLoop
			default:
				continue
			}
		}
	}

	return []byte(msgSender), nil
}
