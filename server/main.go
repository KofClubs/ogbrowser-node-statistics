package main

import (
	"github.com/BurntSushi/toml"
	"github.com/sirupsen/logrus"
	"time"
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

//var nodeMap map[net.Addr]NodeInfo /* 键：节点地址，值：节点信息 */
//var blockConfirmTime map[string]int  /* 键：区块哈希，值：最早收到推送时间 */

func main() {
	var conf Config
	_, err := toml.DecodeFile("../config.toml", &conf)
	if err != nil {
		panic(err)
	}

	initLogger(conf.LogDir, conf.LogLevel, true)

	server := NewServer(conf)
	err = server.Start()
	if err != nil {
		logrus.Error(err)
		return
	}
	defer server.Stop()

	// do not let the program ends
	for {
		time.Sleep(time.Hour)
	}
}
