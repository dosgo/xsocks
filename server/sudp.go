package server

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"log"
	"net"
	"sync"
	"time"
	"github.com/dosgo/xsocks/comm"
	"github.com/dosgo/xsocks/comm/udpHeader"
)


var addrTun sync.Map
type keepInfo struct {
	cancel context.CancelFunc
	lastTime int64;
}


var tunKeep sync.Map

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
	data := make([]byte,1024*10)
	var buffer bytes.Buffer
	var aesGcm=comm.NewAesGcm();
	if aesGcm==nil {
		fmt.Println("aesGcm init error")
	}
	go checkKeep();
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
	if err!=nil {
		timeStr:=fmt.Sprintf("%d",time.Now().Unix())
		nonce:=timeStr[:len(timeStr)-2]
		fmt.Println("Decryption failed nonce:",nonce,err)
		return
	}
	//read Mtu
	mtu := binary.LittleEndian.Uint16(ciphertext[:2])
	if mtu<1 {
		mtu=1024;
	}
	var channelLinkID *channel.Endpoint
	v,ok := addrTun.Load(addr.String())
	if !ok{
		_stack,channelLinkID,err:=StartTunStack(mtu);
		if err!=nil {
			return;
		}
		addrTun.Store(addr.String(),channelLinkID)
		go newTun(_stack,channelLinkID,addr,conn);
	}else{
		channelLinkID=v.(*channel.Endpoint)
	}
	InjectInbound(channelLinkID,ciphertext[2:])
}


func newTun(_stack *stack.Stack, channelLinkID *channel.Endpoint,addr net.Addr,udpComm *udpHeader.UdpConn){
	defer _stack.Close();
	defer addrTun.Delete(addr.String())
	var aesGcm=comm.NewAesGcm();
	var buffer =new(bytes.Buffer)
	defer fmt.Printf("channelLinkID recv exit \r\n");
	ctx, cancel := context.WithCancel(context.Background())
	for {
			pkt,res :=channelLinkID.ReadContext(ctx)
			if !res {
				break;
			}
			tunKeep.Store(addr.String(),keepInfo{cancel:cancel,lastTime: time.Now().Unix()});
			buffer.Reset()
			buffer.Write(pkt.Pkt.NetworkHeader().View())
			buffer.Write(pkt.Pkt.TransportHeader().View())
			buffer.Write(pkt.Pkt.Data.ToView())
			if buffer.Len()>0 {
				ciphertext,err:=aesGcm.AesGcm(buffer.Bytes(),true);
				if err==nil {
					udpComm.WriteTo(ciphertext, addr)
				}else{
					log.Printf("err:%v\r\n",err);
				}
			}
	}
}

func checkKeep(){
	for{
		tunKeep.Range(func(k, v interface{}) bool {
				keepInfo:=v.(keepInfo);
				if keepInfo.lastTime+60*4<time.Now().Unix() {
					keepInfo.cancel();
					tunKeep.Delete(k)
				}
				return true
		});
		time.Sleep(time.Minute*1);
	}
}