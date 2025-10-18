package processor

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"

	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/relayer/v2/specy"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	specytypes "github.com/cosmos/relayer/v2/specy/types"
	geth "github.com/ethereum/go-ethereum/ethclient"

	"regexp"
	"time"

	"github.com/cosmos/relayer/v2/utils"
)

func HandleEventWithSpecy(
	events []abci.Event,
	base64Encoded bool,
) {

	for _, event := range events {
		var evt sdk.StringEvent
		if base64Encoded {
			evt = utils.ParseBase64Event(event)
		} else {
			evt = sdk.StringifyEvent(event)
		}

		switch evt.Type {
		case "create_task":
			registerTaskOnScheduler(evt)

		case "cancle_task":
			unregisterTaskOnScheduler(evt)

		case "execute_task":
			//监听到任务执行事件，由调度器根据task内容生成调用eth的交易，模拟跨链执行过程
			executeTaskOnScheduler(evt)

			// listen regulatory events and init or update info
			//case "register":
			//case "relation":
		}
	}
}

func executeTaskOnScheduler(evt sdk.StringEvent) {
	fmt.Printf("executeTaskOnScheduler event: %+v \n", evt)

	//获取event 中的msg字段，其包含合约地址，方法，calldata
	var msg string
	for _, attr := range evt.Attributes {
		if attr.Key == "task_msgs" {
			msg = attr.Value
		}
	}
	fmt.Println(msg)

	//调用eth client 执行交易

	var req Request
	if err := json.Unmarshal([]byte(msg), &req); err != nil {
		fmt.Fprintln(os.Stderr, "bad json:", err)
		os.Exit(1)
	}
	if req.Contract == "" || req.Function == "" {
		fmt.Fprintln(os.Stderr, "json must include contract & function")
		os.Exit(1)
	}

	// 收集类型与值
	typeStrs := make([]string, 0, len(req.Inputs))
	vals := make([]interface{}, 0, len(req.Inputs))
	for _, in := range req.Inputs {
		typeStrs = append(typeStrs, in.Type)
		v, err := coerceValue(in.Type, in.Value)
		if err != nil {
			fmt.Fprintf(os.Stderr, "parse input %q: %v\n", in.Name, err)
			os.Exit(1)
		}
		vals = append(vals, v)
	}

	// 构造 calldata（无 ABI 文件）
	calldata, err := buildCalldata(req.Function, typeStrs, vals)
	if err != nil {
		fmt.Fprintln(os.Stderr, "build calldata:", err)
		os.Exit(1)
	}

	// 发送交易
	ctx := context.Background()
	to := common.HexToAddress(req.Contract)
	txHash, err := sendTx(ctx, DEFAULT_RPC, to, calldata)
	if err != nil {
		fmt.Fprintln(os.Stderr, "send tx:", err)
		os.Exit(1)
	}
	fmt.Println(txHash)

}

func registerTaskOnScheduler(evt sdk.StringEvent) {
	// 入参打印
	fmt.Printf("registerTaskOnScheduler event: %+v \n", evt)

	var creator string
	var taskName string
	var taskHash string
	var connectionId string
	var msg string
	var ruleFile string
	var taskType = "0"
	var intervalType = "0"
	var interval = 1
	var startTime time.Time
	var checkData string

	for _, attr := range evt.Attributes {
		switch attr.Key {
		case "creator":
			creator = attr.Value
		case "task_name":
			taskName = attr.Value
		case "task_hash":
			taskHash = attr.Value
		case "connect_id":
			connectionId = attr.Value
		case "task_msgs":
			msg = attr.Value
		case "task_rule_file":
			ruleFile = attr.Value
			reg := regexp.MustCompile(`after (\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\+\d{2}:\d{2})`)
			match := reg.FindStringSubmatch(ruleFile)

			if len(match) > 1 {
				// ruleFile 中指定了执行时间的情况
				dateTimeStr := match[1]
				// 解析
				startTime = parseAndCalcStartTime(dateTimeStr)
				// 执行间隔默认是24小时
				if interval == 0 {
					interval = 24 * 60 * 60
				}
			} else {
				// ruleFile 中没有指定执行时间 默认是现在开始(延迟30s)
				startTime = time.Now().Add(30 * time.Second)
			}
		//case "task_type":
		//	taskType = attr.Value
		//case "task_interval_type":
		//	intervalType = attr.Value
		//case "task_interval_number":
		//	interval, _ = strconv.Atoi(attr.Value)
		case "task_check_data":
			checkData = attr.Value
		default:
			continue
		}
	}

	// 注册任务
	task := specytypes.NewTask(taskHash, taskName, creator, connectionId, msg, ruleFile, taskType, intervalType, interval, startTime, checkData)
	specy.RegisterTask(task)
}

