package main

import (
	"fmt"
	"net"
)

func main() {
	ln, err := net.Listen("tcp", "127.0.0.1:2020")
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
		fmt.Println(string(msg))
	}
}
