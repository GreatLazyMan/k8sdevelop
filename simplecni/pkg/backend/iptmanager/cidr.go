package iptmanager

import (
	"net"

	"github.com/GreatLazyMan/simplecni/pkg/constants"
	"github.com/GreatLazyMan/simplecni/pkg/netconfig"
)

type CidrManager struct {
	clustercidr     *net.IPNet
	nodecidr        *net.IPNet
	prevclustercidr *net.IPNet
	prevnodecidr    *net.IPNet
	// not support v6 now
	clustercidrv6     *net.IPNet
	nodecidrv6        *net.IPNet
	prevclustercidrv6 *net.IPNet
	prevnodecidrv6    *net.IPNet
}

func NewCidrManager(ccidr, ncidr, ccidrv6, ncidrv6 *net.IPNet) (*CidrManager, error) {
	cm := new(CidrManager)
	cm.nodecidr = ncidr
	cm.clustercidr = ccidr
	cm.nodecidrv6 = ncidrv6
	cm.clustercidrv6 = ccidrv6
	cidr, err := netconfig.ReadCIDRFromSubnetFile(constants.SUBNET)
	if err != nil {
		return nil, err
	}
	cm.prevnodecidr = cidr

	cidr, err = netconfig.ReadCIDRFromSubnetFile(constants.CIDR)
	if err != nil {
		return nil, err
	}
	cm.prevnodecidr = cidr
	return cm, nil
}