func parseAndCalcStartTime(dateTimeStr string) time.Time {
	fmt.Println(dateTimeStr)

	// 将字符串转换为 time.Time 类型
	dateTime, err := time.Parse("2006-01-02T15:04:05-07:00", dateTimeStr)
	if err != nil {
		fmt.Println("日期时间解析失败:", err)
	} else {
		fmt.Println(dateTime)
	}

	now := time.Now()
	var startTime time.Time
	if dateTime.Before(now) || dateTime.Equal(now) {
		startTime = time.Date(now.Year(), now.Month(), now.Day(), dateTime.Hour(), dateTime.Minute(), dateTime.Second(), 0, now.Location())
	} else {
		startTime = dateTime
	}
	if startTime.Before(now) || startTime.Equal(now) {
		startTime = startTime.AddDate(0, 0, 1)
	}

	return startTime
}

func unregisterTaskOnScheduler(evt sdk.StringEvent) {
	var taskHash string
	for _, attr := range evt.Attributes {
		switch attr.Key {
		case "task_hash":
			taskHash = attr.Value
		}
	}

	// 取消注册任务
	specy.UnregisterTask(taskHash)
}

// ===== 固定配置（按需修改） =====
const (
	DEFAULT_RPC       = "http://127.0.0.1:8545"
	PRIVATE_KEY_HEX   = "0x69b4a7f9c11050565af2f026cca58e4b8becfd8046d94b02fa8182a265c8b747" // <<< 写死的私钥（带0x）
	GAS_MULTIPLIER_BP = 12000                                                                // 估算gas后放大系数(1.2x=12000BP)
)

// ===== JSON 结构（字段固定） =====
type Input struct {
	Name  string      `json:"name"`
	Type  string      `json:"type"`  // 例如: "string","uint256","address","bytes32","bytes","uint256[]"
	Value interface{} `json:"value"` // 可是字符串/数组；数字建议用字符串表示十进制大数
}
type Request struct {
	Contract string  `json:"contract"`
	Function string  `json:"function"` // 函数名，如 setPrice
	Inputs   []Input `json:"inputs"`
}

// ===== 工具：解析输入类型与值（不依赖ABI文件） =====
func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func toBigInt(v string) (*big.Int, error) {
	if v == "" {
		return big.NewInt(0), nil
	}
	bi, ok := new(big.Int).SetString(v, 10)
	if !ok {
		return nil, fmt.Errorf("bad big int: %s", v)
	}
	return bi, nil
}

func hexToBytes(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		return common.FromHex(s), nil
	}
	// 允许不带0x的纯hex
	return common.FromHex("0x" + s), nil
}

// 将 type 字符串转 abi.Type（无需ABI文件）
func buildArguments(typeStrs []string) (abi.Arguments, error) {
	args := abi.Arguments{}
	for _, t := range typeStrs {
		tt := strings.TrimSpace(t)
		typ, err := abi.NewType(tt, "", nil)
		if err != nil {
			return nil, fmt.Errorf("unsupported solidity type %q: %w", tt, err)
		}
		args = append(args, abi.Argument{Type: typ})
	}
	return args, nil
}

// 将 JSON 的 value 转为与 type 匹配的 Go 值
func coerceValue(solType string, raw interface{}) (interface{}, error) {
	lt := strings.ToLower(strings.TrimSpace(solType))

	// 动态/定长数组：xxx[]
	if strings.HasSuffix(lt, "[]") {
		elem := strings.TrimSuffix(lt, "[]")
		// 允许 value 是 "a,b,c" 或 JSON 数组
		switch vv := raw.(type) {
		case string:
			items := splitCSV(vv)
			out := make([]interface{}, 0, len(items))
			for _, it := range items {
				v, err := coerceValue(elem, it)
				if err != nil {
					return nil, err
				}
				out = append(out, v)
			}
			return out, nil
		case []interface{}:
			out := make([]interface{}, 0, len(vv))
			for _, it := range vv {
				v, err := coerceValue(elem, it)
				if err != nil {
					return nil, err
				}
				out = append(out, v)
			}
			return out, nil
		default:
			return nil, fmt.Errorf("array value must be string(csv) or []any; got %T", raw)
		}
	}

	// 标量

	switch lt {
	case "string":
		switch v := raw.(type) {
		case string:
			return v, nil
		default:
			return nil, fmt.Errorf("string expects string, got %T", raw)
		}
	case "address":
		s, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("address expects string")
		}
		if !common.IsHexAddress(s) {
			return nil, fmt.Errorf("invalid address: %s", s)
		}
		return common.HexToAddress(s), nil
	case "bool":
		s, ok := raw.(string)
		if ok {
			sl := strings.ToLower(s)
			return sl == "true" || sl == "1", nil
		}
		if b, ok := raw.(bool); ok {
			return b, nil
		}
		return nil, fmt.Errorf("bool expects string/bool, got %T", raw)
	case "bytes":
		s, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("bytes expects hex string")
		}
		return hexToBytes(s)
	case "bytes32":
		s, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("bytes32 expects hex string")
		}
		b, err := hexToBytes(s)
		if err != nil {
			return nil, err
		}
		if len(b) > 32 {
			return nil, errors.New("bytes32 too long (>32)")
		}
		var arr [32]byte
		copy(arr[:], b)
		return arr, nil
	default:
		// int/uint
		if strings.HasPrefix(lt, "uint") || strings.HasPrefix(lt, "int") {
			switch v := raw.(type) {
			case string:
				return toBigInt(v)
			case float64:
				return big.NewInt(int64(v)), nil // 不推荐：大数会丢精度。建议总用字符串。
			default:
				return nil, fmt.Errorf("%s expects string number, got %T", solType, raw)
			}
		}
	}

	return nil, fmt.Errorf("unsupported type in demo: %s", solType)
}

