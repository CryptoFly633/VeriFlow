package executor

import (
	"context"
	"fmt"
	specyconfig "github.com/cosmos/relayer/v2/specy/config"
	"github.com/cosmos/relayer/v2/specy/types"
	specytypes "github.com/cosmos/relayer/v2/specy/types"
	"google.golang.org/grpc"
	"log"
	"sync"
	"time"
)

var engineStream types.Regulator_GetTaskResultClient

type Heartbeat struct {
	isConnected bool
	mutex       sync.Mutex
}

var instance *Heartbeat
var once sync.Once

func ConnectSpecyEngineWithHeartbeat(ctx context.Context) {
	// 创建并缓存长连接
	createAndCacheEngineStream(ctx, false)

	// 启动心跳检测
	go func() {
		heartbeat := getHeartbeat()
		heartbeat.start(ctx, 5*time.Second)
	}()
}

func getHeartbeat() *Heartbeat {
	once.Do(func() {
		instance = &Heartbeat{isConnected: true}
	})
	return instance
}

func (hb *Heartbeat) start(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval) // 每隔interval时间发送一次心跳请求

	for range ticker.C {
		if hb.isConnected {
			// 发送心跳请求并获取响应
			_, err := engineStream.Recv()
			if err == nil {
				return
			}
			fmt.Println("\nGRPC stream 心跳请求无响应...")
		}

		// 如果连接中断，进行相应处理
		fmt.Println("GRPC stream 开始重连...")

		// 加锁以保证线程安全
		hb.mutex.Lock()
		hb.isConnected = false

		// 重新创建并缓存长连接
		stream := createAndCacheEngineStream(ctx, true)
		if stream != nil {
			hb.isConnected = true
		}
		// 解锁
		hb.mutex.Unlock()
	}
}

func createAndCacheEngineStream(ctx context.Context, isHeartbeat bool) types.Regulator_GetTaskResultClient {
	engineNodeAddress := specyconfig.Config.EngineInfo.EngineNodeAddress
	fmt.Printf("-------------engineNodeAddress: %s \n", engineNodeAddress)

	size := 2100000000
	clientCon, err := grpc.Dial(engineNodeAddress, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(10*time.Second), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(size)))
	if err != nil {
		if !isHeartbeat {
			log.Fatalf("Failed to connnect engine: %v \n", err)
			return nil
		}
		log.Printf("Failed to connnect engine: %v \n", err)
		return nil
	}

	client := types.NewRegulatorClient(clientCon)
	fmt.Printf("-------------client: %+v \n", client)

	stream, err := client.GetTaskResult(ctx)
	fmt.Printf("-------------stream: %+v \n", stream)

	if err != nil {
		log.Fatalf("Failed to create grpc stream with engine: %v", err)
		return nil
	}

	// 保存stream连接到全局变量
	engineStream = stream

	return stream
}

func InvokeEngineWithTx(
	ctx context.Context,
	cts []*types.ContractEvent,
	txHash []byte,
	msgSender []byte,
	chainID string,
	height uint64,
) (types.ProofResponse, error) {

	tmd := &types.TxMetaData{
		FromAddress: msgSender,
		ToAddress:   []byte{},
		Value:       0,
	}
	data := &types.Data{
		Meta:   tmd,
		Events: cts,
	}
	//构建监管请求
	pr := &types.ProofRequest{
		ChainType:    "cosmos",
		ChainID:      chainID,
		Data:         data,
		TxHash:       txHash,
		Height:       height,
		TxIndex:      1,
		OriginalData: nil,
	}

	cproof, err := SendProofRequest(ctx, *pr)

	if err != nil {
		fmt.Println("监管出错！！！！！")
		return types.ProofResponse{}, err
	}
	//对cproof进行检查，后期确定好如何检查在完善
	fmt.Println("监管成功，结束调用")
	fmt.Print(cproof)
	return cproof, err
}

func InvokeEngineWithTask(task *specytypes.Task) (types.TaskResponse, error) {
	// 构建请求
	request := &types.TaskRequest{
		Taskhash:  []byte(task.TaskHash),
		RuleFile:  task.RuleFile,
		InputData: task.CheckData,
	}

	response, err := SendTaskRequest(*request)
	return response, err
}

