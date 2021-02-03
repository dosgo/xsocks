package server

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
	"xSocks/comm"
	"xSocks/param"
)


var addrTun sync.Map
var addrLastTime sync.Map

func StartSudp(_addr string) error {
	addr, err := net.ResolveUDPAddr("udp", _addr)
	if err != nil {
		log.Println(err)
		return err;
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err;
	}
	defer conn.Close()
	data := make([]byte,65535)
	var buffer bytes.Buffer
	var aesGcm=comm.NewAesGcm();
	if(aesGcm==nil){
		fmt.Println("aesGcm init error")
	}
	go autoFree();
	for {
		n, rAddr, err := conn.ReadFromUDP(data)
		if err != nil {
			log.Println(err)
			continue
		}
		sudpRecv(data[:n],rAddr,conn,buffer,aesGcm);
	}
}

func autoFree(){
	for{
		addrLastTime.Range(func(_k, _v interface{}) bool {
			lastTime:=_v.(int64)
			k:=_k.(string)
			if(lastTime+600<time.Now().Unix()){
				_v2,ok:=addrTun.Load(k)
				if ok{
					fmt.Println("auto Free close")
					tunConn:=_v2.(net.Conn)
					tunConn.Close();
					addrTun.Delete(k)
				}
				addrLastTime.Delete(_k)
			}
			return true
		})
		time.Sleep(time.Second*60);
	}
}


func sudpRecv(buf []byte,addr *net.UDPAddr,conn *net.UDPConn,buffer bytes.Buffer,aesGcm *comm.AesGcm){
	//
	addrLastTime.Store(addr.String(),time.Now().Unix());
	videoHeader:=comm.NewVideoChat();
	ciphertext,err:=aesGcm.AesGcm(buf[videoHeader.Size():],false);
	if (err!=nil){
		timeStr:=fmt.Sprintf("%d",time.Now().Unix())
		nonce:=timeStr[:len(timeStr)-2]
		fmt.Println("Decryption failed nonce:",nonce,err)
		return
	}
	//read Mtu
	mtu := binary.LittleEndian.Uint16(ciphertext[:2])
	if(mtu<1){
		mtu=1024;
	}
	var tunConn net.Conn
	v,ok := addrTun.Load(addr.String())
	if !ok{
		tunConn, err = net.DialTimeout("tcp", "127.0.0.1:"+param.TunPort, param.ConnectTime)
		if (err != nil) {
			log.Printf("err:%v\r\n", param.TunPort)
			return;
		}
		tunConn.Write(ciphertext[:2])
		addrTun.Store(addr.String(),tunConn)
		go tunRecv(tunConn,addr,conn,videoHeader);
	}else{
		tunConn=v.(net.Conn)
	}
	var packLenByte []byte = make([]byte, 2)
	binary.LittleEndian.PutUint16(packLenByte, uint16(len(ciphertext)-2))
	buffer.Reset()
	buffer.Write(packLenByte)
	buffer.Write(ciphertext[2:])
	tunConn.Write(buffer.Bytes());
}

func tunRecv(tunConn net.Conn,addr *net.UDPAddr,udpComm *net.UDPConn,videoHeader *comm.VideoChat){
	var bufByte []byte = make([]byte,65535)
	var packLenByte []byte = make([]byte, 2)
	var header []byte = make([]byte, videoHeader.Size())
	var buffer bytes.Buffer
	var aesGcm=comm.NewAesGcm();
	defer  addrTun.Delete(addr.String())
	if(aesGcm==nil){
		fmt.Println("aesGcm init err")
		return
	}
	for {
		_, err := io.ReadFull(tunConn, packLenByte)
		packLen := binary.LittleEndian.Uint16(packLenByte)
		if (err != nil||int(packLen)>len(bufByte)) {
			break;
		}
		n, err := io.ReadFull(tunConn, bufByte[:int(packLen)])
		if (err != nil) {
			fmt.Printf("recv pack err :%v\r\n", err)
			break;
		}else {
			 videoHeader.Serialize(header)
			 buffer.Reset()
			 buffer.Write(header)
			 ciphertext,_:=aesGcm.AesGcm(bufByte[:n],true);
			 buffer.Write(ciphertext)
			 udpComm.WriteTo(buffer.Bytes(), addr)
		}
	}
}




