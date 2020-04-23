// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package addrmgr_test

import (
	"errors"
	"fmt"
	"github.com/AsimovNetwork/asimov/common"
	"github.com/AsimovNetwork/asimov/socks"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/AsimovNetwork/asimov/addrmgr"
	"github.com/AsimovNetwork/asimov/protos"
)

// naTest is used to describe a test to be performed against the NetAddressKey
// method.
type naTest struct {
	in   protos.NetAddress
	want string
}

// naTests houses all of the tests to be performed against the NetAddressKey
// method.
var naTests = make([]naTest, 0)

// Put some IP in here for convenience. Points to google.
var someIP = "173.194.115.66"

// addNaTests
func addNaTests() {
	// IPv4
	// Localhost
	addNaTest("127.0.0.1", 8333, "127.0.0.1:8333")
	addNaTest("127.0.0.1", 8334, "127.0.0.1:8334")

	// Class A
	addNaTest("1.0.0.1", 8333, "1.0.0.1:8333")
	addNaTest("2.2.2.2", 8334, "2.2.2.2:8334")
	addNaTest("27.253.252.251", 8335, "27.253.252.251:8335")
	addNaTest("123.3.2.1", 8336, "123.3.2.1:8336")

	// Private Class A
	addNaTest("10.0.0.1", 8333, "10.0.0.1:8333")
	addNaTest("10.1.1.1", 8334, "10.1.1.1:8334")
	addNaTest("10.2.2.2", 8335, "10.2.2.2:8335")
	addNaTest("10.10.10.10", 8336, "10.10.10.10:8336")

	// Class B
	addNaTest("128.0.0.1", 8333, "128.0.0.1:8333")
	addNaTest("129.1.1.1", 8334, "129.1.1.1:8334")
	addNaTest("180.2.2.2", 8335, "180.2.2.2:8335")
	addNaTest("191.10.10.10", 8336, "191.10.10.10:8336")

	// Private Class B
	addNaTest("172.16.0.1", 8333, "172.16.0.1:8333")
	addNaTest("172.16.1.1", 8334, "172.16.1.1:8334")
	addNaTest("172.16.2.2", 8335, "172.16.2.2:8335")
	addNaTest("172.16.172.172", 8336, "172.16.172.172:8336")

	// Class C
	addNaTest("193.0.0.1", 8333, "193.0.0.1:8333")
	addNaTest("200.1.1.1", 8334, "200.1.1.1:8334")
	addNaTest("205.2.2.2", 8335, "205.2.2.2:8335")
	addNaTest("223.10.10.10", 8336, "223.10.10.10:8336")

	// Private Class C
	addNaTest("192.168.0.1", 8333, "192.168.0.1:8333")
	addNaTest("192.168.1.1", 8334, "192.168.1.1:8334")
	addNaTest("192.168.2.2", 8335, "192.168.2.2:8335")
	addNaTest("192.168.192.192", 8336, "192.168.192.192:8336")

	// IPv6
	// Localhost
	addNaTest("::1", 8333, "[::1]:8333")
	addNaTest("fe80::1", 8334, "[fe80::1]:8334")

	// Link-local
	addNaTest("fe80::1:1", 8333, "[fe80::1:1]:8333")
	addNaTest("fe91::2:2", 8334, "[fe91::2:2]:8334")
	addNaTest("fea2::3:3", 8335, "[fea2::3:3]:8335")
	addNaTest("feb3::4:4", 8336, "[feb3::4:4]:8336")

	// Site-local
	addNaTest("fec0::1:1", 8333, "[fec0::1:1]:8333")
	addNaTest("fed1::2:2", 8334, "[fed1::2:2]:8334")
	addNaTest("fee2::3:3", 8335, "[fee2::3:3]:8335")
	addNaTest("fef3::4:4", 8336, "[fef3::4:4]:8336")
}

func addNaTest(ip string, port uint16, want string) {
	nip := net.ParseIP(ip)
	na := *protos.NewNetAddressIPPort(nip, port, common.SFNodeNetwork)
	test := naTest{na, want}
	naTests = append(naTests, test)
}

type mockNetAdapter struct {
	proxy *socks.Proxy
	onionProxy *socks.Proxy
	noOnion bool
	timeout time.Duration
}

