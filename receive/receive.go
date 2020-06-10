package main

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"time"
)

// TCP通信全局变量
const (
	Host   = "127.0.0.1" /* 本地IP地址 */
	Port   = 2020        /* 本地端口 */
	MsgLen = 64          /* 消息长度上界 */
)

// Message 消息类型
type Message struct {
	NodeName string `json:"nodename"` /* 节点名 */
	BlockID  int64  `json:"blockid"`  /* 区块编号 */
}

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
	readChan := make(chan []byte) /* 读消息 */
	endChan := make(chan bool)    /* 读消息结束 */
	go readMsg(conn, readChan, endChan)
	for {
		select {
		case msg := <-readChan:
			currentTime := int64(time.Now().Nanosecond())
			var obj Message
			err := json.Unmarshal(msg, &obj) /* 反序列化 */
			if err != nil {
				fmt.Println(err)
				return
			}
			// fmt.Println("Node name:", obj.NodeName)
			// fmt.Println("Block ID:", obj.BlockID)
			nodeAddr := conn.RemoteAddr()
			if _, blockFound := blockPushTime[obj.BlockID]; !blockFound /* 出块尚未被记录 */ {
				blockPushTime[obj.BlockID] = currentTime
			}
			if ni, nodeFound := nodeMap[obj.NodeName]; nodeFound { /* 节点已经被记录 */
				ni.update(obj.BlockID, currentTime, blockPushTime[obj.BlockID])
			} else /* 节点尚未被记录 */ {
				nodeMap[obj.NodeName] = *newNodeInfo(nodeAddr, obj.BlockID, currentTime, blockPushTime[obj.BlockID])
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

func readMsg(conn net.Conn, readChan chan<- []byte, endChan chan<- bool) {
	for {
		msg := make([]byte, MsgLen)
		_, err := conn.Read(msg)
		if err != nil {
			fmt.Println(err)
			break
		}
		readChan <- msg
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
