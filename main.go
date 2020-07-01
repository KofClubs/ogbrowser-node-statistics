package main

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"net"
	"ogbrowser-node-statistics/server"
	"time"
)

var nodeMap map[net.Addr]server.NodeInfo /* 键：节点地址，值：节点信息 */
var blockConfirmTime map[string]int      /* 键：区块哈希，值：最早收到推送时间 */

func main() {
	viper.AutomaticEnv()
	conf := server.Config{
		Host:       "0.0.0.0",
		Port:       8080,
		LogDir:     "log",
		LogLevel:   "trace",
		KafkaAddr:  viper.GetString("kafka.broker"),
		KafkaTopic: viper.GetString("kafka.topic"),
	}
	fmt.Printf("%+v\n", conf)

	server.InitLogger(conf.LogDir, conf.LogLevel, true)

	localServer := server.NewServer(conf)
	err := localServer.Start()
	if err != nil {
		logrus.Error(err)
		return
	}
	defer localServer.Stop()

	// do not let the program ends
	for {
		time.Sleep(time.Hour)
	}
}