func (na * mockNetAdapter) DialTimeout(addr net.Addr) (net.Conn, error) {
	if strings.Contains(addr.String(), ".onion:") {
		// noonion means the onion address dial function results in
		// an error.
		if na.noOnion {
			return nil, errors.New("tor has been disabled")
		}
		if na.onionProxy != nil {
			return na.onionProxy.DialTimeout(addr.Network(), addr.String(), na.timeout)
		}
	}

	if na.proxy != nil {
		return na.proxy.DialTimeout(addr.Network(), addr.String(), na.timeout)
	}

	return net.DialTimeout(addr.Network(), addr.String(), na.timeout)
}

// Lookup resolves the IP of the given host using the correct DNS lookup
// function depending on the configuration options.  For example, addresses will
// be resolved using tor when the --proxy flag was specified unless --noonion
// was also specified in which case the normal system DNS resolver will be used.
//
// Any attempt to resolve a tor address (.onion) will return an error since they
// are not intended to be resolved outside of the tor proxy.
func (na * mockNetAdapter) Lookup(host string) ([]net.IP, error) {
	return nil, errors.New("not implemented")
}

func (na * mockNetAdapter) SupportOnion() bool {
	return !na.noOnion
}

var defaultTestNetAdapter = &mockNetAdapter {
noOnion: true,
timeout: time.Second * 1,
}

func TestStartStop(t *testing.T) {
	n := addrmgr.New("teststartstop", defaultTestNetAdapter)
	n.Start()
	err := n.Stop()
	if err != nil {
		t.Fatalf("Address Manager failed to stop: %v", err)
	}
}

func TestAddAddressByIP(t *testing.T) {
	fmtErr := fmt.Errorf("")
	addrErr := &net.AddrError{}
	var tests = []struct {
		addrIP string
		err    error
	}{
		{
			someIP + ":8333",
			nil,
		},
		{
			someIP,
			addrErr,
		},
		{
			someIP[:12] + ":8333",
			fmtErr,
		},
		{
			someIP + ":abcd",
			fmtErr,
		},
	}

	amgr := addrmgr.New("testaddressbyip", nil)
	for i, test := range tests {
		err := amgr.AddAddressByIP(test.addrIP)
		if test.err != nil && err == nil {
			t.Errorf("TestGood test %d failed expected an error and got none", i)
			continue
		}
		if test.err == nil && err != nil {
			t.Errorf("TestGood test %d failed expected no error and got one", i)
			continue
		}
		if reflect.TypeOf(err) != reflect.TypeOf(test.err) {
			t.Errorf("TestGood test %d failed got %v, want %v", i,
				reflect.TypeOf(err), reflect.TypeOf(test.err))
			continue
		}
	}
}

func TestAddLocalAddress(t *testing.T) {
	var tests = []struct {
		address  protos.NetAddress
		priority addrmgr.AddressPriority
		valid    bool
	}{
		{
			protos.NetAddress{IP: net.ParseIP("192.168.0.100")},
			addrmgr.InterfacePrio,
			false,
		},
		{
			protos.NetAddress{IP: net.ParseIP("204.124.1.1")},
			addrmgr.InterfacePrio,
			true,
		},
		{
			protos.NetAddress{IP: net.ParseIP("204.124.1.1")},
			addrmgr.BoundPrio,
			true,
		},
		{
			protos.NetAddress{IP: net.ParseIP("::1")},
			addrmgr.InterfacePrio,
			false,
		},
		{
			protos.NetAddress{IP: net.ParseIP("fe80::1")},
			addrmgr.InterfacePrio,
			false,
		},
		{
			protos.NetAddress{IP: net.ParseIP("2620:100::1")},
			addrmgr.InterfacePrio,
			true,
		},
	}
	amgr := addrmgr.New("testaddlocaladdress", nil)
	for x, test := range tests {
		result := amgr.AddLocalAddress(&test.address, test.priority)
		if result == nil && !test.valid {
			t.Errorf("TestAddLocalAddress test #%d failed: %s should have "+
				"been accepted", x, test.address.IP)
			continue
		}
		if result != nil && test.valid {
			t.Errorf("TestAddLocalAddress test #%d failed: %s should not have "+
				"been accepted", x, test.address.IP)
			continue
		}
	}
}

func TestAttempt(t *testing.T) {
	n := addrmgr.New("testattempt", defaultTestNetAdapter)

	// Add a new address and get it
	err := n.AddAddressByIP(someIP + ":8333")
	if err != nil {
		t.Fatalf("Adding address failed: %v", err)
	}
	ka := n.GetAddress()

	if !ka.LastAttempt().IsZero() {
		t.Errorf("Address should not have attempts, but does")
	}

	na := ka.NetAddress()
	n.Attempt(na)

	if ka.LastAttempt().IsZero() {
		t.Errorf("Address should have an attempt, but does not")
	}
}

