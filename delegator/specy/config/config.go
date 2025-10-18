package config

import (
	"encoding/json"
	"fmt"
	specytypes "github.com/cosmos/relayer/v2/specy/types"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"path"
	"runtime"
)

var (
	Config = &SpecyConfig{}
)

// SpecyConfig specy network config infos
type SpecyConfig struct {
	ChainInfo  SpecyChainInfo  `yaml:"chain_info"`
	EngineInfo SpecyEngineInfo `yaml:"engine_info"`
}

type SpecyChainInfo struct {
	ChainId                string `yaml:"chain_id"`
	BinaryLocation         string `yaml:"chain_binary_location"`
	HomeDir                string `yaml:"home_dir"`
	NodeAddress            string `yaml:"node_address"`
	ValidatorWalletAddress string `yaml:"validator_wallet_address"`
}

type SpecyEngineInfo struct {
	EngineNodeAddress string `yaml:"engine_node_address"`
	IASReport         string `yaml:"ias_report"`
	EnclavePK         string `yaml:"enclave_pk"`
}

// ChainBinaryInfo target chain binary info
type ChainBinaryInfo struct {
	ChainId        string
	BinaryLocation string
}

func ReadSpecyConfigInfo() {
	// 获取当前文件的路径
	_, filename, _, _ := runtime.Caller(0)
	root := path.Dir(filename)

	// 读取配置文件
	configPath := path.Join(root, "..", "..", "specy-config.yaml")
	cfg, err := readConfigFile(configPath)
	if err != nil {
		log.Fatalf("Failed to read config file: %v", err)
		return
	}

	// 保存配置到全局变量
	Config = cfg
}

func readConfigFile(filePath string) (*SpecyConfig, error) {
	// 读取配置文件内容
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	// 解析配置文件为结构体
	var config SpecyConfig
	if err := yaml.Unmarshal(content, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func AddTargetChainConfig(chainName string, binaryLocation string) {
	chainConfig := &ChainBinaryInfo{chainName, binaryLocation}
	targetChainBinaryInfoMap := make(map[string]*ChainBinaryInfo)
	targetChainBinaryInfoMap[chainName] = chainConfig

	redis := GetRedisClient()
	mapJSONBytes, err := json.Marshal(targetChainBinaryInfoMap)
	if err != nil {
		fmt.Errorf("json marshal error: %+v \n", err)
	}

	redis.Set(specytypes.TargetChainBinaryInfo, string(mapJSONBytes), 0)
}

func GetTargetChainBinaryInfoMap() (map[string]*ChainBinaryInfo, error) {
	redis := GetRedisClient()
	result, err := redis.Get(specytypes.TargetChainBinaryInfo).Result()
	if err != nil {
		return nil, err
	}
	targetChainBinaryInfoMap := make(map[string]*ChainBinaryInfo)
	err = json.Unmarshal([]byte(result), &targetChainBinaryInfoMap)
	if err != nil {
		fmt.Errorf("json unmarshal error: %+v \n", err)
	}

	return targetChainBinaryInfoMap, nil
}

func DeleteTargetChainConfig(chainName string) (err error) {
	targetChainBinaryInfoMap, err := GetTargetChainBinaryInfoMap()
	if err != nil {
		return err
	}
	delete(targetChainBinaryInfoMap, chainName)
	// update (can be omitted?)
	//Cache.Set(specytypes.TargetChainBinaryInfo, targetChainBinaryInfoMap, cache.NoExpiration)
	return nil
}

func GetTargetChainConfig(chainName string) (*ChainBinaryInfo, error) {
	targetChainBinaryInfoMap, err := GetTargetChainBinaryInfoMap()
	if err != nil {
		return nil, err
	}
	chainConfig := targetChainBinaryInfoMap[chainName]
	if chainConfig == nil {
		fmt.Printf("chain with name %s is not configured \n", chainName)
		return nil, nil
	}
	return chainConfig, nil
}
