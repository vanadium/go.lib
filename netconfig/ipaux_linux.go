// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build linux

package netconfig

// We connect to the Netlink Route socket and parse messages to
// look for network configuration changes.  This is very Linux
// specific, hence the file name.

import (
	"errors"
	"fmt"
	"net"
	"syscall"
	"unsafe"

	"v.io/x/lib/vlog"
)

/*
#include <linux/rtnetlink.h>
*/
import "C"

// All rtnetlink attributes start with this header.
type rtAttrHdr C.struct_rtattr

const rtAttrHdrLen = C.sizeof_struct_rtattr

// The address change messages (RTM_NEWADDR, RTM_DELADDR, RTM_GETADDR).
type ifaddrMsgHdr C.struct_ifaddrmsg

const ifaddrMsgHdrLen = C.sizeof_struct_ifaddrmsg

type rtAttribute fmt.Stringer

type rtAddressMessage struct {
	name       string
	hdr        ifaddrMsgHdr
	attributes []rtAttribute
}

// Attribute types (see rtnetlink(7))
type ifaAddress net.IP
type ifaLocal net.IP
type ifaBroadcast net.IP
type ifaAnycast net.IP
type ifaMulticast net.IP
type ifaLabel string
type ifaCacheInfo C.struct_ifa_cacheinfo

const ifaCacheInfoLen = C.sizeof_struct_ifa_cacheinfo

// String routines to make debugging easier.
func (a ifaAddress) String() string   { return "Address=" + net.IP(a).String() }
func (a ifaLocal) String() string     { return "Local=" + net.IP(a).String() }
func (a ifaBroadcast) String() string { return "Braodcast=" + net.IP(a).String() }
func (a ifaAnycast) String() string   { return "Anycast=" + net.IP(a).String() }
func (a ifaMulticast) String() string { return "Anycast=" + net.IP(a).String() }
func (a ifaLabel) String() string     { return "Label=" + string(a) }
func (a ifaCacheInfo) String() string {
	return fmt.Sprintf("CacheInfo[preferred %d valid %d cstamp %d tstamp %d]", a.ifa_prefered, a.ifa_valid, a.cstamp, a.tstamp)
}
func (m *rtAddressMessage) String() string {
	return fmt.Sprintf("%s: index %d %v", m.name, m.hdr.ifa_index, m.attributes)
}

// Address looks for the address attribute in an rtAddressMessage.  If it isn't there, just assume the null address.
func (m *rtAddressMessage) Address() net.IP {
	for _, a := range m.attributes {
		switch a.(type) {
		case ifaAddress:
			return net.IP(a.(ifaAddress))
		}
	}
	return net.IPv4zero
}

func parsertAddressAttribute(b []byte) (rtAttribute, []byte, error) {
	if len(b) == 0 {
		return nil, nil, nil
	}
	if len(b) < rtAttrHdrLen {
		return nil, nil, errors.New("attribute too short")
	}
	ahdr := (*rtAttrHdr)(unsafe.Pointer(&b[0]))
	rounded := ((ahdr.rta_len + 3) / 4) * 4
	if len(b) < int(rounded) {
		return nil, nil, errors.New("attribute too short")
	}
	remaining := b[rounded:]
	b = b[rtAttrHdrLen:ahdr.rta_len]
	switch ahdr.rta_type {
	case C.IFA_ADDRESS:
		return rtAttribute(ifaAddress(net.IP(b))), remaining, nil
	case C.IFA_LOCAL:
		return rtAttribute(ifaLocal(b)), remaining, nil
	case C.IFA_LABEL:
		return rtAttribute(ifaLabel(b)), remaining, nil
	case C.IFA_BROADCAST:
		return rtAttribute(ifaBroadcast(b)), remaining, nil
	case C.IFA_ANYCAST:
		return rtAttribute(ifaAnycast(b)), remaining, nil
	case C.IFA_CACHEINFO:
		if len(b) < ifaCacheInfoLen {
			return nil, nil, errors.New("attribute too short")
		}
		return rtAttribute(ifaCacheInfo(*(*C.struct_ifa_cacheinfo)(unsafe.Pointer(&b[0])))), remaining, nil
	case C.IFA_MULTICAST:
		return rtAttribute(ifaMulticast(b)), remaining, nil
	}
	return nil, remaining, errors.New("unknown attribute")
}

func parsertAddressMessage(nlm syscall.NetlinkMessage) (*rtAddressMessage, error) {
	var name string
	switch nlm.Header.Type {
	case C.RTM_NEWADDR:
		name = "RTM_NEWADDR"
	case C.RTM_DELADDR:
		name = "RTM_DELADDR"
	case C.RTM_GETADDR:
		name = "RTM_GETADDR"
	default:
		return nil, fmt.Errorf("unknown message type")
	}
	if len(nlm.Data) < ifaddrMsgHdrLen {
		return nil, errors.New("bad length")
	}
	m := &rtAddressMessage{name: name, hdr: *(*ifaddrMsgHdr)(unsafe.Pointer(&nlm.Data[0]))}
	b := nlm.Data[ifaddrMsgHdrLen:]
	for {
		var a rtAttribute
		var err error
		a, b, err = parsertAddressAttribute(b)
		if b == nil {
			break
		}
		if err == nil {
			m.attributes = append(m.attributes, a)
		}
	}
	return m, nil
}

