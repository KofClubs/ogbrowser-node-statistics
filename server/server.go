package server

import (
	"fmt"
	"github.com/Shopify/sarama"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Config 服务器信息
// 从config.toml读
type Config struct {
	Host       string
	Port       int
	LogDir     string
	LogLevel   string
	KafkaAddr  string
	KafkaTopic string
}

type Server struct {
	conf         Config
	localAddress string

	blockCh          chan int
	blockConfirmTime *OrderedMap // stores earliest confirm time of each Sequencer
	nodes            map[string]*Node

	produceCh chan []byte
	producer  sarama.AsyncProducer
	ln        net.Listener

	closeCh chan struct{}
}

func NewServer(conf Config) *Server {
	s := &Server{}

	s.conf = conf
	s.localAddress = ":" + strconv.Itoa(conf.Port)

	s.blockCh = make(chan int)
	s.blockConfirmTime = NewOrderedMap(1000)
	s.nodes = make(map[string]*Node)

	s.closeCh = make(chan struct{})
	return s
}

func (s *Server) Start() error {
	logrus.Info("Server started")

	// server start listening
	ln, err := net.Listen("tcp", s.localAddress)
	if err != nil {
		return err
	}
	s.ln = ln
	go s.listen()

	// init a kafka producer
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Partitioner = sarama.NewRandomPartitioner
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true
	config.Version = sarama.V0_11_0_2
	producer, err := sarama.NewAsyncProducer([]string{s.conf.KafkaAddr}, config)
	if err != nil {
		return err
	}

	s.produceCh = make(chan []byte)
	s.producer = producer

	go s.producing()
	return nil
}

func (s *Server) Stop() {
	logrus.Info("Server stopped")
	close(s.closeCh)

	for _, node := range s.nodes {
		node.Close()
	}
	s.ln.Close()
	s.producer.AsyncClose()
}

func (s *Server) AddNewNode(node *Node) {
	_, exist := s.nodes[node.address]
	if exist {
		return
	}
	s.nodes[node.address] = node
}

func (s *Server) RemoveNode(node *Node) {
	delete(s.nodes, node.address)
}

func (s *Server) listen() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			logrus.Error(err)
			continue
		}
		logrus.Infof("new connection come: %v", conn.RemoteAddr())

		node := NewNode(conn, s)
		s.AddNewNode(node)
	}
}

func (s *Server) sendKafkaMsg(msg []byte) {
	s.produceCh <- msg
}

// producing force the kafka sending synchronized
func (s *Server) producing() {
	for {
		select {
		case <-s.closeCh:
			logrus.Info("producing closed")
			return
		case msg := <-s.produceCh:
			s.produceKafkaMsg(msg)
		}
	}
}

func (s *Server) produceKafkaMsg(content []byte) error {
	msg := &sarama.ProducerMessage{
		Topic: s.conf.KafkaTopic,
		Value: sarama.ByteEncoder(content),
	}
	s.producer.Input() <- msg
	select {
	case msg = <-s.producer.Successes():
		logrus.Tracef("offset: %d, time: %s, partitions: %d", msg.Offset, msg.Timestamp.String(), msg.Partition)
		return nil
	case fail := <-s.producer.Errors():
		logrus.Error(fail.Err)
		return fail.Err
	}
	return nil
}

// InsertOrIgnoreConfirmTime inserts a confirm time and calculate
// the broadcast time between new confirm and earliest confirm.
func (s *Server) InsertOrIgnoreConfirmTime(hash string, currTime time.Time) time.Duration {
	if !s.blockConfirmTime.Exists(hash) {
		s.blockConfirmTime.Add(hash, currTime)
		return time.Duration(0)
	}

	oldTimeI, _ := s.blockConfirmTime.Get(hash)
	oldTime := oldTimeI.(time.Time)
	if currTime.Before(oldTime) {
		// this should never happen
		logrus.Errorf("earliest confirm time is later than newest confirm time, earliest: %s, newest: %s",
			oldTime.String(), currTime.String())

		s.blockConfirmTime.Update(hash, currTime)
		return time.Duration(0)
	}

	return currTime.Sub(oldTime)
}

/**
OrderedMap
*/

type OrderedMap struct {
	cap  int
	keys []string
	data map[string]interface{}

	mu sync.RWMutex
}

func NewOrderedMap(cap int) *OrderedMap {
	om := &OrderedMap{}

	om.cap = cap
	om.keys = make([]string, 0)
	om.data = make(map[string]interface{})

	return om
}

func (om *OrderedMap) Len() int {
	om.mu.RLock()
	defer om.mu.RUnlock()

	return om.len()
}
func (om *OrderedMap) len() int {
	return len(om.keys)
}

func (om *OrderedMap) Exists(k string) bool {
	om.mu.RLock()
	defer om.mu.RUnlock()

	return om.exists(k)
}
func (om *OrderedMap) exists(k string) bool {
	_, exists := om.data[k]
	return exists
}

func (om *OrderedMap) Add(k string, v interface{}) interface{} {
	om.mu.Lock()
	defer om.mu.Unlock()

	if om.exists(k) {
		return nil
	}
	if om.len() < om.cap {
		om.keys = append(om.keys, k)
		om.data[k] = v
		return nil
	}

	oldestKey := om.keys[0]
	oldestValue := om.data[oldestKey]
	delete(om.data, oldestKey)
	om.keys = append(om.keys[1:], k)
	om.data[k] = v

	return oldestValue
}

func (om *OrderedMap) Update(k string, v interface{}) {
	om.mu.Lock()
	defer om.mu.Unlock()

	if !om.exists(k) {
		return
	}
	om.data[k] = v
}

func (om *OrderedMap) Get(k string) (interface{}, error) {
	om.mu.RLock()
	defer om.mu.RUnlock()

	if !om.exists(k) {
		return nil, fmt.Errorf("the value of key %s, not exists", k)
	}
	return om.data[k], nil
}

//func (om *OrderedMap) CalAvgTimeGap() time.Duration {
//	if om.len() <= 1 {
//		return time.Duration(0)
//	}
//
//	oldest := om.data[om.keys[0]]
//	latest := om.data[om.keys[len(om.keys)-1]]
//
//	avg := int64(latest.Sub(oldest)) / int64(om.len())
//	return time.Duration(avg)
//}
