# xSocks


一个基于quic/websocket+tls的透明代理工具，支持智能DNS(windows 不支持)，本软件只用于学习网络原理使用编程学习练手，请勿用于非法用途，后果自负。


#优点
 1.部署超级简单
 2.纯go语言编写交叉编译简单，移植简单
 3.维护简单要换传输协议，只要在server client目录各加一个实现文件，然后tunnel增加一个类型即可。



#编译方法
1.git clone https://github.com/dosgo/xSocks.git

2.cd cli

3.go build server.go  //编译服务器端

4.go build client.go  //编译客户端


#使用方法
一.自签名证书使用

  1.运行server -webPort 443 -quicPort 5002  -password  123456  //运行启动后会在当前目录生成你xx_server.key/xx_server.perm/xx_ca.perm/xx_ca.key; 4个文件。
  
  2.使用quic协议 client  -sock5Addr "127.0.0.1:6000"  -serverAddr "quic://你的vps公网Ip:5002" -password 123456   //启动好后使用SwitchyOmega填写127.0.0.1:6000地址就可以了
  
  3.使用websocket+tls client -caFile "第一步生成的xx_ca.perm" -sock5Addr "127.0.0.1:6000"  -serverAddr "wss://你的vps公网Ip" -password 123456  //启动好后使用SwitchyOmega填写127.0.0.1:6000地址就可以了
  
 
二.收费tls证书使用
   
  1.运行server -webPort 443 -quicPort 5002  -password  123456 -certFile "购买的收费证书路径"  -keyFile "私钥路径" 
  
  2.使用quic协议 client -sock5Addr "127.0.0.1:6000"  -serverAddr "quic://你的vps公网Ip:5002" -password 123456   //启动好后使用SwitchyOmega填写127.0.0.1:6000地址就可以了
  
  3.使用websocket+tls client   -sock5Addr "127.0.0.1:6000"  -serverAddr "wss://你证书签名的域名" -password 123456  //启动好后使用SwitchyOmega填写127.0.0.1:6000地址就可以了
  

#感谢
  github.com/google/netstack
  github.com/yinghuocho/gotun2socks
  github.com/miekg/dns
  github.com/xtaci/smux
  github.com/hashicorp/yamux
