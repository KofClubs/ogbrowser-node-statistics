package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/Shopify/sarama"
	"github.com/latifrons/soccerdash"
)

// TCP通信全局变量
// TODO 移动到配置文件
const (
	Host    = "0.0.0.0" /* 测试IP地址 */
	Port    = 2020      /* 本地端口 */
	ChanCap = 30        /* 消息读入管道容量 */
)

// BlockInfo 最新节点信息
// 键"LatestSequencer"对应的值，给出了被需要的字段
type BlockInfo struct {
	Hash   string `json:"Hash"`   /* 区块哈希 */
	Height int64  `json:"Height"` /* 区块高度 */
}

// NodeInfo 节点信息类型
// 键节点地址对应的值
type NodeInfo struct {
	NodeName         string   /* 节点名 */
	Version          string   /* 运行版本 */
	ConnNum          string   /* 连接数 */
	LatestBlock      string   /* 最新区块编号（高度） */
	LatestBlockHash  string   /* 最新区块哈希 */
	LatestBlockTime  int      /* 最新区块时间 */
	BroadcastTime    [100]int /* 最近100次区块广播时间 */
	Filled           bool     /* 区块广播是否满100次 */
	Rank             int8     /* 最旧或第100新区块广播时间，等待被更新成最新区块广播时间 */
	AvgBroadcastTime int      /* 平均区块广播时间 */
	IsProducer       bool     /* 是否属于出块委员会 */
	TxPoolNum        string   /* 待处理交易 */
	// 最新区块广播时间：BroadcastTime[Rank-1]
}

// Node 被压入kafka队列的节点信息
// 暂时不实现节点延迟
type Node struct {
	NodeName string `json:"node_name"` /* 节点名 */
	NodeIP   string `json:"node_ip"`   /* IP地址 */
	Version  string `json:"version"`   /* 运行版本 */
	// NodeDelay           int64  `json:"node_delay"`
	ConnNum             int64  `json:"conn_num"`              /* 连接数 */
	LatestBlock         string `json:"latest_block"`          /* 最新区块编号（高度） */
	LatestBlockHash     string `json:"latest_block_hash"`     /* 最新区块哈希 */
	LatestBlockTime     int64  `json:"latest_block_time"`     /* 最新区块时间 */
	BroadcastTime       int64  `json:"broadcast_time"`        /* 最新区块广播时间 */
	AvgBroadcastTime    int64  `json:"avg_broadcast_time"`    /* 平均区块广播时间 */
	IsProducer          bool   `json:"is_producer"`           /* 是否属于出块委员会 */
	PendingTransactions int64  `json:"pending_transactions" ` /* 待处理交易 */
}

var nodeMap map[net.Addr]NodeInfo /* 键：节点地址，值：节点信息 */
var blockPushTime map[string]int  /* 键：区块哈希，值：最早收到推送时间 */

