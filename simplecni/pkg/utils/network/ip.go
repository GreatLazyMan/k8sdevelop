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

func Contains(ip *net.IP, ipn *net.IPNet) bool {
	ipv4 := ip.To4()
	return ((ipv4[0] & ipn.Mask[0]) == (ipn.IP[0] & ipn.Mask[0])) &&
		((ipv4[1] & ipn.Mask[1]) == (ipn.IP[1] & ipn.Mask[1])) &&
		((ipv4[2] & ipn.Mask[2]) == (ipn.IP[2] & ipn.Mask[2])) &&
		((ipv4[3] & ipn.Mask[3]) == (ipn.IP[3] & ipn.Mask[3]))
}
