package comm

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"fmt"
	"time"
	"github.com/dosgo/xsocks/param"
)

type AesGcm struct {
	Aead cipher.AEAD
}

func NewAesGcm() (*AesGcm){
	//key
	key:=fmt.Sprintf("%x",md5.Sum([]byte(param.Args.Password)))
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil;
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil;
	}
	return  &AesGcm{aesgcm};
}

/*aesGcm*/
func (aesGcm *AesGcm) AesGcm(buf []byte,encode bool)  ([]byte,error){
	//gen nonce
	timeStr:=fmt.Sprintf("%d",time.Now().UTC().Unix())
	nonceMd5:=fmt.Sprintf("%x",md5.Sum([]byte(timeStr[:len(timeStr)-2])))
	if(encode) {
		return aesGcm.Aead.Seal(nil, []byte(nonceMd5[:12]), buf, nil), nil;
	}else{
		return aesGcm.Aead.Open(nil, []byte(nonceMd5[:12]), buf, nil)
	}
}

