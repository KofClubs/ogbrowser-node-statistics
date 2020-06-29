package main

import (
	"bufio"
	"encoding/json"
	"github.com/annchain/OG/types/tx_types"
	"github.com/latifrons/soccerdash"
	"github.com/sirupsen/logrus"
	"net"
	"time"
)

type Node struct {
	address string
	conn    net.Conn

	info          *NodeInfo
	broadcastInfo *BroadcastInfo
	confirmTimes  *OrderedMap /* 最近100次区块广播时间 */

	server *Server
}

func NewNode(conn net.Conn, server *Server) *Node {
	n := &Node{}

	n.address = conn.RemoteAddr().String() /* 节点地址 */
	n.conn = conn

	n.info = newNodeInfo(n.address)
	n.broadcastInfo = newBroadcastInfo(100)
	n.confirmTimes = NewOrderedMap(100)

	n.server = server

	go n.readLoop()
	return n
}

func (node *Node) Close() {
	node.conn.Close()
	node.server.RemoveNode(node)
}

func (node *Node) readLoop() {
	r := bufio.NewReader(node.conn)
	for {
		msg, err := r.ReadBytes('\n')
		if err != nil {
			logrus.Errorf("node: %s, read bytes error: %v. node will be removed", node.address, err)
			node.Close()
			return
		}

		logrus.Tracef("msg: %s", string(msg))
		node.readMsg(msg)
	}
}

func (node *Node) readMsg(msg []byte) {
	var msgObj soccerdash.Message
	err := json.Unmarshal(msg, &msgObj) /* 反序列化 */
	if err != nil {
		logrus.Errorf("unmarshal msg error: %v", err)
		return
	}
	// fmt.Println(msgObj.Key)
	switch msgObj.Key /* 消息的键 */ {
	// 节点名
	case "nodeName":
		node.info.nodeName = msgObj.Value.(string)
		break
	// 版本
	case "version":
		node.info.version = msgObj.Value.(string)
		break
	// 连接数
	case "connNum":
		node.info.connNum = msgObj.Value.(int)
		break
	// 最新区块
	case "LatestSequencer":
		currentTime := time.Now()
		var blockInfoObj tx_types.Sequencer
		err := json.Unmarshal([]byte(msgObj.Value.(string)), &blockInfoObj)
		if err != nil {
			logrus.Error(err)
			break
		}

		hash := blockInfoObj.Hash.Hex()
		broadcastTime := node.server.InsertOrIgnoreConfirmTime(hash, currentTime)
		node.broadcastInfo.Add(hash, broadcastTime)
		node.confirmTimes.Add(hash, currentTime)

		node.info.latestBroadcastTime = broadcastTime
		node.info.avgBroadcastTime = node.broadcastInfo.AvgTime()
		node.info.latestBlockHeight = blockInfoObj.Height
		node.info.latestBlockHash = blockInfoObj.Hash.String()
		// 区块时间，当前使用接收时间，应该使用区块时间戳
		node.info.latestBlockTime = currentTime
		break
	// 是否属于出块委员会
	case "isProducer":
		node.info.isProducer = msgObj.Value.(bool)
		break
	// 待处理交易
	case "txPoolNum":
		node.info.txPoolNum = msgObj.Value.(int)
		break
	default:
		logrus.Errorf("Unknown message key: %s", msgObj.Key)
	}

	if !node.info.allowedToSend() {
		node.server.sendKafkaMsg(node.info.toKafkaMsg())
	}

}

type BroadcastInfo struct {
	total time.Duration
	times *OrderedMap
}

func newBroadcastInfo(cap int) *BroadcastInfo {
	return &BroadcastInfo{
		total: 0,
		times: NewOrderedMap(cap),
	}
}

func (b *BroadcastInfo) Add(hash string, broadcastTime time.Duration) {
	removedTimeI := b.times.Add(hash, broadcastTime)
	if removedTimeI != nil {
		b.total -= removedTimeI.(time.Duration)
	}
	b.total += broadcastTime
}

func (b *BroadcastInfo) AvgTime() time.Duration {
	return b.total / time.Duration(b.times.Len())
}

// NodeInfo 节点信息类型
// 键节点地址对应的值
type NodeInfo struct {
	address             string
	nodeName            string    /* 节点名 */
	version             string    /* 运行版本 */
	connNum             int       /* 连接数 */
	latestBlockHeight   uint64    /* 最新区块编号（高度） */
	latestBlockHash     string    /* 最新区块哈希 */
	latestBlockTime     time.Time /* 最新区块时间 */
	latestBroadcastTime time.Duration
	avgBroadcastTime    time.Duration /* 平均区块广播时间 */
	isProducer          bool          /* 是否属于出块委员会 */
	txPoolNum           int           /* 待处理交易 */
}

func newNodeInfo(address string) *NodeInfo {
	ni := &NodeInfo{
		address:             address,
		nodeName:            "",
		version:             "",
		connNum:             0,
		latestBlockHeight:   0,
		latestBlockHash:     "",
		latestBlockTime:     time.Now(),
		latestBroadcastTime: time.Duration(0),
		avgBroadcastTime:    0,
		isProducer:          false,
		txPoolNum:           0,
	}
	return ni
}

func (ni *NodeInfo) allowedToSend() bool {
	return ni.nodeName != "" && ni.version != "" && ni.connNum != 0 && ni.latestBlockHeight != 0
}

func (ni *NodeInfo) toKafkaMsg() []byte {
	nodeKafka := NodeKafka{}

	nodeKafka.NodeName = ni.nodeName
	nodeKafka.NodeIP = ni.address
	nodeKafka.Version = ni.version
	nodeKafka.ConnNum = ni.connNum
	nodeKafka.LatestBlock = string(ni.latestBlockHeight)
	nodeKafka.LatestBlockHash = ni.latestBlockHash
	nodeKafka.LatestBlockTime = ni.latestBlockTime.Nanosecond() / 1e6
	nodeKafka.BroadcastTime = int(ni.latestBroadcastTime.Nanoseconds() / 1e6)
	nodeKafka.AvgBroadcastTime = int(ni.avgBroadcastTime.Nanoseconds() / 1e6)
	nodeKafka.IsProducer = ni.isProducer
	nodeKafka.PendingTransactions = ni.txPoolNum

	b, err := json.Marshal(nodeKafka)
	if err != nil {
		logrus.Errorf("marshal to kafka msg error: %v", err)
		return nil
	}
	return b
}

// NodeKafka 被压入kafka队列的节点信息
// 暂时不实现节点延迟
type NodeKafka struct {
	NodeName string `json:"node_name"` /* 节点名 */
	NodeIP   string `json:"node_ip"`   /* IP地址 */
	Version  string `json:"version"`   /* 运行版本 */
	// NodeDelay           int64  `json:"node_delay"`
	ConnNum             int    `json:"conn_num"`              /* 连接数 */
	LatestBlock         string `json:"latest_block"`          /* 最新区块编号（高度） */
	LatestBlockHash     string `json:"latest_block_hash"`     /* 最新区块哈希 */
	LatestBlockTime     int    `json:"latest_block_time"`     /* 最新区块时间 */
	BroadcastTime       int    `json:"broadcast_time"`        /* 最新区块广播时间 */
	AvgBroadcastTime    int    `json:"avg_broadcast_time"`    /* 平均区块广播时间 */
	IsProducer          bool   `json:"is_producer"`           /* 是否属于出块委员会 */
	PendingTransactions int    `json:"pending_transactions" ` /* 待处理交易 */
}
