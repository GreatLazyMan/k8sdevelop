package network

import (
	"math/bits"
	"net"
)

type IP4 uint32

var Localhost *net.IPNet

func init() {
	_, Localhost, _ = net.ParseCIDR("127.0.0.1/32")
}

func CountMaskLen(data net.IPMask) int {
	pos := 0
	for i := 3; i > 0; i-- {
		pos += bits.TrailingZeros8(uint8(data[i]))
	}

	return 32 - pos
}
