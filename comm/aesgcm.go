package comm

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"time"
	"fmt"
	"xSocks/param"
)

/*aesGcm*/
func AesGcm(buf []byte,encode bool)  ([]byte,error){
	//key
	key:=fmt.Sprintf("%x",md5.Sum([]byte(param.Password)))
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil,err;
	}
	//gen nonce
	timeStr:=fmt.Sprintf("%d",time.Now().UTC().Unix())
	nonceMd5:=fmt.Sprintf("%x",md5.Sum([]byte(timeStr[:len(timeStr)-2])))
	aesgcm, err := cipher.NewGCM(block)
	if(encode) {
		return aesgcm.Seal(nil, []byte(nonceMd5[:12]), buf, nil), nil;
	}else{
		return aesgcm.Open(nil, []byte(nonceMd5[:12]), buf, nil)
	}
}