func TestConnected(t *testing.T) {
	n := addrmgr.New("testconnected", defaultTestNetAdapter)

	// Add a new address and get it
	err := n.AddAddressByIP(someIP + ":8333")
	if err != nil {
		t.Fatalf("Adding address failed: %v", err)
	}
	ka := n.GetAddress()
	na := ka.NetAddress()
	// make it an hour ago
	na.Timestamp = time.Unix(time.Now().Add(time.Hour*-1).Unix(), 0)

	n.Connected(na)

	if !ka.NetAddress().Timestamp.After(na.Timestamp) {
		t.Errorf("Address should have a new timestamp, but does not")
	}
}

func TestNeedMoreAddresses(t *testing.T) {
	n := addrmgr.New("testneedmoreaddresses", defaultTestNetAdapter)
	addrsToAdd := 1500
	b := n.NeedMoreAddresses()
	if !b {
		t.Errorf("Expected that we need more addresses")
	}
	addrs := make([]*protos.NetAddress, addrsToAdd)

	var err error
	for i := 0; i < addrsToAdd; i++ {
		s := fmt.Sprintf("%d.%d.173.147:8333", i/128+60, i%128+60)
		addrs[i], err = n.DeserializeNetAddress(s, common.SFNodeNetwork)
		if err != nil {
			t.Errorf("Failed to turn %s into an address: %v", s, err)
		}
	}

	srcAddr := protos.NewNetAddressIPPort(net.IPv4(173, 144, 173, 111), 8333, 0)

	n.AddAddresses(addrs, srcAddr)
	numAddrs := n.NumAddresses()
	if numAddrs > addrsToAdd {
		t.Errorf("Number of addresses is too many %d vs %d", numAddrs, addrsToAdd)
	}

	b = n.NeedMoreAddresses()
	if b {
		t.Errorf("Expected that we don't need more addresses")
	}
}

func TestGood(t *testing.T) {
	n := addrmgr.New("testgood", defaultTestNetAdapter)
	addrsToAdd := 64 * 64
	addrs := make([]*protos.NetAddress, addrsToAdd)

	var err error
	for i := 0; i < addrsToAdd; i++ {
		s := fmt.Sprintf("%d.173.147.%d:8333", i/64+60, i%64+60)
		addrs[i], err = n.DeserializeNetAddress(s, common.SFNodeNetwork)
		if err != nil {
			t.Errorf("Failed to turn %s into an address: %v", s, err)
		}
	}

	srcAddr := protos.NewNetAddressIPPort(net.IPv4(173, 144, 173, 111), 8333, 0)

	n.AddAddresses(addrs, srcAddr)
	for _, addr := range addrs {
		n.Good(addr)
	}

	numAddrs := n.NumAddresses()
	if numAddrs >= addrsToAdd {
		t.Errorf("Number of addresses is too many: %d vs %d", numAddrs, addrsToAdd)
	}

	numCache := len(n.AddressCache())
	if numCache >= numAddrs/4 {
		t.Errorf("Number of addresses in cache: got %d, want %d", numCache, numAddrs/4)
	}
}

func TestGetAddress(t *testing.T) {
	n := addrmgr.New("testgetaddress", defaultTestNetAdapter)

	// Get an address from an empty set (should error)
	if rv := n.GetAddress(); rv != nil {
		t.Errorf("GetAddress failed: got: %v want: %v\n", rv, nil)
	}

	// Add a new address and get it
	err := n.AddAddressByIP(someIP + ":8333")
	if err != nil {
		t.Fatalf("Adding address failed: %v", err)
	}
	ka := n.GetAddress()
	if ka == nil {
		t.Fatalf("Did not get an address where there is one in the pool")
	}
	if ka.NetAddress().IP.String() != someIP {
		t.Errorf("Wrong IP: got %v, want %v", ka.NetAddress().IP.String(), someIP)
	}

	// Mark this as a good address and get it
	n.Good(ka.NetAddress())
	ka = n.GetAddress()
	if ka == nil {
		t.Fatalf("Did not get an address where there is one in the pool")
	}
	if ka.NetAddress().IP.String() != someIP {
		t.Errorf("Wrong IP: got %v, want %v", ka.NetAddress().IP.String(), someIP)
	}

	numAddrs := n.NumAddresses()
	if numAddrs != 1 {
		t.Errorf("Wrong number of addresses: got %d, want %d", numAddrs, 1)
	}
}

