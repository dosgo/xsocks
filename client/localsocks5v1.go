package client

import (
	"bufio"
	"context"
	"encoding/binary"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/things-go/go-socks5"
)

type LocalSocksV1 struct {
	l        net.Listener
	udpProxy *UdpProxy
}

// 域名白名单
var specialDomains = map[string]bool{
	"example.com":     true,
	"specialsite.org": true,
}

// 解析SOCKS5请求
func parseSocks5Request(conn net.Conn) (string, error) {
	reader := bufio.NewReader(conn)
	// 读取SOCKS5版本和命令
	ver, err := reader.ReadByte()
	if err != nil {
		return "", err
	}
	cmd, err := reader.ReadByte()
	if err != nil {
		return "", err
	}

	// 确保是SOCKS5版本和CONNECT命令
	if ver != 5 || (cmd != 1 && cmd != 3) {
		return "", net.ErrClosed
	}

	// 读取目标地址
	targetAddr, err := readTargetAddress(reader)
	if err != nil {
		return "", err
	}

	return targetAddr, nil
}

// 读取目标地址
func readTargetAddress(reader *bufio.Reader) (string, error) {
	// 读取地址类型
	addrType, err := reader.ReadByte()
	if err != nil {
		return "", err
	}

	var targetAddr string
	switch addrType {
	case 1: // IPv4
		ipBytes := make([]byte, 4)
		_, err := reader.Read(ipBytes)
		if err != nil {
			return "", err
		}
		targetAddr = net.IP(ipBytes).String()
	case 3: // 域名
		domainLen, err := reader.ReadByte()
		if err != nil {
			return "", err
		}
		domainBytes := make([]byte, domainLen)
		_, err = reader.Read(domainBytes)
		if err != nil {
			return "", err
		}
		targetAddr = string(domainBytes)
	case 4: // IPv6
		ipBytes := make([]byte, 16)
		_, err := reader.Read(ipBytes)
		if err != nil {
			return "", err
		}
		targetAddr = net.IP(ipBytes).String()
	default:
		return "", net.ErrClosed
	}

	// 读取端口号
	portBytes := make([]byte, 2)
	_, err = reader.Read(portBytes)
	if err != nil {
		return "", err
	}
	port := int(binary.BigEndian.Uint16(portBytes))

	return targetAddr + ":" + strconv.Itoa(port), nil
}

// 主函数
func (lSocks *LocalSocksV1) Start(address string) error {
	// 监听SOCKS5端口
	listener, err := net.Listen("tcp", address)
	if err != nil {
		panic(err)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		go func(conn net.Conn) {
			// 解析SOCKS5请求
			targetAddr, err := parseSocks5Request(conn)
			if err != nil {
				conn.Close()
				return
			}

			// 检查域名是否在白名单中
			domain := strings.Split(targetAddr, ":")[0]
			if specialDomains[domain] {
				// 执行特殊处理
				specialHandle(conn, targetAddr)
			} else {
				// 默认处理
				defaultHandle(conn, targetAddr)
			}
		}(conn)
	}
}

// 特殊处理
func specialHandle(conn net.Conn, targetAddr string) {
	// 这里可以实现针对特定域名的特殊逻辑
	// 例如，你可以连接到一个特殊的代理，或者执行某种数据修改
	// 然后转发请求到目标服务器
}

// 默认处理
func defaultHandle(conn net.Conn, targetAddr string) {
	// 连接到目标服务器
	target, err := net.Dial("tcp", targetAddr)
	if err != nil {
		conn.Close()
		return
	}
	defer target.Close()
	// 设置双向数据传输
	go io.Copy(target, conn)
	go io.Copy(conn, target)
}

// 主函数
func main() {
	// 监听SOCKS5端口
	listener, err := net.Listen("tcp", ":1080")
	if err != nil {
		panic(err)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		go handleConnection(conn)
	}
}

// 处理连接
func handleConnection(conn net.Conn) {
	reader := bufio.NewReader(conn)

	// 解析SOCKS5请求
	ver, err := reader.ReadByte()
	if err != nil {
		conn.Close()
		return
	}

	if ver != 5 {
		conn.Close()
		return
	}

	// 读取命令
	cmd, err := reader.ReadByte()
	if err != nil {
		conn.Close()
		return
	}

	if cmd == 3 { // UDP Associate
		handleUdpAssociate(conn, reader)
	} else {
		// 处理TCP请求
		// ...
	}
}