func main() {
	address := Host + ":" + strconv.Itoa(Port)
	ln, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Println(err)
		return
	}
	nodeMap = make(map[net.Addr]NodeInfo)
	blockPushTime = make(map[string]int)
	defer ln.Close()
	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	// TODO 节点-区块信息归类
	defer conn.Close()
	nodeAddr := conn.RemoteAddr() /* 节点地址 */
	if _, isOldNode := nodeMap[nodeAddr]; !isOldNode {
		nodeMap[nodeAddr] = *newNodeInfo()
	}
	readChan := make(chan []byte, ChanCap) /* 读消息 */
	countChan := make(chan int)            /* 消息数目 */
	go readMsg(conn, readChan, countChan)
	ni, _ := nodeMap[nodeAddr]
	receivedCount := 0
	for {
		select {
		case msg := <-readChan:
			receivedCount++
			var msgObj soccerdash.Message
			err := json.Unmarshal(msg, &msgObj) /* 反序列化 */
			if err != nil {
				fmt.Println(err)
				break
			}
			// fmt.Println(msgObj.Key)
			switch msgObj.Key {
			case "NodeName":
				if ni.NodeName == "" {
					ni.NodeName = msgObj.Value.(string)
				}
				break
			case "Version":
				if ni.Version == "" {
					ni.Version = msgObj.Value.(string)
				}
				break
			case "ConnNum":
				if ni.ConnNum == "" {
					ni.ConnNum = msgObj.Value.(string)
				}
				break
			case "LatestSequencer":
				currentTime := time.Now().Nanosecond()
				var blockInfoObj BlockInfo
				err := json.Unmarshal([]byte(msgObj.Value.(string)), &blockInfoObj)
				if err != nil {
					fmt.Println(err)
					break
				}
				if _, isOldBlock := blockPushTime[blockInfoObj.Hash]; !isOldBlock /* 出块尚未被记录 */ {
					blockPushTime[blockInfoObj.Hash] = currentTime
				}
				bt := currentTime - blockPushTime[blockInfoObj.Hash] /* 广播时间=当前时间-区块最早被记录时间 */
				if ni.LatestBlockHash == "" /* 节点的区块信息尚无记录 */ {
					ni.AvgBroadcastTime = bt
				} else /* 节点的区块信息已经有记录 */ {
					if ni.Filled {
						ni.AvgBroadcastTime = (ni.AvgBroadcastTime*100 + bt - ni.BroadcastTime[ni.Rank]) / 100
					} else {
						ni.AvgBroadcastTime = (ni.AvgBroadcastTime*int(ni.Rank) + bt) / (int(ni.Rank) + 1)
						if ni.Rank == 99 {
							ni.Filled = true
						}
					}
				}
				ni.LatestBlock = strconv.FormatInt(blockInfoObj.Height, 10)
				ni.LatestBlockHash = blockInfoObj.Hash
				ni.LatestBlockTime = currentTime
				ni.BroadcastTime[ni.Rank] = bt
				if ni.Rank == 99 {
					ni.Rank = 0
				} else {
					ni.Rank++
				}
				break
			case "IsProducer":
				ni.IsProducer = msgObj.Value.(bool)
				break
			case "TxPoolNum":
				ni.TxPoolNum = msgObj.Value.(string)
				break
			default:
				fmt.Println("Message format err!")
			}
		case sentCount := <-countChan:
			if sentCount == receivedCount {
				break
			} else {
				continue
			}
		}
		if ni.allowedToSend() {
			kafkaProducer(convert2KafkaMsg(nodeAddr, &ni))
		}
	}
}

func readMsg(conn net.Conn, readChan chan<- []byte, countChan chan<- int) {
	count := 0
	for {
		r := bufio.NewReader(conn)
		msg, err := r.ReadBytes('\n')
		if err != nil {
			fmt.Println(err)
			break
		}
		readChan <- msg
		count++
	}
	countChan <- count
}

func newNodeInfo() *NodeInfo {
	return &NodeInfo{"", "", "", "", "", -1, [100]int{}, false, 0, 0, false, ""}
}

func (ni *NodeInfo) allowedToSend() bool {
	return ni.NodeName != "" && ni.Version != "" && ni.ConnNum != "" && ni.LatestBlock != "" && ni.LatestBlockTime > 0
}

func convert2KafkaMsg(addr net.Addr, ni *NodeInfo) []byte {
	n := Node{ni.NodeName, addr.String(), ni.Version, 0, ni.LatestBlock, ni.LatestBlockHash, 0, 0, 0, ni.IsProducer, 0}
	connNum, err := strconv.ParseInt(ni.ConnNum, 10, 64)
	if err != nil {
		return nil
	}
	n.ConnNum = connNum
	n.LatestBlockTime = int64(ni.LatestBlockTime)
	n.BroadcastTime = int64(ni.BroadcastTime[ni.Rank-1])
	n.AvgBroadcastTime = int64(ni.AvgBroadcastTime)
	pt, err := strconv.ParseInt(ni.TxPoolNum, 10, 64)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	n.PendingTransactions = pt
	b, err := json.Marshal(n)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	// fmt.Println(string(b))
	// fmt.Println("")
	return b
}

func kafkaProducer(content []byte) {
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Partitioner = sarama.NewRandomPartitioner
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true
	config.Version = sarama.V0_11_0_2
	producer, err := sarama.NewAsyncProducer([]string{"nbstock.top:30040"}, config)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer producer.AsyncClose()
	msg := &sarama.ProducerMessage{
		Topic: "nodeinfo",
		Value: sarama.ByteEncoder(content),
	}
	producer.Input() <- msg
	select {
	case <-producer.Successes():
		// fmt.Println("offset: ", suc.Offset, "timestamp: ", suc.Timestamp.String(), "partitions: ", suc.Partition)
	case fail := <-producer.Errors():
		fmt.Println(fail.Err)
	}
}