// 生成函数签名字符串："fn(t1,t2,...)"
func makeSignature(fn string, typeStrs []string) string {
	return fmt.Sprintf("%s(%s)", strings.TrimSpace(fn), strings.Join(typeStrs, ","))
}

// 组装 calldata：4字节选择器 + 编码参数
func buildCalldata(fn string, types []string, values []interface{}) ([]byte, error) {
	// 1) 计算 4 字节 selector
	sig := makeSignature(fn, types)
	selector := crypto.Keccak256([]byte(sig))[:4]

	// 2) 对参数进行 ABI 编码（不需要ABI文件；仅用 abi.Arguments）
	args, err := buildArguments(types)
	if err != nil {
		return nil, err
	}
	enc, err := args.Pack(values...)
	if err != nil {
		return nil, fmt.Errorf("abi pack: %w", err)
	}

	return append(selector, enc...), nil
}

// ====== 交易发送 ======
func loadPK(hex string) (*ecdsa.PrivateKey, common.Address, error) {
	hex = strings.TrimSpace(hex)
	if hex == "" {
		return nil, common.Address{}, errors.New("empty private key")
	}
	pk, err := crypto.HexToECDSA(strings.TrimPrefix(hex, "0x"))
	if err != nil {
		return nil, common.Address{}, err
	}
	return pk, crypto.PubkeyToAddress(pk.PublicKey), nil
}

func bumpGas(gas uint64) uint64 {
	// 放大系数(基点)
	if GAS_MULTIPLIER_BP <= 10000 {
		return gas
	}
	return uint64((uint64(GAS_MULTIPLIER_BP) * gas) / 10000)
}

func sendTx(ctx context.Context, rpc string, to common.Address, data []byte) (string, error) {
	client, err := geth.DialContext(ctx, rpc)
	if err != nil {
		return "", err
	}
	defer client.Close()

	pk, from, err := loadPK(PRIVATE_KEY_HEX)
	if err != nil {
		return "", err
	}
	chainID, err := client.NetworkID(ctx)
	if err != nil {
		return "", err
	}
	nonce, err := client.PendingNonceAt(ctx, from)
	if err != nil {
		return "", err
	}
	tip, err := client.SuggestGasTipCap(ctx)
	if err != nil || tip == nil {
		tip = big.NewInt(1_000_000_000) // 兜底 1 gwei
	}
	header, _ := client.HeaderByNumber(ctx, nil)
	base := big.NewInt(0)
	if header != nil && header.BaseFee != nil {
		base = header.BaseFee
	}
	feeCap := new(big.Int).Add(new(big.Int).Mul(base, big.NewInt(2)), tip)

	// 估算 gas（用 EOA from 估算）
	gas, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From:  from,
		To:    &to,
		Value: big.NewInt(0),
		Data:  data,
	})
	if err != nil {
		return "", fmt.Errorf("estimate gas: %w", err)
	}
	gas = bumpGas(gas)

	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		To:        &to,
		Value:     big.NewInt(0),
		Gas:       gas,
		GasTipCap: tip,
		GasFeeCap: feeCap,
		Data:      data,
	})
	signer := types.LatestSignerForChainID(chainID)
	stx, err := types.SignTx(tx, signer, pk)
	if err != nil {
		return "", err
	}
	if err := client.SendTransaction(ctx, stx); err != nil {
		return "", err
	}
	return stx.Hash().Hex(), nil
}