// The link change messages (RTM_NEWLINK, RTM_DELLINK, RTM_GETLINK).
type ifInfoMsgHdr C.struct_ifinfomsg

const ifInfoMsgHdrLen = C.sizeof_struct_ifinfomsg

type rtLinkMessage struct {
	name       string
	hdr        ifInfoMsgHdr
	attributes []rtAttribute
}

// Attribute types (see rtnetlink(7))
type iflaAddress []byte
type iflaBroadcast []byte
type iflaIFName string
type iflaMTU uint32
type iflaLink int
type iflaQDisc string
type iflaOperstate int
type iflaStats C.struct_rtnl_link_stats

const iflaStatsLen = C.sizeof_struct_rtnl_link_stats

// String routines to make debugging easier.
func (a iflaAddress) String() string   { return fmt.Sprintf("HWAddress=%v", []byte(a)) }
func (a iflaBroadcast) String() string { return fmt.Sprintf("HWBroadcast=%v", []byte(a)) }
func (a iflaIFName) String() string    { return "Name=" + string(a) }
func (a iflaMTU) String() string       { return fmt.Sprintf("MTU=%d", uint32(a)) }
func (a iflaLink) String() string      { return fmt.Sprintf("Type=%d", int(a)) }
func (a iflaQDisc) String() string     { return "Qdisc=" + string(a) }
func (a iflaStats) String() string {
	return fmt.Sprintf("Stats[rx %d tx %d ...]", a.rx_packets, a.tx_packets)
}
func (a iflaOperstate) String() string { return fmt.Sprintf("Operstate=%d", int(a)) }
func (m *rtLinkMessage) String() string {
	return fmt.Sprintf("%s: index %d %v", m.name, m.hdr.ifi_index, m.attributes)
}

func parseRTLinkAttribute(b []byte) (rtAttribute, []byte, error) {
	if len(b) == 0 {
		return nil, nil, nil
	}
	if len(b) < rtAttrHdrLen {
		return nil, nil, errors.New("attribute too short")
	}
	ahdr := (*rtAttrHdr)(unsafe.Pointer(&b[0]))
	rounded := ((ahdr.rta_len + 3) / 4) * 4
	if len(b) < int(rounded) {
		return nil, nil, errors.New("attribute too short")
	}
	remaining := b[rounded:]
	b = b[rtAttrHdrLen:ahdr.rta_len]
	switch ahdr.rta_type {
	case C.IFLA_ADDRESS:
		return rtAttribute(iflaAddress(b)), remaining, nil
	case C.IFLA_BROADCAST:
		return rtAttribute(iflaBroadcast(b)), remaining, nil
	case C.IFLA_IFNAME:
		return rtAttribute(iflaIFName(string(b))), remaining, nil
	case C.IFLA_MTU:
		return rtAttribute(iflaMTU(*(*C.uint)(unsafe.Pointer(&b[0])))), remaining, nil
	case C.IFLA_LINK:
		return rtAttribute(iflaMTU(*(*C.int)(unsafe.Pointer(&b[0])))), remaining, nil
	case C.IFLA_QDISC:
		return rtAttribute(iflaQDisc(string(b))), remaining, nil
	case C.IFLA_STATS:
		if len(b) < iflaStatsLen {
			return nil, remaining, errors.New("attribute too short")
		}
		return rtAttribute(iflaStats(*(*C.struct_rtnl_link_stats)(unsafe.Pointer(&b[0])))), remaining, nil
	case C.IFLA_OPERSTATE:
		return rtAttribute(iflaOperstate(*(*C.int)(unsafe.Pointer(&b[0])))), remaining, nil
	}
	return nil, remaining, errors.New("unknown attribute")
}

func parsertLinkMessage(nlm syscall.NetlinkMessage) (*rtLinkMessage, error) {
	var name string
	switch nlm.Header.Type {
	case C.RTM_NEWLINK:
		name = "RTM_NEWLINK"
	case C.RTM_DELLINK:
		name = "RTM_DELLINK"
	case C.RTM_GETLINK:
		name = "RTM_GETLINK"
	default:
		return nil, fmt.Errorf("unknown message type")
	}
	if len(nlm.Data) < ifInfoMsgHdrLen {
		return nil, errors.New("bad length")
	}
	m := &rtLinkMessage{name: name, hdr: *(*ifInfoMsgHdr)(unsafe.Pointer(&nlm.Data[0]))}
	b := nlm.Data[ifInfoMsgHdrLen:]
	for {
		var a rtAttribute
		var err error
		a, b, err = parseRTLinkAttribute(b)
		if b == nil {
			break
		}
		if err == nil {
			m.attributes = append(m.attributes, a)
		}
	}
	return m, nil
}

