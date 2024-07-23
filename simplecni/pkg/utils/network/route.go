//go:build !windows
// +build !windows

// Copyright 2017 flannel authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package network

import (
	"bytes"

	"github.com/vishvananda/netlink"
	log "k8s.io/klog/v2"
)

const (
	routeCheckRetries = 10
)

func routeEqual(x, y netlink.Route) bool {
	// For ipip backend, when enabling directrouting, link index of some routes may change
	// For both ipip and host-gw backend, link index may also change if updating ExtIface
	if x.Dst.IP.Equal(y.Dst.IP) && x.Gw.Equal(y.Gw) && bytes.Equal(x.Dst.Mask, y.Dst.Mask) && x.LinkIndex == y.LinkIndex {
		return true
	}
	return false
}

func routeAdd(route *netlink.Route, ipFamily int, addToRouteList, removeFromRouteList func(netlink.Route)) {
	addToRouteList(*route)
	// Check if route exists before attempting to add it
	routeList, err := netlink.RouteListFiltered(ipFamily, &netlink.Route{Dst: route.Dst}, netlink.RT_FILTER_DST)
	if err != nil {
		log.Warningf("Unable to list routes: %v", err)
	}

	if len(routeList) > 0 && !routeEqual(routeList[0], *route) {
		// Same Dst different Gw or different link index. Remove it, correct route will be added below.
		log.Warningf("Replacing existing route to %v with %v", routeList[0], route)
		if err := netlink.RouteDel(&routeList[0]); err != nil {
			log.Errorf("Effor deleteing route to %v: %v", routeList[0].Dst, err)
			return
		}
		removeFromRouteList(routeList[0])
	}
	routeList, err = netlink.RouteListFiltered(ipFamily, &netlink.Route{Dst: route.Dst}, netlink.RT_FILTER_DST)
	if err != nil {
		log.Warningf("Unable to list routes: %v", err)
	}

	if len(routeList) > 0 && routeEqual(routeList[0], *route) {
		// Same Dst and same Gw, keep it and do not attempt to add it.
		log.Infof("Route to %v already exists, skipping.", route)
	} else if err := netlink.RouteAdd(route); err != nil {
		log.Errorf("Error adding route to %v: %s", route, err)
		return
	}
	_, err = netlink.RouteListFiltered(ipFamily, &netlink.Route{Dst: route.Dst}, netlink.RT_FILTER_DST)
	if err != nil {
		log.Warningf("Unable to list routes: %v", err)
	}
}
