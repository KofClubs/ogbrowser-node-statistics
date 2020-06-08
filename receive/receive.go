package main

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
)

const (
	Host   = "127.0.0.1"
	Port   = 2020
	MsgLen = 64
)

type Message struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

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
	readChan := make(chan []byte)
	endChan := make(chan bool)
	go readMsg(conn, readChan, endChan)
	for {
		select {
		case msg := <-readChan:
			var obj Message
			err := json.Unmarshal(msg, &obj)
			if err != nil {
				fmt.Println(err)
				return
			}
			// TODO 对obj的相关处理
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
		fmt.Println("Received:", msg)
		readChan <- msg
	}
	endChan <- true
}
