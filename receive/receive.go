package receive

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"

	"github.com/latifrons/soccerdash"
)

const (
	Host   = "127.0.0.1"
	Port   = 2020
	MsgLen = 64
)

func Receive() {
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
			return
		}
		go task(conn)
	}
}

func task(conn net.Conn) {
	defer conn.Close()
	for {
		msg := make([]byte, MsgLen)
		_, err := conn.Read(msg)
		if err != nil {
			fmt.Println(err)
			return
		}
		var obj soccerdash.Message
		err := json.Unmarshal(msg, &obj)
		if err != nil {
			fmt.Println(err)
			return
		}
		// Do sth with obj
	}
}
