package main

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"

	"github.com/latifrons/soccerdash"
)

// TCP通信全局变量
const (
	Host   = "127.0.0.1" /* 本地IP地址 */
	Port   = 2020        /* 本地端口 */
	MsgLen = 1024        /* 消息长度上界 */
)

// NodeInfo 节点信息类型
type NodeInfo struct {
	Addr             net.Addr   /* 地址 */
	LatestBlockID    int64      /* 最新区块编号 */
	LatestBlockTime  int64      /* 最新区块时间 */
	BroadcastTime    [100]int64 /* 最近100次区块广播时间 */
	Filled           bool       /* 区块广播是否满100次 */
	Rank             int8       /* 最旧或第100新区块广播时间，等待被更新成最新区块广播时间 */
	AvgBroadcastTime float64    /* 平均区块广播时间 */
	// 最新区块广播时间：BroadcastTime[Rank-1]
}

var nodeMap map[string]NodeInfo   /* 键：节点名，值：节点信息 */
var blockPushTime map[int64]int64 /* 键：区块编号，值：最早收到推送时间 */

func main() {
	address := Host + ":" + strconv.Itoa(Port)
	ln, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Println(err)
		return
	}
	nodeMap = make(map[string]NodeInfo)
	blockPushTime = make(map[int64]int64)
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
	defer conn.Close()
	readChan := make(chan []byte, MsgLen<<3) /* 读消息 */
	lenChan := make(chan int)                /* 消息长度 */
	endChan := make(chan bool)               /* 读消息结束 */
	go readMsg(conn, readChan, lenChan, endChan)
	for {
		select {
		case msg := <-readChan:
			// currentTime := int64(time.Now().Nanosecond())
			// fmt.Println(msg)
			var msgObj soccerdash.Message
			len := <-lenChan
			err := json.Unmarshal(msg[:len], &msgObj) /* 反序列化 */
			if err != nil {
				fmt.Println(err)
				return
			}
			switch msgObj.Key {
			case "NodeName":
				// TODO 节点名
				fmt.Println("NodeName:", msgObj.Value)
				break
			case "Version":
				// TODO 运行版本
				fmt.Println("Version:", msgObj.Value)
				break
			case "NodeDelay":
				// TODO 节点延迟
				fmt.Println("NodeDelay:", msgObj.Value)
				break
			case "ConnNum":
				// TODO 连接数
				fmt.Println("ConnNum:")
				break
			case "LatestSequencer":
				// TODO 最新区块
				// nodeAddr := conn.RemoteAddr()
				// if _, blockFound := blockPushTime[obj.BlockID]; !blockFound /* 出块尚未被记录 */ {
				// 	blockPushTime[obj.BlockID] = currentTime
				// }
				// if ni, nodeFound := nodeMap[obj.NodeName]; nodeFound { /* 节点已经被记录 */
				// 	ni.update(obj.BlockID, currentTime, blockPushTime[obj.BlockID])
				// } else /* 节点尚未被记录 */ {
				// 	nodeMap[obj.NodeName] = *newNodeInfo(nodeAddr, obj.BlockID, currentTime, blockPushTime[obj.BlockID])
				// }
				fmt.Println("LatestSequencer:")
				break
			case "IsProducer":
				// TODO 出块委员会
				fmt.Println("IsProducer:")
				break
			case "TxPoolNum":
				// TODO 待处理交易
				fmt.Println("TxPoolNum:")
				break
			default:
				fmt.Println("Msg format err!")
			}
		case end := <-endChan:
			if end {
				break
			}
		default:
			break
		}
	}
}

func readMsg(conn net.Conn, readChan chan<- []byte, lenChan chan<- int, endChan chan<- bool) {
	for {
		msg := make([]byte, MsgLen)
		len, err := conn.Read(msg)
		if err != nil {
			fmt.Println(err)
			break
		}
		readChan <- msg
		lenChan <- len
	}
	endChan <- true
}

func newNodeInfo(addr net.Addr, bid int64, ct int64, bpt int64) *NodeInfo {
	bt := ct - bpt
	return &NodeInfo{addr, bid, bt, [100]int64{0: bt}, false, 1, float64(bt)}
}

func (ni *NodeInfo) update(bid int64, ct int64, bpt int64) {
	ni.LatestBlockID = bid
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