func SendTaskRequest(request types.TaskRequest) (types.TaskResponse, error) {
	log.Default().Println("开始engine逻辑")
	// 获取缓存的stream
	stream := engineStream
	stream.Send(&request)
	resp, err := stream.Recv()
	fmt.Printf("-------------resp: %+v \n", resp)

	if err != nil {
		fmt.Errorf("-------------err: %+v \n", err)
		return types.TaskResponse{}, err
	}
	return *resp, nil
}

/** ---------------------------------- deprecated functions ---------------------------------- */

func cacheEngineStream(ctx context.Context, stream *types.Regulator_GetTaskResultClient) context.Context {
	specyInfoMap := *ctx.Value(types.SpecyInfoKey).(*map[string]any)
	specyInfoMap[types.EngineStreamKey] = &stream

	return context.WithValue(ctx, types.SpecyInfoKey, &specyInfoMap)
}

func getCachedEngineStream(ctx context.Context) types.Regulator_GetTaskResultClient {
	specyInfoMap, ok := ctx.Value(types.SpecyInfoKey).(map[string]any)
	if !ok {
		return nil
	}
	cs, ok := specyInfoMap[types.EngineStreamKey].(types.Regulator_GetTaskResultClient)
	if !ok {
		return nil
	}
	return cs
}

func SendProofRequest(ctx context.Context, pr types.ProofRequest) (types.ProofResponse, error) {
	//	//如果stream失效则应该进行重新构造尝试
	//	log.Default().Println("开始监管逻辑")
	//	cs := getCachedComplianceStream(ctx)
	//	if cs == nil {
	//		// TODO query registation endpoint info in global context, if does not exist, query chain and store in global context
	//		//registrationList := k.GetAllRegistration(ctx)
	//		//log.Default().Println(registrationList[0])
	//		//if len(registrationList) == 0 {
	//		//	return types.ProofResponse{}, types.ErrEmptyRegistrationList
	//		//}
	//
	//		//complianceAddress := registrationList[0].Endpoint.IpAddress + ":" + fmt.Sprint(registrationList[0].Endpoint.Port)
	//		complianceAddress := getRegulatoryEndpoint(ctx)
	//		fmt.Printf("-------------complianceAddress: %s \n", complianceAddress)
	//
	//		clientCon, err := grpc.Dial(complianceAddress, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(10*time.Second))
	//		if err != nil {
	//			log.Fatal(err)
	//		}
	//
	//		client := types.NewRegulatorClient(clientCon)
	//		fmt.Printf("-------------client: %+v \n", client)
	//
	//		stream, err := client.GetComplianceProof(context.Background())
	//		fmt.Printf("-------------stream: %+v \n", stream)
	//
	//		if err != nil {
	//			return types.ProofResponse{}, err
	//		}
	//		cacheComplianceStream(ctx, &stream)
	//		cs = stream
	//	}
	//	cs.Send(&pr)
	//	resp, err := cs.Recv()
	//	if err != nil {
	//		return types.ProofResponse{}, err
	//	}
	//	return *resp, nil
	return types.ProofResponse{}, nil
}

//
//func cacheComplianceStream(ctx context.Context, stream *types.Regulator_GetComplianceProofClient) {
//	specyInfoMap := ctx.Value(types.SpecyInfoKey).(map[string]any)
//	specyInfoMap[types.ComplianceStreamKey] = &stream
//
//	ctx = context.WithValue(ctx, types.SpecyInfoKey, &specyInfoMap)
//}
//
//func getCachedComplianceStream(ctx context.Context) types.Regulator_GetComplianceProofClient {
//	specyInfoMap, ok := ctx.Value(types.SpecyInfoKey).(map[string]any)
//	if !ok {
//		return nil
//	}
//	cs, ok := specyInfoMap[types.ComplianceStreamKey].(types.Regulator_GetComplianceProofClient)
//	if !ok {
//		return nil
//	}
//	return cs
//}

func getRegulatoryEndpoint(ctx context.Context) string {
	specyInfoMap := ctx.Value(types.SpecyInfoKey).(map[string]any)
	return specyInfoMap[types.RegistrationEndpointKey].(string)
}
