import requests
import psycopg2
import time
import json
from psycopg2.extras import RealDictCursor
from typing import List, Dict, Any, Optional, Generator
import logging
import sys
import os
from slither.tools.contract_abstract.database_manager import DatabaseManager

logger = logging.getLogger(__name__)
logger.setLevel(logging.INFO)

BASE_URL = "https://api.etherscan.io/api"

class TransactionInfo:
    def __init__(self, target_address, etherscan_api_key, contract_info, db_config=None, logic_address=None):
        self.contract_info = contract_info
        self.etherscan_api_key = etherscan_api_key
        self.address = target_address
        self.logic_address = logic_address
        self.table_name = f"ethereum_transactions_{self.address}"
        self.db_config = db_config or {
            'host': 'localhost',
            'database': 'ethereum_transactions',
            'user': 'postgres',
            'password': 'password',
            'port': 5432
        }
        self.db_connection = None
        self.contract_creation_block = None
        
        # 初始化数据库管理器并设置数据库环境
        self.db_manager = DatabaseManager(self.db_config)
        if not self.db_manager.setup_database():
            raise Exception("数据库环境设置失败")
        
        self.connect_db()
        self.create_tables()
        self.revert_threshold = 12
        if self.logic_address:
            self.abi = self.get_contract_abi(self.logic_address)
        else:
            self.abi = self.get_contract_abi(self.address)

    def connect_db(self):
        """连接到PostgreSQL数据库"""
        max_retries = 3
        retry_delay = 2
        
        for attempt in range(max_retries):
            try:
                logger.info(f"尝试连接数据库: {self.db_config['host']}:{self.db_config['port']}/{self.db_config['database']}")
                self.db_connection = psycopg2.connect(**self.db_config)
                self.db_connection.autocommit = False
                logger.info("成功连接到PostgreSQL数据库")
                return
            except psycopg2.OperationalError as e:
                error_msg = str(e).lower()
                logger.error(f"数据库连接失败 (尝试 {attempt + 1}/{max_retries}): {e}")
                
                if "database" in error_msg and "does not exist" in error_msg:
                    # 尝试使用小写数据库名称连接
                    original_db_name = self.db_config['database']
                    lowercase_db_name = original_db_name.lower()
                    
                    if original_db_name != lowercase_db_name:
                        logger.info(f"尝试使用小写数据库名称连接: {lowercase_db_name}")
                        try:
                            temp_config = self.db_config.copy()
                            temp_config['database'] = lowercase_db_name
                            self.db_connection = psycopg2.connect(**temp_config)
                            self.db_connection.autocommit = False
                            logger.info(f"成功连接到数据库 (使用小写名称: {lowercase_db_name})")
                            # 更新配置中的数据库名称为小写
                            self.db_config['database'] = lowercase_db_name
                            return
                        except psycopg2.OperationalError as e2:
                            logger.error(f"使用小写数据库名称连接也失败: {e2}")
                    
                    logger.error(f"数据库 {self.db_config['database']} 不存在")
                    logger.info("请确保数据库已创建，或者检查数据库管理器是否正确运行")
                    raise
                elif "authentication failed" in error_msg:
                    logger.error("数据库认证失败，请检查用户名和密码")
                    raise
                elif "connection refused" in error_msg:
                    logger.error("无法连接到PostgreSQL服务，请确保服务正在运行")
                    raise
                elif attempt < max_retries - 1:
                    logger.info(f"等待 {retry_delay} 秒后重试...")
                    time.sleep(retry_delay)
                    retry_delay *= 2  # 指数退避
                else:
                    logger.error("数据库连接失败，已达到最大重试次数")
                    raise
            except Exception as e:
                logger.error(f"数据库连接时发生未知错误: {e}")
                raise

    def create_tables(self):
        """创建交易数据表"""
        if not self.db_connection:
            self.connect_db()
        
        cursor = self.db_connection.cursor()
        
        # 创建交易表
        create_table_sql = f"""
        CREATE TABLE IF NOT EXISTS {self.table_name} (
            block_hash VARCHAR(66),
            block_number BIGINT,
            time_stamp BIGINT,
            hash VARCHAR(66) PRIMARY KEY,
            nonce BIGINT,
            transaction_index INTEGER,
            from_address VARCHAR(42),
            to_address VARCHAR(42),
            value NUMERIC(65,0),
            gas BIGINT,
            gas_price BIGINT,
            input_data TEXT,
            method_id VARCHAR(10),
            function_name TEXT,
            contract_address VARCHAR(42),
            cumulative_gas_used BIGINT,
            tx_receipt_status INTEGER,
            gas_used BIGINT,
            confirmations BIGINT,
            is_error INTEGER,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        );
        """
        
        # 创建索引以提高查询性能
        create_indexes_sql = f"""
        CREATE INDEX IF NOT EXISTS idx_{self.table_name}_block_hash ON {self.table_name}(block_hash);
        CREATE INDEX IF NOT EXISTS idx_{self.table_name}_block_number ON {self.table_name}(block_number);
        CREATE INDEX IF NOT EXISTS idx_{self.table_name}_from_address ON {self.table_name}(from_address);
        CREATE INDEX IF NOT EXISTS idx_{self.table_name}_to_address ON {self.table_name}(to_address);
        CREATE INDEX IF NOT EXISTS idx_{self.table_name}_time_stamp ON {self.table_name}(time_stamp);
        """
        
        try:
            cursor.execute(create_table_sql)
            cursor.execute(create_indexes_sql)
            # 提交事务
            self.db_connection.commit()
            logger.info("数据库表已经创建或者创建成功")
        except Exception as e:
            logger.error(f"创建表失败: {e}")
            self.db_connection.rollback()
            raise
        finally:
            cursor.close()

    def get_contract_abi(self, address):
        params = {
            "module": "contract",
            "action": "getabi",
            "address": address,
            "apikey": self.etherscan_api_key
        }
        resp = requests.get(BASE_URL, params=params).json()
        abi_json = resp.get("result")
        if abi_json and abi_json != 'Contract source code not verified':
            return self.contract_info.w3.codec.decode_abi if abi_json is None else abi_json
        else:
            raise Exception("ABI not available or contract not verified on Etherscan")

    def _get_transactions(self, address, start_block, end_block, action):
        """获取指定地址的交易记录（支持分页）
        
        Args:
            address: 合约地址
            start_block: 起始区块号
            end_block: 结束区块号
            action: API动作类型 ("txlist" 或 "txlistinternal")
            
        Returns:
            List[Dict]: 交易记录列表，失败时返回空列表
        """
        all_transactions = []
        page = 1
        offset = 10000  # Etherscan API每页最大记录数
        max_pages = 100  # 最大页数限制，防止无限循环
        
        logger.info(f"开始获取 {action} 交易记录，区块范围: {start_block}-{end_block}")
        
        while page <= max_pages:
            params = {
                "module": "account",
                "action": action,
                "address": address,
                "startblock": start_block,
                "endblock": end_block,
                "sort": "asc",
                "page": page,
                "offset": offset,
                "apikey": self.etherscan_api_key
            }
            
            try:
                # 发送HTTP请求
                logger.info(f"请求Etherscan API: {action}, 地址: {address}, 区块范围: {start_block}-{end_block}, 页码: {page}")
                resp = requests.get(BASE_URL, params=params, timeout=30)
                resp.raise_for_status()  # 检查HTTP状态码
                
                # 解析JSON响应
                data = resp.json()
                
                # 检查API响应状态
                if data.get("status") == "1":
                    result = data.get("result", [])
                    if isinstance(result, list):
                        if len(result) == 0:
                            logger.info(f"第 {page} 页没有更多数据，停止分页")
                            break
                        
                        all_transactions.extend(result)
                        logger.info(f"第 {page} 页获取到 {len(result)} 条 {action} 交易记录，累计: {len(all_transactions)} 条")
                        
                        # 如果返回的记录数少于offset，说明已经是最后一页
                        if len(result) < offset:
                            logger.info(f"第 {page} 页记录数 ({len(result)}) 少于每页限制 ({offset})，已获取所有数据")
                            break
                        # 如果返回的记录数大于offset, 也无法继续下一页，因为当前账号限制因此需要raise Exception
                        if len(result) >= offset:
                            logger.error(f"第 {page} 页记录数 ({len(result)}) 大于每页限制 ({offset})，无法继续下一页")
                            raise Exception(f"第 {page} 页记录数 ({len(result)}) 大于每页限制 ({offset})，无法继续下一页")
                        
                        page += 1
                        
                        # 添加延迟避免API限流
                        time.sleep(0.2)
                        
                    else:
                        logger.error(f"API返回的result不是列表格式: {type(result)}")
                        raise Exception(f"API返回的result不是列表格式: {type(result)}")
                else:
                    # API返回错误状态
                    error_message = data.get("message", "未知错误")
                    logger.error(f"Etherscan API返回错误: {error_message}")
                    
                    # 处理特定的API错误
                    if "rate limit" in error_message.lower():
                        logger.warning("达到API速率限制，等待后重试...")
                        time.sleep(1)  # 等待1秒后重试
                        continue
                    elif "invalid api key" in error_message.lower():
                        logger.error("API密钥无效")
                        raise Exception("API密钥无效")
                    elif "no transactions found" in error_message.lower():
                        logger.info(f"区块范围 {start_block}-{end_block} 中没有找到 {action} 交易")
                        break
                    else:
                        raise Exception(f"Etherscan API返回错误: {error_message}")
                        
            except requests.exceptions.Timeout:
                logger.error(f"请求Etherscan API超时: {action}, 页码: {page}")
                raise Exception(f"请求Etherscan API超时: {action}")
            except requests.exceptions.ConnectionError as e:
                logger.error(f"网络连接错误: {e}")
                raise Exception(f"网络连接错误: {e}")
            except requests.exceptions.HTTPError as e:
                logger.error(f"HTTP请求错误: {e}")
                raise Exception(f"HTTP请求错误: {e}")
            except requests.exceptions.RequestException as e:
                logger.error(f"请求异常: {e}")
                raise Exception(f"请求异常: {e}")
            except json.JSONDecodeError as e:
                logger.error(f"JSON解析错误: {e}")
                raise Exception(f"JSON解析错误: {e}")
            except Exception as e:
                logger.error(f"获取 {action} 交易时发生未知错误: {e}")
                raise Exception(f"获取 {action} 交易时发生未知错误: {e}")
        
        if page > max_pages:
            logger.warning(f"达到最大页数限制 ({max_pages})，可能还有更多数据未获取")
        
        logger.info(f"总共获取到 {len(all_transactions)} 条 {action} 交易记录")
        return all_transactions

    def decode_input(self, abi, tx_input):
        contract = self.contract_info.w3.eth.contract(abi=abi)
        try:
            func_obj, params = contract.decode_function_input(tx_input)
            return func_obj.fn_name, params
        except Exception:
            raise Exception(f"解码交易输入失败: {tx_input}")

    def get_transactions_from_etherscan(self, start_block, end_block):
        """获取指定区块范围内的所有交易记录（包括普通交易和内部交易）
        
        Args:
            start_block: 起始区块号
            end_block: 结束区块号
            
        Returns:
            List[Dict]: 合并后的交易记录列表
        """
        try:
            logger.info(f"开始获取区块范围 {start_block}-{end_block} 的所有交易记录")
            
            # 获取普通交易
            logger.info("=== 获取普通交易 ===")
            normal_txs = self._get_transactions(self.address, start_block, end_block, "txlist")
            logger.info(f"✅ 普通交易获取完成，共 {len(normal_txs)} 条")
            
            # 获取内部交易
            logger.info("=== 获取内部交易 ===")
            internal_txs = self._get_transactions(self.address, start_block, end_block, "txlistinternal")
            logger.info(f"✅ 内部交易获取完成，共 {len(internal_txs)} 条")
            
            # 合并交易记录
            all_txs = normal_txs + internal_txs
            logger.info(f"🎉 区块范围 {start_block}-{end_block} 总共获取到 {len(all_txs)} 条交易记录")
            logger.info(f"   - 普通交易: {len(normal_txs)} 条")
            logger.info(f"   - 内部交易: {len(internal_txs)} 条")
            
            return all_txs
            
        except Exception as e:
            raise Exception(f"获取交易记录时发生错误: {e}")

    def save_transactions_to_db(self, transactions: List[Dict[str, Any]]) -> int:
        """将交易数据保存到PostgreSQL数据库"""
        if not self.db_connection:
            self.connect_db()
        
        cursor = self.db_connection.cursor()
        saved_count = 0
        
        insert_sql = f"""
        INSERT INTO {self.table_name} (
            block_hash, block_number, time_stamp, hash, nonce, transaction_index,
            from_address, to_address, value, gas, gas_price, input_data,
            method_id, function_name, contract_address, cumulative_gas_used,
            tx_receipt_status, gas_used, confirmations, is_error
        ) VALUES (
            %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s
        ) ON CONFLICT (hash) DO NOTHING
        """
        
        try:
            for tx in transactions:
                # 处理数据转换
                values = (
                    tx.get('blockHash'),
                    int(tx.get('blockNumber', 0)) if tx.get('blockNumber') else None,
                    int(tx.get('timeStamp', 0)) if tx.get('timeStamp') else None,
                    tx.get('hash'),
                    int(tx.get('nonce', 0)) if tx.get('nonce') else None,
                    int(tx.get('transactionIndex', 0)) if tx.get('transactionIndex') else None,
                    tx.get('from'),
                    tx.get('to'),
                    tx.get('value'),
                    int(tx.get('gas', 0)) if tx.get('gas') else None,
                    int(tx.get('gasPrice', 0)) if tx.get('gasPrice') else None,
                    tx.get('input'),
                    tx.get('methodId'),
                    tx.get('functionName'),
                    tx.get('contractAddress'),
                    int(tx.get('cumulativeGasUsed', 0)) if tx.get('cumulativeGasUsed') else None,
                    int(tx.get('txreceipt_status', 0)) if tx.get('txreceipt_status') else None,
                    int(tx.get('gasUsed', 0)) if tx.get('gasUsed') else None,
                    int(tx.get('confirmations', 0)) if tx.get('confirmations') else None,
                    int(tx.get('isError', 0)) if tx.get('isError') else None
                )
                
                cursor.execute(insert_sql, values)
                saved_count += cursor.rowcount
            
            # 提交事务
            self.db_connection.commit()
            logger.info(f"成功保存 {saved_count} 条交易记录到数据库")
            return saved_count
            
        except Exception as e:
            logger.error(f"保存第{saved_count}个交易数据失败: {e}")
            self.db_connection.rollback()
            raise
        finally:
            cursor.close()

    def get_transactions_from_db(self, start_block, end_block, limit = 10000, page = 1) -> List[Dict[str, Any]]:
        """从数据库查询交易数据（支持分页）"""
        if not self.db_connection:
            self.connect_db()
        
        # 设置合理的分页参数
        page_size = min(limit, 10000) if limit else 10000  # 每页最大10000条
        offset = (page - 1) * page_size
        
        cursor = self.db_connection.cursor(cursor_factory=RealDictCursor)
        
        query = f"SELECT * FROM {self.table_name} WHERE 1=1"
        params = []
        
        if start_block:
            query += " AND block_number >= %s"
            params.append(start_block)
        
        if end_block:
            query += " AND block_number <= %s"
            params.append(end_block)
        
        query += " ORDER BY block_number ASC, transaction_index ASC LIMIT %s OFFSET %s"
        params.extend([page_size, offset])
        
        try:
            cursor.execute(query, params)
            results = cursor.fetchall()
            logger.info(f"查询第{page}页，每页{page_size}条，实际返回{len(results)}条记录")
            return [dict(row) for row in results]
        except Exception as e:
            logger.error(f"查询交易数据失败: {e}")
            raise
        finally:
            cursor.close()

    def get_transactions_paginated(self, start_block, end_block, page_size=10000, 
                                 max_pages=None, callback=None) -> Generator[List[Dict[str, Any]], None, None]:
        """
        分页查询交易数据，支持回调函数处理每页数据
        
        Args:
            start_block: 起始区块号
            end_block: 结束区块号
            page_size: 每页大小，默认10000
            max_pages: 最大页数限制，None表示无限制
            callback: 回调函数，用于处理每页数据，参数为(page_num, transactions)
        
        Yields:
            List[Dict[str, Any]]: 每页的交易数据（按区块号升序）
        """
        if not self.db_connection:
            self.connect_db()
        
        page = 1
        total_processed = 0
        
        while True:
            if max_pages and page > max_pages:
                logger.info(f"已达到最大页数限制: {max_pages}")
                break
                
            transactions = self.get_transactions_from_db(
                start_block=start_block, 
                end_block=end_block, 
                limit=page_size, 
                page=page
            )
            
            if not transactions:
                logger.info(f"第{page}页无数据，查询结束")
                break
                
            total_processed += len(transactions)
            logger.info(f"处理第{page}页，本页{len(transactions)}条，累计{total_processed}条")
            
            # 如果提供了回调函数，先执行回调
            if callback:
                try:
                    callback(page, transactions)
                except Exception as e:
                    logger.error(f"回调函数执行失败 (第{page}页): {e}")
            
            # 返回当前页数据
            yield transactions
            
            page += 1

    def get_total_transaction_count(self, start_block=None, end_block=None) -> int:
        """
        获取指定范围内的交易总数
        
        Args:
            start_block: 起始区块号
            end_block: 结束区块号
            
        Returns:
            int: 交易总数
        """
        if not self.db_connection:
            self.connect_db()
        
        cursor = self.db_connection.cursor()
        
        query = f"SELECT COUNT(*) as total FROM {self.table_name} WHERE 1=1"
        params = []
        
        if start_block:
            query += " AND block_number >= %s"
            params.append(start_block)
        
        if end_block:
            query += " AND block_number <= %s"
            params.append(end_block)
        
        try:
            cursor.execute(query, params)
            result = cursor.fetchone()
            total_count = result[0] if result else 0
            logger.info(f"查询范围内交易总数: {total_count}")
            return total_count
        except Exception as e:
            logger.error(f"获取交易总数失败: {e}")
            raise
        finally:
            cursor.close()

    def get_transactions_by_batch(self, start_block=None, end_block=None, batch_size=10000, 
                                callback=None) -> Generator[List[Dict[str, Any]], None, None]:
        """
        批量查询交易数据，适合大数据量处理
        
        Args:
            start_block: 起始区块号
            end_block: 结束区块号
            batch_size: 批次大小，默认10000
            callback: 回调函数，用于处理每批数据，参数为(batch_num, transactions)
        
        Yields:
            List[Dict[str, Any]]: 每批的交易数据（按区块号升序）
        """
        if not self.db_connection:
            self.connect_db()
        
        batch_num = 1
        total_processed = 0
        
        while True:
            transactions = self.get_transactions_from_db(
                start_block=start_block, 
                end_block=end_block, 
                limit=batch_size, 
                page=batch_num
            )
            
            if not transactions:
                logger.info(f"第{batch_num}批无数据，查询结束")
                break
                
            total_processed += len(transactions)
            logger.info(f"处理第{batch_num}批，本批{len(transactions)}条，累计{total_processed}条")
            
            # 如果提供了回调函数，先执行回调
            if callback:
                try:
                    callback(batch_num, transactions)
                except Exception as e:
                    logger.error(f"回调函数执行失败 (第{batch_num}批): {e}")
            
            # 返回当前批数据
            yield transactions
            
            batch_num += 1

    def save_single_transaction(self, transaction: Dict[str, Any]) -> bool:
        """保存单条交易数据"""
        return self.save_transactions_to_db([transaction]) > 0

    def get_latest_block_number(self) -> Optional[int]:
        """获取当前存储的交易中最新的区块号"""
        if not self.db_connection:
            self.connect_db()
        
        cursor = self.db_connection.cursor()
        
        query = f"""
        SELECT MAX(block_number) as latest_block 
        FROM {self.table_name} 
        WHERE block_number IS NOT NULL
        """
        
        try:
            cursor.execute(query)
            result = cursor.fetchone()
            
            if result and result[0] is not None:
                latest_block = int(result[0])
                logger.info(f"当前存储的最新区块号: {latest_block}")
                return latest_block
            else:
                logger.info("数据库中暂无交易数据")
                return 0
                
        except Exception as e:
            logger.error(f"获取最新区块号失败: {e}")
            return None
        finally:
            cursor.close()

    def get_latest_block_info(self) -> Optional[Dict[str, Any]]:
        """获取最新区块的详细信息"""
        if not self.db_connection:
            self.connect_db()
        
        cursor = self.db_connection.cursor(cursor_factory=RealDictCursor)
        
        query = f"""
        SELECT 
            block_number,
            block_hash,
            time_stamp,
            COUNT(*) as transaction_count,
            MIN(created_at) as first_transaction_time,
            MAX(created_at) as last_transaction_time
        FROM {self.table_name} 
        WHERE block_number = (
            SELECT MAX(block_number) 
            FROM {self.table_name} 
            WHERE block_number IS NOT NULL
        )
        GROUP BY block_number, block_hash, time_stamp
        """
        
        try:
            cursor.execute(query)
            result = cursor.fetchone()
            
            if result:
                block_info = dict(result)
                logger.info(f"最新区块信息: 区块号 {block_info['block_number']}, "
                      f"包含 {block_info['transaction_count']} 笔交易")
                return block_info
            else:
                logger.info("数据库中暂无交易数据")
                return None
                
        except Exception as e:
            logger.error(f"获取最新区块信息失败: {e}")
            return None
        finally:
            cursor.close()

    def get_block_range(self) -> Optional[Dict[str, int]]:
        """获取数据库中区块的范围（最小和最大区块号）"""
        if not self.db_connection:
            self.connect_db()
        
        cursor = self.db_connection.cursor()
        
        query = f"""
        SELECT 
            MIN(block_number) as min_block,
            MAX(block_number) as max_block,
            COUNT(DISTINCT block_number) as total_blocks
        FROM {self.table_name} 
        WHERE block_number IS NOT NULL
        """
        
        try:
            cursor.execute(query)
            result = cursor.fetchone()
            
            if result and result[0] is not None:
                block_range = {
                    'min_block': int(result[0]),
                    'max_block': int(result[1]),
                    'total_blocks': int(result[2])
                }
                logger.info(f"区块范围: {block_range['min_block']} - {block_range['max_block']} "
                      f"(共 {block_range['total_blocks']} 个区块)")
                return block_range
            else:
                logger.info("数据库中暂无交易数据")
                return None
                
        except Exception as e:
            logger.error(f"获取区块范围失败: {e}")
            return None
        finally:
            cursor.close()

    def get_etherscan_latest_block(self) -> Optional[int]:
        """从Etherscan获取最新区块号"""
        params = {
            "module": "proxy",
            "action": "eth_blockNumber",
            "apikey": self.etherscan_api_key
        }
        
        try:
            resp = requests.get(BASE_URL, params=params)
            resp.raise_for_status()  # 检查HTTP错误
            data = resp.json()
            
            if data.get("result"):
                # Etherscan返回的是十六进制格式的区块号
                latest_block = int(data["result"], 16)
                logger.info(f"Etherscan最新区块号: {latest_block}")
                return latest_block
            else:
                logger.error(f"Etherscan API返回错误: {data}")
                return None
                
        except requests.exceptions.RequestException as e:
            logger.error(f"请求Etherscan API失败: {e}")
            return None
        except ValueError as e:
            logger.error(f"解析区块号失败: {e}")
            return None
        except Exception as e:
            logger.error(f"获取Etherscan最新区块号时发生未知错误: {e}")
            return None

    def get_etherscan_block_info(self, block_number: int) -> Optional[Dict[str, Any]]:
        """从Etherscan获取指定区块的详细信息"""
        params = {
            "module": "proxy",
            "action": "eth_getBlockByNumber",
            "tag": hex(block_number),
            "boolean": "true",
            "apikey": self.etherscan_api_key
        }
        
        try:
            resp = requests.get(BASE_URL, params=params)
            resp.raise_for_status()
            data = resp.json()
            
            if data.get("status") == "1" and data.get("result"):
                block_info = data["result"]
                # 转换十六进制数据
                processed_info = {
                    "number": int(block_info["number"], 16) if block_info["number"] != "0x" else 0,
                    "hash": block_info["hash"],
                    "parentHash": block_info["parentHash"],
                    "timestamp": int(block_info["timestamp"], 16) if block_info["timestamp"] != "0x" else 0,
                    "transactions": len(block_info["transactions"]) if block_info["transactions"] else 0,
                    "gasLimit": int(block_info["gasLimit"], 16) if block_info["gasLimit"] != "0x" else 0,
                    "gasUsed": int(block_info["gasUsed"], 16) if block_info["gasUsed"] != "0x" else 0,
                    "miner": block_info["miner"],
                    "difficulty": int(block_info["difficulty"], 16) if block_info["difficulty"] != "0x" else 0,
                    "totalDifficulty": int(block_info["totalDifficulty"], 16) if block_info["totalDifficulty"] != "0x" else 0,
                    "size": int(block_info["size"], 16) if block_info["size"] != "0x" else 0
                }
                logger.info(f"区块 {block_number} 信息: {processed_info['transactions']} 笔交易, "
                          f"Gas使用: {processed_info['gasUsed']}/{processed_info['gasLimit']}")
                return processed_info
            else:
                logger.error(f"Etherscan API返回错误: {data}")
                return None
                
        except requests.exceptions.RequestException as e:
            logger.error(f"请求Etherscan API失败: {e}")
            return None
        except ValueError as e:
            logger.error(f"解析区块信息失败: {e}")
            return None
        except Exception as e:
            logger.error(f"获取区块信息时发生未知错误: {e}")
            return None

    def get_contract_creation_tx(self) -> Optional[Dict[str, Any]]:
        """获取合约的部署交易信息"""
        params = {
            "module": "contract",
            "action": "getcontractcreation",
            "contractaddresses": self.address,
            "apikey": self.etherscan_api_key
        }
        
        try:
            resp = requests.get(BASE_URL, params=params)
            resp.raise_for_status()
            data = resp.json()
            
            if data.get("status") == "1" and data.get("result"):
                creation_info = data["result"][0]  # 通常只有一个结果
                logger.info(f"合约 {self.address} 部署信息: 区块 {creation_info.get('blockNumber')}, "
                          f"交易哈希 {creation_info.get('txHash')}")
                return creation_info
            else:
                logger.error(f"获取合约部署信息失败: {data}")
                return None
                
        except requests.exceptions.RequestException as e:
            logger.error(f"请求Etherscan API失败: {e}")
            return None
        except Exception as e:
            logger.error(f"获取合约部署交易时发生未知错误: {e}")
            return None

    def get_contract_creation_block(self) -> Optional[int]:
        """获取合约的部署区块号"""
        if self.contract_creation_block is not None:
            return self.contract_creation_block
        creation_info = self.get_contract_creation_tx()
        if creation_info and creation_info.get("blockNumber"):
            try:
                block_number = int(creation_info["blockNumber"])
                logger.info(f"合约 {self.address} 部署区块号: {block_number}")
                return block_number
            except ValueError:
                logger.error(f"解析部署区块号失败: {creation_info['blockNumber']}")
                return None
        return None

    def close_db_connection(self):
        """关闭数据库连接"""
        if self.db_connection:
            self.db_connection.close()
            logger.info("数据库连接已关闭")

    def __del__(self):
        """析构函数，确保关闭数据库连接"""
        self.close_db_connection()
    
    



    

        