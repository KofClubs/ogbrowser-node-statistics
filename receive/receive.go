package receive

import (
	"config"
	"fmt"
	"net"
	"strconv"

	"github.com/latifrons/soccerdash"
)

func Receive() {
	address := config.Host + ":" + strconv.Itoa(config.Port)
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
		msg := make([]byte, 64)
		_, err := conn.Read(msg)
		if err != nil {
			fmt.Println(err)
			return
		}
		// Do sth. with msg
	}
}

func deserialize(str string) soccerdash.Message {
	// Do sth.
}
