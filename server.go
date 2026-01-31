package main

import (
	"fmt"
	"log"
	"net"
	"sync"
)

var ch1 = make(chan string, 100)
var conns []net.Conn
var mu sync.Mutex  // 读写锁
var mu1 sync.Mutex // 读写锁
var id = make(map[net.Conn]int)
var ids = 1

func removeConnection(conn net.Conn) { // 删除连接
	mu.Lock()
	defer mu.Unlock()
	for i, c := range conns {
		if c == conn {
			// 删除第i个元素
			conns = append(conns[:i], conns[i+1:]...)
			return
		}
	}
}

func process(conn net.Conn) {
	defer conn.Close()
	defer removeConnection(conn)
	buf := make([]byte, 1024)
	for {
		fmt.Println("服务器在等待客户端发送数据...\n" + conn.RemoteAddr().String())
		//conn.Read,阻塞等待客户端发送数据,读取到数据后,放到buf切片中
		if n, err := conn.Read(buf); err != nil {
			log.Println("read error:", err)
			return
		} else {
			mu1.Lock()
			clientID := id[conn]
			mu1.Unlock()
			message := fmt.Sprintf("[%d]:%s", clientID, string(buf[:n]))
			ch1 <- message
		}
	}
}

func main() {
	fmt.Println("服务器启动...")
	fmt.Println("固定端口:8000")
	listener, err := net.Listen("tcp", ":8000") //监听
	if err != nil {
		log.Fatal(err)
	}
	go func() { //消息分发
		for {
			ciphertext := <-ch1
			mu.Lock()
			for _, conn := range conns {
				_, _ = conn.Write([]byte(ciphertext))
			}
			mu.Unlock()
		}
	}()
	for {
		conn, err := listener.Accept() //阻塞等待连接
		if err != nil {
			log.Print(err) // e.g., connection aborted
			//continue
		}
		go process(conn)
		mu.Lock()
		mu1.Lock()
		id[conn] = ids
		ids++
		mu1.Unlock()
		conns = append(conns, conn)
		mu.Unlock()
	}
}
