package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/latifrons/soccerdash"
	"github.com/sirupsen/logrus"
	"net"
	"sync"
	"time"
)

const (
	SDKeyNodeName        = "NodeName"
	SDKeyVersion         = "Version"
	SDKeyConnNum         = "ConnNum"
	SDKeyLatestSequencer = "LatestSequencer"
	SDKeyIsProducer      = "IsProducer"
	SDKeyTxPoolNum       = "TxPoolNum"
)

type Node struct {
	address string
	conn    net.Conn

	info          *NodeInfo
	broadcastInfo *BroadcastInfo
	confirmTimes  *OrderedMap /* 最近100次区块广播时间 */
	sendFlag      bool
	sendMu        sync.RWMutex

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
	go n.sendCheck()
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
	case SDKeyNodeName:
		nodeName := msgObj.Value.(string)
		if node.info.nodeName == nodeName {
			return
		}
		node.info.nodeName = nodeName
		break
	// 版本
	case SDKeyVersion:
		version := msgObj.Value.(string)
		if node.info.version == version {
			return
		}
		node.info.version = version
		break
	// 连接数
	case SDKeyConnNum:
		connNum := int(msgObj.Value.(float64))
		if node.info.connNum == connNum {
			return
		}
		node.info.connNum = connNum
		break
	// 最新区块
	case SDKeyLatestSequencer:
		seqMap, ok := msgObj.Value.(map[string]interface{})
		if !ok {
			logrus.Errorf("cannot convert seq to map")
			return
		}

		hash := seqMap["Hash"].(string)
		height := uint64(seqMap["Height"].(float64))

		currentTime := time.Now()
		node.confirmTimes.Add(hash, currentTime)

		broadcastTime := node.server.InsertOrIgnoreConfirmTime(hash, currentTime)
		node.broadcastInfo.Add(hash, broadcastTime)

		node.info.latestBroadcastTime = broadcastTime
		node.info.avgBroadcastTime = node.broadcastInfo.AvgTime()
		node.info.latestBlockHeight = height
		node.info.latestBlockHash = hash
		// 区块时间，当前使用接收时间，应该使用区块时间戳
		node.info.latestBlockTime = currentTime
		break
	// 是否属于出块委员会
	case SDKeyIsProducer:
		isProducer := msgObj.Value.(bool)
		if node.info.isProducer == isProducer {
			return
		}
		node.info.isProducer = isProducer
		break
	// 待处理交易
	case SDKeyTxPoolNum:
		txPoolNum := int(msgObj.Value.(float64))
		if node.info.txPoolNum == txPoolNum {
			return
		}
		node.info.txPoolNum = txPoolNum
		break
	default:
		logrus.Errorf("Unknown message key: %s", msgObj.Key)
	}

	if node.info.allowedToSend() {
		//node.server.sendKafkaMsg(node.info.toKafkaMsg())
		node.sendMu.Lock()
		node.sendFlag = true
		node.sendMu.Unlock()
	}
}

func (node *Node) sendCheck() {
	timer := time.NewTicker(time.Millisecond * 300)
	for {
		select {
		case <-timer.C:
			doit := false
			node.sendMu.Lock()
			if node.sendFlag {
				node.sendFlag = false
				doit = true
			}
			node.sendMu.Unlock()
			if doit {
				node.server.sendKafkaMsg(node.info.toKafkaMsg())
			}
		}
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
	if b.times.Len() == 0 {
		return 0
	}
	return b.total / time.Duration(b.times.Len())
}

// NodeInfo 节点信息类型
// 键节点地址对应的值
type NodeInfo struct {
	address             string
	nodeName            string    /* 节点名 */
	nodeLatitude        float64   // 经度
	nodeLongitude       float64   // 纬度
	nodeLocation        string    // 地理位置
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

	location, err := GetLocation(address)
	if err == nil {
		ni.nodeLatitude = location.Latitude
		ni.nodeLongitude = location.Longitude
		ni.nodeLocation = location.Location
	}

	return ni
}

func (ni *NodeInfo) allowedToSend() bool {
	return ni.nodeName != "" && ni.latestBlockHeight != 0
}

func (ni *NodeInfo) toKafkaMsg() []byte {
	nodeKafka := NodeKafka{
		NodeName:            ni.nodeName,
		NodeIP:              ni.address,
		NodeLatitude:        fmt.Sprintf("%f", ni.nodeLatitude),
		NodeLongitude:       fmt.Sprintf("%f", ni.nodeLongitude),
		NodeLocation:        ni.nodeLocation,
		Version:             ni.version,
		ConnNum:             ni.connNum,
		LatestBlock:         fmt.Sprintf("%d", ni.latestBlockHeight),
		LatestBlockHash:     ni.latestBlockHash,
		LatestBlockTime:     int(ni.latestBlockTime.UnixNano() / 1e6),
		BroadcastTime:       int(ni.latestBroadcastTime.Nanoseconds() / 1e6),
		AvgBroadcastTime:    int(ni.avgBroadcastTime.Nanoseconds() / 1e6),
		IsProducer:          ni.isProducer,
		PendingTransactions: ni.txPoolNum,
	}

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
	NodeName      string `json:"node_name"`      /* 节点名 */
	NodeIP        string `json:"node_ip"`        /* IP地址 */
	NodeLatitude  string `json:"node_latitude"`  // 经度
	NodeLongitude string `json:"node_longitude"` // 纬度
	NodeLocation  string `json:"node_location"`  // 地理位置
	Version       string `json:"version"`        /* 运行版本 */
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
