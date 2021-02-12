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
	"xSocks/comm/udpHeader"
	"xSocks/param"
)


var addrTun sync.Map

func StartSudp(_addr string) error {
	addr, err := net.ResolveUDPAddr("udp", _addr)
	if err != nil {
		log.Println(err)
		return err;
	}
	_conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err;
	}
	conn:=udpHeader.NewUdpConn(_conn);
	defer conn.Close()
	data := make([]byte,65535)
	var buffer bytes.Buffer
	var aesGcm=comm.NewAesGcm();
	if(aesGcm==nil){
		fmt.Println("aesGcm init error")
	}
	for {
		n, rAddr, err := conn.ReadFrom(data)
		if err != nil {
			log.Println(err)
			continue
		}
		sudpRecv(data[:n],rAddr,conn,buffer,aesGcm);
	}
}


func sudpRecv(buf []byte,addr net.Addr,conn *udpHeader.UdpConn,buffer bytes.Buffer,aesGcm *comm.AesGcm){
	ciphertext,err:=aesGcm.AesGcm(buf,false);
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
		go tunRecv(tunConn,addr,conn);
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

func tunRecv(tunConn net.Conn,addr net.Addr,udpComm *udpHeader.UdpConn){
	var bufByte []byte = make([]byte,65535)
	var packLenByte []byte = make([]byte, 2)
	var aesGcm=comm.NewAesGcm();
	defer  addrTun.Delete(addr.String())
	if(aesGcm==nil){
		fmt.Println("aesGcm init err")
		return
	}
	for {
		tunConn.SetReadDeadline(time.Now().Add(60*time.Second))
		_, err := io.ReadFull(tunConn, packLenByte)
		packLen := binary.LittleEndian.Uint16(packLenByte)
		if (err != nil||int(packLen)>len(bufByte)) {
			break;
		}
		tunConn.SetReadDeadline(time.Now().Add(60*time.Second))
		n, err := io.ReadFull(tunConn, bufByte[:int(packLen)])
		if (err != nil) {
			fmt.Printf("recv pack err :%v\r\n", err)
			break;
		}else {
			 ciphertext,err:=aesGcm.AesGcm(bufByte[:n],true);
			 if(err==nil) {
				 udpComm.WriteTo(ciphertext, addr)
			 }else{
			 	log.Printf("err:%v\r\n",err);
			 }
		}
	}
}