func TestGetBestLocalAddress(t *testing.T) {
	localAddrs := []protos.NetAddress{
		{IP: net.ParseIP("192.168.0.100")},
		{IP: net.ParseIP("::1")},
		{IP: net.ParseIP("fe80::1")},
		{IP: net.ParseIP("2001:470::1")},
	}

	var tests = []struct {
		remoteAddr protos.NetAddress
		want0      protos.NetAddress
		want1      protos.NetAddress
		want2      protos.NetAddress
		want3      protos.NetAddress
	}{
		{
			// Remote connection from public IPv4
			protos.NetAddress{IP: net.ParseIP("204.124.8.1")},
			protos.NetAddress{IP: net.IPv4zero},
			protos.NetAddress{IP: net.IPv4zero},
			protos.NetAddress{IP: net.ParseIP("204.124.8.100")},
			protos.NetAddress{IP: net.ParseIP("fd87:d87e:eb43:25::1")},
		},
		{
			// Remote connection from private IPv4
			protos.NetAddress{IP: net.ParseIP("172.16.0.254")},
			protos.NetAddress{IP: net.IPv4zero},
			protos.NetAddress{IP: net.IPv4zero},
			protos.NetAddress{IP: net.IPv4zero},
			protos.NetAddress{IP: net.IPv4zero},
		},
		{
			// Remote connection from public IPv6
			protos.NetAddress{IP: net.ParseIP("2602:100:abcd::102")},
			protos.NetAddress{IP: net.IPv6zero},
			protos.NetAddress{IP: net.ParseIP("2001:470::1")},
			protos.NetAddress{IP: net.ParseIP("2001:470::1")},
			protos.NetAddress{IP: net.ParseIP("2001:470::1")},
		},
		/* XXX
		{
			// Remote connection from Tor
			protos.NetAddress{IP: net.ParseIP("fd87:d87e:eb43::100")},
			protos.NetAddress{IP: net.IPv4zero},
			protos.NetAddress{IP: net.ParseIP("204.124.8.100")},
			protos.NetAddress{IP: net.ParseIP("fd87:d87e:eb43:25::1")},
		},
		*/
	}

	amgr := addrmgr.New("testgetbestlocaladdress", nil)

	// Test against default when there's no address
	for x, test := range tests {
		got := amgr.GetBestLocalAddress(&test.remoteAddr)
		if !test.want0.IP.Equal(got.IP) {
			t.Errorf("TestGetBestLocalAddress test1 #%d failed for remote address %s: want %s got %s",
				x, test.remoteAddr.IP, test.want1.IP, got.IP)
			continue
		}
	}

	for _, localAddr := range localAddrs {
		amgr.AddLocalAddress(&localAddr, addrmgr.InterfacePrio)
	}

	// Test against want1
	for x, test := range tests {
		got := amgr.GetBestLocalAddress(&test.remoteAddr)
		if !test.want1.IP.Equal(got.IP) {
			t.Errorf("TestGetBestLocalAddress test1 #%d failed for remote address %s: want %s got %s",
				x, test.remoteAddr.IP, test.want1.IP, got.IP)
			continue
		}
	}

	// Add a public IP to the list of local addresses.
	localAddr := protos.NetAddress{IP: net.ParseIP("204.124.8.100")}
	amgr.AddLocalAddress(&localAddr, addrmgr.InterfacePrio)

	// Test against want2
	for x, test := range tests {
		got := amgr.GetBestLocalAddress(&test.remoteAddr)
		if !test.want2.IP.Equal(got.IP) {
			t.Errorf("TestGetBestLocalAddress test2 #%d failed for remote address %s: want %s got %s",
				x, test.remoteAddr.IP, test.want2.IP, got.IP)
			continue
		}
	}
	/*
		// Add a Tor generated IP address
		localAddr = protos.NetAddress{IP: net.ParseIP("fd87:d87e:eb43:25::1")}
		amgr.AddLocalAddress(&localAddr, addrmgr.ManualPrio)

		// Test against want3
		for x, test := range tests {
			got := amgr.GetBestLocalAddress(&test.remoteAddr)
			if !test.want3.IP.Equal(got.IP) {
				t.Errorf("TestGetBestLocalAddress test3 #%d failed for remote address %s: want %s got %s",
					x, test.remoteAddr.IP, test.want3.IP, got.IP)
				continue
			}
		}
	*/
}

func TestNetAddressKey(t *testing.T) {
	addNaTests()

	t.Logf("Running %d tests", len(naTests))
	for i, test := range naTests {
		key := addrmgr.NetAddressKey(&test.in)
		if key != test.want {
			t.Errorf("NetAddressKey #%d\n got: %s want: %s", i, key, test.want)
			continue
		}
	}

}