// 处理UDP Associate请求
func handleUdpAssociate(conn net.Conn, reader *bufio.Reader) {
	// UDP Associate请求的其余部分
	// ...

	// 分配一个本地UDP端口
	udpListener, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		conn.Close()
		return
	}
	defer udpListener.Close()

	// 发送UDP Associate响应
	resp := make([]byte, 8)
	resp[0] = 5 // SOCKS5版本
	resp[1] = 0 // 成功
	resp[2] = 0 // 保留
	resp[3] = 1 // IPv4
	copy(resp[4:], udpListener.LocalAddr().(*net.UDPAddr).IP.To4())
	binary.BigEndian.PutUint16(resp[8:], uint16(udpListener.LocalAddr().(*net.UDPAddr).Port))

	_, err = conn.Write(resp)
	if err != nil {
		conn.Close()
		return
	}

	// UDP数据包处理循环
	go func() {
		for {
			buf := make([]byte, 512)
			n, _, err := udpListener.ReadFromUDP(buf)
			if err != nil {
				return
			}

			// 发送到SOCKS5连接
			_, err = conn.Write(buf[:n])
			if err != nil {
				return
			}
		}
	}()

	go func() {
		for {
			buf := make([]byte, 512)
			n, err := conn.Read(buf)
			if err != nil {
				return
			}

			// 检查域名并进行特殊处理
			domain := checkDomain(buf)
			if specialDomains[domain] {
				// 特殊处理
				// ...
			} else {
				// 默认处理
				_, err = udpListener.WriteToUDP(buf[:n], &net.UDPAddr{IP: net.IPv4(0, 0, 0, 0), Port: 0}) // 修正目标地址
				if err != nil {
					return
				}
			}
		}
	}()
}

// 检查域名
func checkDomain(buf []byte) string {
	// 解析域名逻辑
	// ...
	return ""
}

// 自定义的SOCKS5处理器
// 自定义的SOCKS5处理器
type MyHandler struct{}

/*https://github.com/things-go/go-socks5*/
// 实现Handler接口中的HandleConnect方法
func (h *MyHandler) handleConnection(ctx context.Context, writer io.Writer, request *socks5.Request) error {
	// 从context中获取目标地址
	//targetAddr := ctx.TargetAddr
	targetAddr := request.RemoteAddr
	// 检查目标地址是否在白名单中
	if h.isSpecialDomain(targetAddr.String()) {
		// 进行特殊处理，例如连接到不同的服务器
		// 或者应用额外的安全检查
		log.Println("Handling special domain:", targetAddr)
		// 这里可以实现你自己的逻辑
		// ...
	} else {
		// 进行常规处理，例如直接连接到目标服务器
		log.Println("Handling regular domain:", targetAddr)
		// 这里可以实现你自己的逻辑
		// ...
	}

	// 建立到目标服务器的连接
	targetConn, err := net.Dial("tcp", targetAddr.String())
	if err != nil {
		log.Println("Failed to connect to target:", err)
		return err
	}
	defer targetConn.Close()

	// 开始转发数据
	go func() {
		_, err := io.Copy(targetConn, writer)
		if err != nil {
			log.Println("Failed to copy data:", err)
		}
	}()
	go func() {
		_, err := io.Copy(ctx.Conn, targetConn)
		if err != nil {
			log.Println("Failed to copy data:", err)
		}
	}()

	return nil
}

// 检查目标地址是否在特殊域名单中
func (h *MyHandler) isSpecialDomain(addr string) bool {
	// 这里可以实现你自己的逻辑来检查地址
	// ...
	return false
}

func main1() {
	var myHandler MyHandler
	// 创建自定义的SOCKS5处理器实例
	// Create a SOCKS5 server
	server := socks5.NewServer(
		socks5.WithLogger(socks5.NewLogger(log.New(os.Stdout, "socks5: ", log.LstdFlags))),
		socks5.WithConnectHandle(myHandler.handleConnection),
	)

	// Create SOCKS5 proxy on localhost port 8000
	if err := server.ListenAndServe("tcp", ":10800"); err != nil {
		panic(err)
	}
}

// 实现Handler接口中的HandleUDPAssociate方法
func (h *MyHandler) handleAssociate(ctx context.Context, conn conn, req *Request) error {
	// 创建一个本地的UDP监听器
	udpListener, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return nil, err
	}
	defer udpListener.Close()

	// UDP监听器的本地地址
	localAddr := udpListener.LocalAddr().(*net.UDPAddr)

	// UDP数据包处理循环
	go func() {
		for {
			buf := make([]byte, 512)
			n, remoteAddr, err := udpListener.ReadFromUDP(buf)
			if err != nil {
				log.Println("Error reading from UDP listener:", err)
				break
			}

			// 这里可以实现对UDP数据包的特殊处理
			// 例如，检查remoteAddr，如果是特殊域名则进行特殊处理

			// 将UDP数据包转发到正确的目的地
			// 注意，这里需要你根据实际情况填充目的地的地址和端口
			destAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}
			_, err = udpListener.WriteToUDP(buf[:n], destAddr)
			if err != nil {
				log.Println("Error forwarding UDP packet:", err)
				break
			}
		}
	}()

	return localAddr, nil
}
