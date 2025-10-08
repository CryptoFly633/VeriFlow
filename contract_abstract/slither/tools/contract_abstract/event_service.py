from argparse import ArgumentParser
import json
import logging
import sys
from slither.tools.contract_abstract.onchain.contract_info import ContractInfo
from slither.tools.contract_abstract.onchain.event_info import EventInfo
from slither.tools.contract_abstract.onchain.transaction_info import TransactionInfo

def parse_args():
    parser = ArgumentParser(description="获取合约event信息")
    parser.add_argument("--meta-path", required=True, action="store", help="元数据文件路径")
    parser.add_argument("--rpc-url", required=True, action="store", help="以太坊RPC URL")
    parser.add_argument("--etherscan-apikey", required=True, action="store", help="Etherscan API密钥")
    
    # 数据库配置参数

    parser.add_argument("--event-db-host", default="localhost", help="event数据库主机地址")
    parser.add_argument("--event-db-port", type=int, default=5432, help="event数据库端口")
    parser.add_argument("--event-db-name", default="ethereum_events", help="event数据库名称")
    parser.add_argument("--event-db-user", default="zhiqiang", help="event数据库用户名")
    parser.add_argument("--event-db-password", default="password", help="event数据库密码")

    parser.add_argument("--tranaction-db-host", default="localhost", help="transaction数据库主机地址")
    parser.add_argument("--tranaction-db-port", type=int, default=5432, help="transaction数据库端口")
    parser.add_argument("--tranaction-db-name", default="ethereum_transactions", help="transaction数据库名称")
    parser.add_argument("--tranaction-db-user", default="zhiqiang", help="transaction数据库用户名")
    parser.add_argument("--tranaction-db-password", default="password", help="transaction数据库密码")
    
    return parser.parse_args()

logger = logging.getLogger(__name__)
logger.setLevel(logging.INFO)

def main() -> None:
    args = parse_args()
    # try:
    # 读取元数据文件
    logger.info("开始读取元数据文件...")
    with open(args.meta_path, "r") as f:
        meta_json = json.load(f)
    logger.info(f"成功读取元数据文件: {args.meta_path}")

    meta_json = meta_json[list(meta_json.keys())[0]]

    # 初始化合约信息
    logger.info("初始化合约信息...")
    contract_info = ContractInfo(args.rpc_url)
    logger.info("合约信息初始化成功")

     # 构建transaction数据库配置
    transaction_db_config = {
        'host': args.tranaction_db_host,
        'port': args.tranaction_db_port,
        'database': args.tranaction_db_name,
        'user': args.tranaction_db_user,
        'password': args.tranaction_db_password
    }
    logger.info(f"transaction数据库配置: {args.tranaction_db_host}:{args.tranaction_db_port}/{args.tranaction_db_name}")

    # 连接交易数据库
    logger.info("连接交易数据库...")
    if meta_json.get("logic_address"):
        transaction_info = TransactionInfo(
            meta_json["address"], 
            args.etherscan_apikey, 
            contract_info, 
            db_config=transaction_db_config,
            logic_address=meta_json["logic_address"]
        )
        logger.info(f"使用代理合约模式，逻辑地址: {meta_json['logic_address']}")
    else:
        transaction_info = TransactionInfo(
            meta_json["address"], 
            args.etherscan_apikey, 
            contract_info,
            db_config=transaction_db_config
        )
        logger.info("使用普通合约模式")
    logger.info("交易数据库连接成功")

    event_db_config = {
        'host': args.event_db_host,
        'port': args.event_db_port,
        'database': args.event_db_name,
        'user': args.event_db_user,
        'password': args.event_db_password
    }

    event_info = EventInfo(meta_json["address"], contract_info, event_db_config)
    latest_block = transaction_info.get_latest_block_number()
    local_latest_block = event_info.get_current_block_number()
    deployment_block = transaction_info.get_contract_creation_block()

    start_block = max(local_latest_block, deployment_block)

    if start_block < latest_block:
        event_info.fetch_and_save_all_events(start_block, latest_block)
    else:
        logger.info("当前事件同步区块号与最新区块号一致，无需同步")

if __name__ == "__main__":
    main()
    

    