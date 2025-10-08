#使用infra获得某个给定地址合约的所有emit的event，并保存到数据库中

import psycopg2
import time
import json
import logging
from typing import List, Dict, Any, Optional, Generator
from psycopg2.extras import RealDictCursor
from slither.tools.contract_abstract.database_manager import DatabaseManager

logger = logging.getLogger(__name__)
logger.setLevel(logging.INFO)

class EventInfo:
    def __init__(self, contract_address, contract_info, db_config):
        self.contract_address = contract_address
        self.db_config = db_config
        self.db_connection = None
        self.w3 = contract_info.w3
        self.table_name = f"ethereum_events_{self.contract_address}"
        
        # 初始化数据库管理器并设置数据库环境
        self.db_manager = DatabaseManager(self.db_config)
        if not self.db_manager.setup_database():
            raise Exception("数据库环境设置失败")
        
        self.connect_db()
        self.create_init_tables()
        
    def connect_db(self):
        """连接到PostgreSQL数据库"""
        max_retries = 3
        retry_delay = 2
        
        for attempt in range(max_retries):
            try:
                logger.info(f"尝试连接Event数据库: {self.db_config['host']}:{self.db_config['port']}/{self.db_config['database']}")
                self.db_connection = psycopg2.connect(**self.db_config)
                self.db_connection.autocommit = False
                logger.info("成功连接到Event数据库")
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
                            logger.info(f"成功连接到Event数据库 (使用小写名称: {lowercase_db_name})")
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
    
    def create_init_tables(self):
        """创建事件数据表"""
        if not self.db_connection:
            self.connect_db()
        
        cursor = self.db_connection.cursor()
        
        try:
            # 创建事件数据表
            create_table_sql = f"""
            CREATE TABLE IF NOT EXISTS {self.table_name} (
                __id BIGSERIAL PRIMARY KEY,
                block_number BIGINT NOT NULL,
                block_hash VARCHAR(66) NOT NULL,
                transaction_hash VARCHAR(66) NOT NULL,
                transaction_index INTEGER NOT NULL,
                log_index INTEGER NOT NULL,
                address VARCHAR(42) NOT NULL,
                event_name VARCHAR(255),
                event_signature VARCHAR(66),
                topics JSONB,
                data TEXT,
                removed BOOLEAN DEFAULT FALSE,
                created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                UNIQUE(block_number, transaction_hash, log_index)
            );
            """
            
            cursor.execute(create_table_sql)
            
            # 创建索引以提高查询性能
            cursor.execute(f"""
                CREATE INDEX IF NOT EXISTS idx_{self.table_name}_block_number 
                ON {self.table_name}(block_number)
            """)
            
            cursor.execute(f"""
                CREATE INDEX IF NOT EXISTS idx_{self.table_name}_transaction_hash 
                ON {self.table_name}(transaction_hash)
            """)
            
            cursor.execute(f"""
                CREATE INDEX IF NOT EXISTS idx_{self.table_name}_event_signature 
                ON {self.table_name}(event_signature)
            """)
            
            cursor.execute(f"""
                CREATE INDEX IF NOT EXISTS idx_{self.table_name}_address 
                ON {self.table_name}(address)
            """)
            
            self.db_connection.commit()
            logger.info(f"成功创建事件表: {self.table_name}")
            
        except Exception as e:
            self.db_connection.rollback()
            logger.error(f"创建事件表失败: {e}")
            raise
        finally:
            cursor.close()

    def get_current_block_number(self):
        """
        从数据库中获取当前事件同步的最大区块号
        
        Returns:
            int: 当前最大区块号，如果没有事件记录则返回0
        """
        if not self.db_connection:
            self.connect_db()
        
        cursor = self.db_connection.cursor()
        
        try:
            # 查询事件表中最大的block_number
            query = f"""
            SELECT MAX(block_number) as max_block 
            FROM {self.table_name} 
            WHERE block_number IS NOT NULL
            """
            
            cursor.execute(query)
            result = cursor.fetchone()
            
            if result and result[0] is not None:
                current_block = int(result[0])
                logger.info(f"从数据库获取当前事件同步区块号: {current_block}")
                return current_block
            else:
                logger.info("数据库中未找到事件记录，返回0")
                return 0
                
        except Exception as e:
            logger.error(f"获取当前事件同步区块号失败: {e}")
            return 0
        finally:
            cursor.close()
            
    
    def get_all_events_from_infra(self, start_block=None, end_block=None, 
                                event_signatures=None, batch_size=100000) -> Generator[List[Dict[str, Any]], None, None]:
        """
        从infra获取指定合约的所有事件
        
        Args:
            start_block: 起始区块号
            end_block: 结束区块号
            event_signatures: 事件签名列表，如果为None则获取所有事件
            batch_size: 每批处理的区块数量
            
        Yields:
            List[Dict[str, Any]]: 每批的事件数据
        """
        if not start_block:
            start_block = 0
        
        if not end_block:
            # 获取最新区块号（使用重试机制处理429错误）
            try:
                end_block = self._retry_on_429(self.w3.eth.block_number)
                logger.info(f"获取最新区块号: {end_block}")
            except Exception as e:
                logger.error(f"获取最新区块号失败: {e}")
                return
        
        logger.info(f"开始获取事件: 合约地址={self.contract_address}, 区块范围={start_block}-{end_block}")
        
        current_block = start_block
        total_events = 0
        
        while current_block <= end_block:
            batch_end = min(current_block + batch_size - 1, end_block)
            
            try:
                # 构建过滤条件
                filter_params = {
                    'fromBlock': current_block,
                    'toBlock': batch_end,
                    'address': self.contract_address
                }
                
                # 如果指定了事件签名，添加到过滤条件
                if event_signatures:
                    filter_params['topics'] = [event_signatures]
                
                # 获取事件日志
                logs = self.w3.eth.get_logs(filter_params)
                
                if logs:
                    # 处理事件数据
                    events = []
                    for log in logs:
                        event_data = {
                            'block_number': log['blockNumber'],
                            'block_hash': log['blockHash'].hex(),
                            'transaction_hash': log['transactionHash'].hex(),
                            'transaction_index': log['transactionIndex'],
                            'log_index': log['logIndex'],
                            'address': log['address'],
                            'topics': [topic.hex() for topic in log['topics']],
                            'data': log['data'],
                            'removed': log.get('removed', False)
                        }
                        
                        # 尝试解析事件名称和签名
                        if log['topics']:
                            event_signature = log['topics'][0].hex()
                            event_data['event_signature'] = event_signature
                            
                            # 尝试从事件签名解析事件名称
                            event_name = self._get_event_name_from_signature(event_signature)
                            event_data['event_name'] = event_name
                        
                        events.append(event_data)
                    
                    total_events += len(events)
                    logger.info(f"获取区块 {current_block}-{batch_end} 的事件: {len(events)} 条，累计 {total_events} 条")
                    
                    yield events
                else:
                    logger.info(f"区块 {current_block}-{batch_end} 无事件")
                
            except Exception as e:
                error_msg = str(e)
                logger.error(f"获取区块 {current_block}-{batch_end} 的事件失败: {e}")
                
                # 检查是否是429错误（请求过多）
                if "429" in error_msg and "Too Many Requests" in error_msg:
                    logger.warning(f"遇到429错误，休息1秒后重试...")
                    time.sleep(1)
                    
                    # 重试一次
                    try:
                        logs = self.w3.eth.get_logs(filter_params)
                        
                        if logs:
                            # 处理事件数据
                            events = []
                            for log in logs:
                                event_data = {
                                    'block_number': log['blockNumber'],
                                    'block_hash': log['blockHash'].hex(),
                                    'transaction_hash': log['transactionHash'].hex(),
                                    'transaction_index': log['transactionIndex'],
                                    'log_index': log['logIndex'],
                                    'address': log['address'],
                                    'topics': [topic.hex() for topic in log['topics']],
                                    'data': log['data'],
                                    'removed': log.get('removed', False)
                                }
                                
                                # 尝试解析事件名称和签名
                                if log['topics']:
                                    event_signature = log['topics'][0].hex()
                                    event_data['event_signature'] = event_signature
                                    
                                    # 尝试从事件签名解析事件名称
                                    event_name = self._get_event_name_from_signature(event_signature)
                                    event_data['event_name'] = event_name
                                
                                events.append(event_data)
                            
                            total_events += len(events)
                            logger.info(f"重试成功！获取区块 {current_block}-{batch_end} 的事件: {len(events)} 条，累计 {total_events} 条")
                            
                            yield events
                        else:
                            logger.info(f"重试后区块 {current_block}-{batch_end} 仍无事件")
                    
                    except Exception as retry_e:
                        logger.error(f"重试获取区块 {current_block}-{batch_end} 的事件仍然失败: {retry_e}")
                        raise Exception(f"获取区块 {current_block}-{batch_end} 的事件失败: {e}")
                else:
                    # 非429错误，直接抛出异常
                    raise Exception(f"获取区块 {current_block}-{batch_end} 的事件失败: {e}")
                
            current_block = batch_end + 1
        
        logger.info(f"完成事件获取，总共获取 {total_events} 条事件")

    def _get_event_name_from_signature(self, event_signature: str) -> Optional[str]:
        """
        从事件签名获取事件名称（如果可能的话）
        
        Args:
            event_signature: 事件签名
            
        Returns:
            str: 事件名称，如果无法解析则返回None
        """
        # 这里可以添加事件签名字典来解析事件名称
        # 例如：Transfer(address,address,uint256) -> keccak256 -> 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef
        event_signatures = {
            '0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef': 'Transfer',
            '0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925': 'Approval',
            '0x17307eab39ab6107e8899845ad3d59bd9653f200f220920489ca2b5937696c31': 'ApprovalForAll',
            # 可以添加更多常见的事件签名
        }
        
        return event_signatures.get(event_signature)

    def save_events_to_db(self, events: List[Dict[str, Any]]) -> int:
        """
        将事件数据保存到数据库
        
        Args:
            events: 事件数据列表
            
        Returns:
            int: 成功保存的事件数量
        """
        if not events:
            return 0
        
        if not self.db_connection:
            self.connect_db()
        
        cursor = self.db_connection.cursor()
        
        try:
            # 批量插入事件数据
            insert_query = f"""
            INSERT INTO {self.table_name} 
            (block_number, block_hash, transaction_hash, transaction_index, log_index, 
             address, event_name, event_signature, topics, data, removed)
            VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
            ON CONFLICT (block_number, transaction_hash, log_index) 
            DO UPDATE SET 
                block_hash = EXCLUDED.block_hash,
                transaction_index = EXCLUDED.transaction_index,
                address = EXCLUDED.address,
                event_name = EXCLUDED.event_name,
                event_signature = EXCLUDED.event_signature,
                topics = EXCLUDED.topics,
                data = EXCLUDED.data,
                removed = EXCLUDED.removed
            """
            
            # 准备数据
            data_to_insert = []
            for event in events:
                data_to_insert.append((
                    event['block_number'],
                    event['block_hash'],
                    event['transaction_hash'],
                    event['transaction_index'],
                    event['log_index'],
                    event['address'],
                    event.get('event_name'),
                    event.get('event_signature'),
                    json.dumps(event['topics']),
                    event['data'],
                    event['removed']
                ))
            
            # 执行批量插入
            cursor.executemany(insert_query, data_to_insert)
            self.db_connection.commit()
            
            saved_count = len(events)
            logger.info(f"成功保存 {saved_count} 条事件到数据库")
            return saved_count
            
        except Exception as e:
            self.db_connection.rollback()
            logger.error(f"保存事件到数据库失败: {e}")
            raise
        finally:
            cursor.close()

    def get_events_from_db(self, start_block=None, end_block=None, 
                          event_signature=None, limit=1000, page=1) -> List[Dict[str, Any]]:
        """
        从数据库查询事件数据
        
        Args:
            start_block: 起始区块号
            end_block: 结束区块号
            event_signature: 事件签名
            limit: 限制返回数量
            page: 页码
            
        Returns:
            List[Dict[str, Any]]: 事件数据列表
        """
        if not self.db_connection:
            self.connect_db()
        
        cursor = self.db_connection.cursor(cursor_factory=RealDictCursor)
        
        query = f"SELECT * FROM {self.table_name} WHERE 1=1"
        params = []
        
        if start_block:
            query += " AND block_number >= %s"
            params.append(start_block)
        
        if end_block:
            query += " AND block_number <= %s"
            params.append(end_block)
        
        if event_signature:
            query += " AND event_signature = %s"
            params.append(event_signature)
        
        # 分页
        offset = (page - 1) * limit
        query += " ORDER BY block_number ASC, transaction_index ASC, log_index ASC LIMIT %s OFFSET %s"
        params.extend([limit, offset])
        
        try:
            cursor.execute(query, params)
            results = cursor.fetchall()
            
            # 转换结果
            events = []
            for row in results:
                event = dict(row)
                # 解析topics JSON
                if event['topics']:
                    event['topics'] = json.loads(event['topics'])
                events.append(event)
            
            logger.info(f"查询到 {len(events)} 条事件")
            return events
            
        except Exception as e:
            logger.error(f"查询事件数据失败: {e}")
            raise
        finally:
            cursor.close()

    def get_total_event_count(self, start_block=None, end_block=None, event_signature=None) -> int:
        """
        获取事件总数
        
        Args:
            start_block: 起始区块号
            end_block: 结束区块号
            event_signature: 事件签名
            
        Returns:
            int: 事件总数
        """
        if not self.db_connection:
            self.connect_db()
        
        cursor = self.db_connection.cursor()
        
        query = f"SELECT COUNT(*) FROM {self.table_name} WHERE 1=1"
        params = []
        
        if start_block:
            query += " AND block_number >= %s"
            params.append(start_block)
        
        if end_block:
            query += " AND block_number <= %s"
            params.append(end_block)
        
        if event_signature:
            query += " AND event_signature = %s"
            params.append(event_signature)
        
        try:
            cursor.execute(query, params)
            result = cursor.fetchone()
            total_count = result[0] if result else 0
            logger.info(f"事件总数: {total_count}")
            return total_count
            
        except Exception as e:
            logger.error(f"获取事件总数失败: {e}")
            raise
        finally:
            cursor.close()

    def close_connection(self):
        """
        手动关闭数据库连接
        """
        if self.db_connection:
            self.db_connection.close()
            self.db_connection = None
            logger.info("数据库连接已关闭")

    def __del__(self):
        """
        析构函数，确保在对象销毁时关闭数据库连接
        """
        self.close_connection()

    def fetch_and_save_all_events(self, start_block=None, end_block=None, 
                                 event_signatures=None, batch_size=100000, 
                                 callback=None) -> int:
        """
        获取并保存所有事件到数据库
        
        Args:
            start_block: 起始区块号
            end_block: 结束区块号
            event_signatures: 事件签名列表
            batch_size: 每批处理的区块数量
            callback: 回调函数，用于处理每批事件，参数为(batch_num, events)
            
        Returns:
            int: 总共保存的事件数量
        """
        total_saved = 0
        batch_num = 1
        
        logger.info(f"开始获取并保存事件: 合约地址={self.contract_address}")
        
        try:
            # 获取事件并保存
            for events_batch in self.get_all_events_from_infra(
                start_block=start_block,
                end_block=end_block,
                event_signatures=event_signatures,
                batch_size=batch_size
            ):
                if events_batch:
                    # 如果提供了回调函数，先执行回调
                    if callback:
                        try:
                            callback(batch_num, events_batch)
                        except Exception as e:
                            logger.error(f"回调函数执行失败 (第{batch_num}批): {e}")
                    
                    # 保存到数据库
                    saved_count = self.save_events_to_db(events_batch)
                    total_saved += saved_count
                    
                    logger.info(f"第{batch_num}批: 获取{len(events_batch)}条事件，保存{saved_count}条")
                
                batch_num += 1
            
            logger.info(f"完成事件获取和保存，总共保存 {total_saved} 条事件")
            return total_saved
            
        except Exception as e:
            logger.error(f"获取并保存事件失败: {e}")
            raise