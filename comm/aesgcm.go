package comm

import (
	"crypto/aes"
	"crypto/cipher"
	"time"
	"fmt"
	"xSocks/param"
)

/*aesGcm*/
func AesGcm(buf []byte,encode bool)  ([]byte,error){
	//key
	block, err := aes.NewCipher([]byte(param.Password))
	if err != nil {
		return nil,err;
	}
	//gen nonce
	timeStr:=fmt.Sprintf("%d",time.Now().Unix())
	nonce:=timeStr[:len(timeStr)-2]
	aesgcm, err := cipher.NewGCM(block)
	if(encode) {
		return aesgcm.Seal(nil, []byte(nonce), buf, nil), nil;
	}else{
		return aesgcm.Open(nil, []byte(nonce), buf, nil)
	}
}

