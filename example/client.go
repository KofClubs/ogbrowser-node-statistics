package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
)

func main() {
	for {
		conn, err := net.Dial("tcp", "127.0.0.1:2020")
		if err != nil {
			fmt.Println(err)
			continue
		} else {
			defer conn.Close()
			input := bufio.NewScanner(os.Stdin)
			for input.Scan() {
				line := input.Text()
				lineLen := len(line)
				n := 0
				for flag := 0; flag < lineLen; flag += n {
					var msg string
					if lineLen-flag > 64 {
						msg = line[flag : flag+64]
					} else {
						msg = line[flag:]
					}
					n, err = conn.Write([]byte(msg))
					if err != nil {
						fmt.Println(err)
					}
				}
			}
		}
	}
}
