package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/latifrons/soccerdash"
)

// TCP通信全局变量
const (
	Host    = "127.0.0.1" /* 本地IP地址 */
	Port    = 2020        /* 本地端口 */
	MsgLen  = 4096        /* 消息长度上界 */
	ChanCap = 7           /* 消息读入管道容量 */
)

// BlockInfo 最新节点信息
type BlockInfo struct {
	Hash string `json:"Hash"` /* 区块哈希 */
}

// NodeInfo 节点信息类型
type NodeInfo struct {
	Addr             net.Addr /* 地址 */
	LatestBlockHash  string   /* 最新区块哈希 */
	LatestBlockTime  int      /* 最新区块时间 */
	BroadcastTime    [100]int /* 最近100次区块广播时间 */
	Filled           bool     /* 区块广播是否满100次 */
	Rank             int8     /* 最旧或第100新区块广播时间，等待被更新成最新区块广播时间 */
	AvgBroadcastTime float64  /* 平均区块广播时间 */
	// 最新区块广播时间：BroadcastTime[Rank-1]
}

var nodeMap map[string]NodeInfo  /* 键：节点名，值：节点信息 */
var blockPushTime map[string]int /* 键：区块哈希，值：最早收到推送时间 */

func main() {
	address := Host + ":" + strconv.Itoa(Port)
	ln, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Println(err)
		return
	}
	nodeMap = make(map[string]NodeInfo)
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
	nodeName := ""                         /* 节点名 */
	version := ""                          /* 节点版本 */
	nodeDelay := ""                        /* 节点延迟 */
	connNum := ""                          /* 节点连接数 */
	txPoolNum := ""                        /* 待处理交易 */
	nodeAddr := conn.RemoteAddr()          /* 节点地址 */
	readChan := make(chan []byte, ChanCap) /* 读消息 */
	countChan := make(chan int, ChanCap)   /* 消息数目 */
	go readMsg(conn, readChan, countChan)
	receivedCount := 0
	for {
		select {
		case msg := <-readChan:
			receivedCount++
			var msgObj soccerdash.Message
			err := json.Unmarshal(msg, &msgObj) /* 反序列化 */
			if err != nil {
				fmt.Println(err)
				return
			}
			switch msgObj.Key {
			case "NodeName":
				if nodeName == "" {
					nodeName = msgObj.Value.(string)
				}
				break
			case "Version":
				if version == "" {
					version = msgObj.Value.(string)
				}
				break
			case "NodeDelay":
				if nodeDelay == "" {
					nodeDelay = msgObj.Value.(string)
				}
				break
			case "ConnNum":
				if connNum == "" {
					connNum = msgObj.Value.(string)
				}
				break
			case "LatestSequencer":
				currentTime := time.Now().Nanosecond()
				var blockInfoObj BlockInfo
				err := json.Unmarshal(msgObj.Value.([]byte), &blockInfoObj)
				if err != nil {
					fmt.Println(err)
					break
				}
				if _, blockFound := blockPushTime[blockInfoObj.Hash]; !blockFound /* 出块尚未被记录 */ {
					blockPushTime[blockInfoObj.Hash] = currentTime
				}
				if ni, nodeFound := nodeMap[nodeName]; nodeFound { /* 节点已经被记录 */
					ni.update(blockInfoObj.Hash, currentTime, blockPushTime[blockInfoObj.Hash])
				} else /* 节点尚未被记录 */ {
					nodeMap[nodeName] = *newNodeInfo(nodeAddr, blockInfoObj.Hash, currentTime, blockPushTime[blockInfoObj.Hash])
				}
				break
			case "IsProducer":
				// TODO 出块委员会
				// 新建出块委员会节点向量；
				// 如果true且这个节点不属于出块委员会向量，那么压入；如果false且属于，那么擦除。
				break
			case "TxPoolNum":
				txPoolNum = msgObj.Value.(string)
				if txPoolNum[0] == '-' {
					return
				}
				break
			default:
				fmt.Println("Msg format err!")
			}
		case sentCount := <-countChan:
			if sentCount == receivedCount {
				break
			} else {
				continue
			}
		}
	}
}

func readMsg(conn net.Conn, readChan chan<- []byte, countChan chan<- int) {
	count := 0
	for {
		r := bufio.NewReader(conn)
		msg := make([]byte, MsgLen)
		msg, _, err := r.ReadLine()
		if err != nil {
			fmt.Println(err)
			break
		}
		readChan <- msg
		count++
	}
	countChan <- count
}

func newNodeInfo(addr net.Addr, bh string, ct int, bpt int) *NodeInfo {
	bt := ct - bpt
	return &NodeInfo{addr, bh, bt, [100]int{0: bt}, false, 1, float64(bt)}
}

func (ni *NodeInfo) update(bh string, ct int, bpt int) {
	ni.LatestBlockHash = bh
	ni.LatestBlockTime = ct
	bt := ct - bpt
	if ni.Filled {
		ni.AvgBroadcastTime = (ni.AvgBroadcastTime*100 + float64(bt-ni.BroadcastTime[ni.Rank])) / 100
	} else {
		ni.AvgBroadcastTime = (ni.AvgBroadcastTime*float64(ni.Rank) + float64(bt)) / (float64(ni.Rank) + 1)
		if ni.Rank == 99 {
			ni.Filled = true
		}
	}
	if ni.Rank == 99 {
		ni.Rank = 0
	} else {
		ni.Rank++
	}
	ni.BroadcastTime[ni.Rank] = bt
}
