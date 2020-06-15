package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strconv"

	"github.com/latifrons/soccerdash"
)

// TCP通信全局变量
const (
	Host    = "127.0.0.1" /* 本地IP地址 */
	Port    = 2020        /* 本地端口 */
	MsgLen  = 4096        /* 消息长度上界 */
	ChanCap = 7           /* 消息读入管道容量 */
)

func main() {
	address := Host + ":" + strconv.Itoa(Port)
	ln, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Println(err)
		return
	}
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
			fmt.Println(msgObj.Key)
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
