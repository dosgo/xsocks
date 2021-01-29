package comm

import "encoding/binary"
import 	"math/rand"

type VideoChat struct {
	sn uint32
}

func (vc *VideoChat) Size() int32 {
	return 13
}

// Serialize implements PacketHeader.
func (vc *VideoChat) Serialize(b []byte) {
	vc.sn++
	b[0] = 0xa1
	b[1] = 0x08
	binary.BigEndian.PutUint32(b[2:], vc.sn) // b[2:6]
	b[6] = 0x00
	b[7] = 0x10
	b[8] = 0x11
	b[9] = 0x18
	b[10] = 0x30
	b[11] = 0x22
	b[12] = 0x30
}

func RollUint16() uint32 {
	return uint32( uint16(rand.Intn(65536)))
}


func NewVideoChat() *VideoChat {
	return &VideoChat{
		sn: uint32(RollUint16()),
	}
}