const (
	GROUPS = C.RTMGRP_LINK | C.RTMGRP_IPV4_IFADDR | C.RTMGRP_IPV4_MROUTE | C.RTMGRP_IPV4_ROUTE | C.RTMGRP_IPV6_IFADDR | C.RTMGRP_IPV6_MROUTE | C.RTMGRP_IPV6_ROUTE | C.RTMGRP_NOTIFY
)

func (n *notifier) initLocked() error {
	s, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_RAW, syscall.NETLINK_ROUTE)
	if err != nil {
		return fmt.Errorf("socket(AF_NETLINK, SOCK_RAW, NETLINK_ROUTE) failed: %v", err)
	}

	lsa := &syscall.SockaddrNetlink{Family: syscall.AF_NETLINK, Groups: GROUPS}
	if err := syscall.Bind(s, lsa); err != nil {
		syscall.Close(s)
		return fmt.Errorf("bind(%d, {AF_NETLINK, 0x%x}) failed: %v", s, lsa.Groups)
	}
	go watcher(n, s)
	return nil
}

func watcher(n *notifier, sock int) {
	defer syscall.Close(sock)
	var newAddrs []net.IP
	buf := make([]byte, 4096)
	for {
		nr, _, err := syscall.Recvfrom(sock, buf, 0)
		if err != nil {
			vlog.Infof("recvfrom(%d) on an AF_NETLINK socket failed: %v", sock, err)
			return
		}
		msgs, err := syscall.ParseNetlinkMessage(buf[:nr])
		if err != nil {
			vlog.Infof("ParseNetlinkMessage failed: %s", err)
			continue
		}
	L:
		for _, m := range msgs {
			if am, err := parsertAddressMessage(m); err == nil {
				// NOTE(p): We get continuous NEWADDR messages about some
				// IPv6 addresses in Google corp.  Just ignore duplicate back
				// to back NEWADDRs about the same addresses.
				if am.name == "RTM_NEWADDR" {
					addr := am.Address()
					for _, a := range newAddrs {
						if addr.Equal(a) {
							break L
						}
					}
					newAddrs = append(newAddrs, addr)
				} else {
					newAddrs = nil
				}
			} else if _, err := parsertLinkMessage(m); err == nil {
				newAddrs = nil
			} else {
				continue
			}
			n.ding()
		}
	}
}

func toIP(a []byte) (net.IP, error) {
	switch len(a) {
	case 4:
		return net.IPv4(a[0], a[1], a[2], a[3]), nil
	case 16:
		return net.IP(a), nil
	}
	return net.IPv6unspecified, errors.New("unknown ip address len")
}

// IPRoutes returns all kernel known routes.  If defaultOnly is set, only default routes
// are returned.
func GetIPRoutes(defaultOnly bool) []*IPRoute {
	var iproutes []*IPRoute
	rib, err := syscall.NetlinkRIB(syscall.RTM_GETROUTE, syscall.AF_UNSPEC)
	if err != nil {
		vlog.Infof("Couldn't read: %s", err)
		return iproutes
	}
	msgs, err := syscall.ParseNetlinkMessage(rib)
	if err != nil {
		vlog.Infof("Couldn't parse: %s", err)
		return iproutes
	}
L:
	for _, m := range msgs {
		if m.Header.Type != syscall.RTM_NEWROUTE {
			continue
		}
		attrs, err := syscall.ParseNetlinkRouteAttr(&m)
		if err != nil {
			continue
		}
		r := new(IPRoute)
		for _, a := range attrs {
			switch a.Attr.Type {
			case syscall.RTA_DST:
				if r.Net.IP, err = toIP(a.Value[:]); err != nil {
					continue L
				}
			case syscall.RTA_GATEWAY:
				if r.Gateway, err = toIP(a.Value[:]); err != nil {
					continue L
				}
			case syscall.RTA_OIF:
				r.IfcIndex = int(a.Value[0])
			case syscall.RTA_PREFSRC:
				if r.PreferredSource, err = toIP(a.Value[:]); err != nil {
					continue L
				}
			}
		}

		// There is no RTA_DST attribute if destination is a default gateway.
		// Set the destination IP with zero IP, if not set yet.
		if r.Net.IP == nil {
			if r.Gateway == nil {
				continue
			}
			if r.Gateway.To4() != nil {
				r.Net.IP = net.IPv4zero
			} else {
				r.Net.IP = net.IPv6zero
			}
		}

		addrLen := 128
		if r.Net.IP.To4() != nil {
			addrLen = 32
		}

		b := m.Data[:syscall.SizeofRtMsg]
		a := (*syscall.RtMsg)(unsafe.Pointer(&b[0]))
		if int(a.Dst_len) > addrLen {
			continue
		}
		r.Net.Mask = net.CIDRMask(int(a.Dst_len), addrLen)
		if !defaultOnly || IsDefaultIPRoute(r) {
			iproutes = append(iproutes, r)
		}
	}
	return iproutes
}
