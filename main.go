package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"runtime"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

func main() {
	// Lock the OS Thread: Namespaces are thread-local in Linux
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if os.Geteuid() != 0 {
		log.Fatal("Error: This program must be run as root.")
	}

	// Save root namespace so we can return to it
	rootNs, err := netns.Get()
	if err != nil {
		log.Fatalf("Failed to get root namespace: %v", err)
	}
	defer rootNs.Close()

	// Register cleanup on exit
	defer cleanup()

	// ─────────────────────────────────────────────
	// 1. Create Namespaces
	// ─────────────────────────────────────────────
	ns1, err := netns.NewNamed("ns1")
	if err != nil {
		log.Fatalf("Failed to create ns1: %v", err)
	}
	defer ns1.Close()

	// Return to root before creating next namespace
	if err := netns.Set(rootNs); err != nil {
		log.Fatalf("Failed to return to root ns: %v", err)
	}

	ns2, err := netns.NewNamed("ns2")
	if err != nil {
		log.Fatalf("Failed to create ns2: %v", err)
	}
	defer ns2.Close()

	if err := netns.Set(rootNs); err != nil {
		log.Fatalf("Failed to return to root ns: %v", err)
	}

	routerNs, err := netns.NewNamed("router-ns")
	if err != nil {
		log.Fatalf("Failed to create router-ns: %v", err)
	}
	defer routerNs.Close()

	if err := netns.Set(rootNs); err != nil {
		log.Fatalf("Failed to return to root ns: %v", err)
	}

	fmt.Println("✅ Namespaces Created: ns1, ns2, router-ns")

	// ─────────────────────────────────────────────
	// 2. Setup Bridges in Root Namespace
	// ─────────────────────────────────────────────
	br0, err := setupBridge("br0", "10.0.1.254/24")
	if err != nil {
		log.Fatalf("Failed to setup br0: %v", err)
	}

	br1, err := setupBridge("br1", "10.0.2.254/24")
	if err != nil {
		log.Fatalf("Failed to setup br1: %v", err)
	}

	fmt.Println("✅ Bridges Created: br0 (10.0.1.254/24), br1 (10.0.2.254/24)")

	// ─────────────────────────────────────────────
	// 3. Create veth pairs and connect to bridges
	// ─────────────────────────────────────────────
	//
	// Topology:
	//   ns1 (veth-ns1) <──> (veth-br0) br0 <──> (veth-br0-r) router-ns (veth-r0)
	//   ns2 (veth-ns2) <──> (veth-br1) br1 <──> (veth-br1-r) router-ns (veth-r1)

	if err := connectToBridge("veth-ns1", "veth-br0", ns1, br0); err != nil {
		log.Fatalf("Failed to connect ns1 to br0: %v", err)
	}
	if err := connectToBridge("veth-ns2", "veth-br1", ns2, br1); err != nil {
		log.Fatalf("Failed to connect ns2 to br1: %v", err)
	}
	if err := connectToBridge("veth-r0", "veth-br0-r", routerNs, br0); err != nil {
		log.Fatalf("Failed to connect router-ns to br0: %v", err)
	}
	if err := connectToBridge("veth-r1", "veth-br1-r", routerNs, br1); err != nil {
		log.Fatalf("Failed to connect router-ns to br1: %v", err)
	}

	fmt.Println("✅ veth Pairs Created and Connected to Bridges")

	// ─────────────────────────────────────────────
	// 4. Configure IPs in ns1 and ns2
	// ─────────────────────────────────────────────
	if err := configureInterface(rootNs, ns1, "veth-ns1", "10.0.1.10/24", "10.0.1.254"); err != nil {
		log.Fatalf("Failed to configure ns1 interface: %v", err)
	}
	fmt.Println("✅ ns1: veth-ns1 = 10.0.1.10/24, gateway = 10.0.1.254")

	if err := configureInterface(rootNs, ns2, "veth-ns2", "10.0.2.10/24", "10.0.2.254"); err != nil {
		log.Fatalf("Failed to configure ns2 interface: %v", err)
	}
	fmt.Println("✅ ns2: veth-ns2 = 10.0.2.10/24, gateway = 10.0.2.254")

	// ─────────────────────────────────────────────
	// 5. Configure Router Namespace
	// ─────────────────────────────────────────────
	if err := configureRouter(rootNs, routerNs); err != nil {
		log.Fatalf("Failed to configure router-ns: %v", err)
	}
	fmt.Println("✅ router-ns: veth-r0 = 10.0.1.1/24, veth-r1 = 10.0.2.1/24, IP forwarding ON")

	fmt.Println("\n🚀 Network Simulation is live!")
	fmt.Println("   Run: sudo ip netns exec ns1 ping -c 3 10.0.2.10")
}

// ─────────────────────────────────────────────────────────────────────────────
// setupBridge creates a bridge, assigns an IP, and brings it up.
// ─────────────────────────────────────────────────────────────────────────────
func setupBridge(name, cidr string) (*netlink.Bridge, error) {
	br := &netlink.Bridge{LinkAttrs: netlink.LinkAttrs{Name: name}}
	if err := netlink.LinkAdd(br); err != nil {
		return nil, fmt.Errorf("LinkAdd(%s): %w", name, err)
	}

	link, err := netlink.LinkByName(name)
	if err != nil {
		return nil, fmt.Errorf("LinkByName(%s): %w", name, err)
	}

	addr, err := netlink.ParseAddr(cidr)
	if err != nil {
		return nil, fmt.Errorf("ParseAddr(%s): %w", cidr, err)
	}
	if err := netlink.AddrAdd(link, addr); err != nil {
		return nil, fmt.Errorf("AddrAdd(%s, %s): %w", name, cidr, err)
	}

	if err := netlink.LinkSetUp(link); err != nil {
		return nil, fmt.Errorf("LinkSetUp(%s): %w", name, err)
	}

	return br, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// connectToBridge creates a veth pair, moves one end (vethName) into targetNs,
// and attaches the other end (peerName) to the bridge — all from root namespace.
// ─────────────────────────────────────────────────────────────────────────────
func connectToBridge(vethName, peerName string, targetNs netns.NsHandle, bridge *netlink.Bridge) error {
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: vethName},
		PeerName:  peerName,
	}
	if err := netlink.LinkAdd(veth); err != nil {
		return fmt.Errorf("LinkAdd veth pair (%s<->%s): %w", vethName, peerName, err)
	}

	// Move vethName end into the target namespace
	nsEnd, err := netlink.LinkByName(vethName)
	if err != nil {
		return fmt.Errorf("LinkByName(%s): %w", vethName, err)
	}
	if err := netlink.LinkSetNsFd(nsEnd, int(targetNs)); err != nil {
		return fmt.Errorf("LinkSetNsFd(%s): %w", vethName, err)
	}

	// Attach bridge end to the bridge
	brEnd, err := netlink.LinkByName(peerName)
	if err != nil {
		return fmt.Errorf("LinkByName(%s): %w", peerName, err)
	}
	if err := netlink.LinkSetMaster(brEnd, bridge); err != nil {
		return fmt.Errorf("LinkSetMaster(%s): %w", peerName, err)
	}
	if err := netlink.LinkSetUp(brEnd); err != nil {
		return fmt.Errorf("LinkSetUp(%s): %w", peerName, err)
	}

	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// configureInterface switches into targetNs, brings up loopback + the interface,
// assigns the CIDR, sets the default gateway, then returns to rootNs.
// ─────────────────────────────────────────────────────────────────────────────
func configureInterface(rootNs, targetNs netns.NsHandle, ifaceName, cidr, gateway string) error {
	if err := netns.Set(targetNs); err != nil {
		return fmt.Errorf("Set targetNs: %w", err)
	}
	defer netns.Set(rootNs) // always return to root

	// Bring up loopback
	lo, err := netlink.LinkByName("lo")
	if err != nil {
		return fmt.Errorf("LinkByName(lo): %w", err)
	}
	if err := netlink.LinkSetUp(lo); err != nil {
		return fmt.Errorf("LinkSetUp(lo): %w", err)
	}

	// Configure the veth interface
	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("LinkByName(%s): %w", ifaceName, err)
	}

	addr, err := netlink.ParseAddr(cidr)
	if err != nil {
		return fmt.Errorf("ParseAddr(%s): %w", cidr, err)
	}
	if err := netlink.AddrAdd(link, addr); err != nil {
		return fmt.Errorf("AddrAdd(%s): %w", ifaceName, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("LinkSetUp(%s): %w", ifaceName, err)
	}

	// Add default route via gateway
	if gateway != "" {
		gw := net.ParseIP(gateway)
		if gw == nil {
			return fmt.Errorf("invalid gateway IP: %s", gateway)
		}
		route := &netlink.Route{
			Scope: netlink.SCOPE_UNIVERSE,
			Gw:    gw,
		}
		if err := netlink.RouteAdd(route); err != nil {
			return fmt.Errorf("RouteAdd(gw=%s): %w", gateway, err)
		}
	}

	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// configureRouter sets up both router interfaces and enables IP forwarding
// *inside* router-ns by writing to the namespace-specific /proc path.
// ─────────────────────────────────────────────────────────────────────────────
func configureRouter(rootNs, routerNs netns.NsHandle) error {
	if err := netns.Set(routerNs); err != nil {
		return fmt.Errorf("Set routerNs: %w", err)
	}
	defer netns.Set(rootNs)

	// Bring up loopback
	lo, err := netlink.LinkByName("lo")
	if err != nil {
		return fmt.Errorf("LinkByName(lo): %w", err)
	}
	if err := netlink.LinkSetUp(lo); err != nil {
		return fmt.Errorf("LinkSetUp(lo): %w", err)
	}

	// Configure veth-r0 (faces br0 / 10.0.1.0/24)
	r0, err := netlink.LinkByName("veth-r0")
	if err != nil {
		return fmt.Errorf("LinkByName(veth-r0): %w", err)
	}
	ra0, err := netlink.ParseAddr("10.0.1.1/24")
	if err != nil {
		return fmt.Errorf("ParseAddr(veth-r0): %w", err)
	}
	if err := netlink.AddrAdd(r0, ra0); err != nil {
		return fmt.Errorf("AddrAdd(veth-r0): %w", err)
	}
	if err := netlink.LinkSetUp(r0); err != nil {
		return fmt.Errorf("LinkSetUp(veth-r0): %w", err)
	}

	// Configure veth-r1 (faces br1 / 10.0.2.0/24)
	r1, err := netlink.LinkByName("veth-r1")
	if err != nil {
		return fmt.Errorf("LinkByName(veth-r1): %w", err)
	}
	ra1, err := netlink.ParseAddr("10.0.2.1/24")
	if err != nil {
		return fmt.Errorf("ParseAddr(veth-r1): %w", err)
	}
	if err := netlink.AddrAdd(r1, ra1); err != nil {
		return fmt.Errorf("AddrAdd(veth-r1): %w", err)
	}
	if err := netlink.LinkSetUp(r1); err != nil {
		return fmt.Errorf("LinkSetUp(veth-r1): %w", err)
	}

	// Enable IP forwarding INSIDE router-ns
	// We are already switched into router-ns, so /proc/sys/net/ipv4/ip_forward
	// refers to THIS namespace's forwarding setting.
	if err := os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1\n"), 0644); err != nil {
		return fmt.Errorf("enable ip_forward in router-ns: %w", err)
	}

	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// cleanup removes all namespaces and bridges (deferred safety net).
// The Makefile `clean` target also handles this via ip commands.
// ─────────────────────────────────────────────────────────────────────────────
func cleanup() {
	for _, ns := range []string{"ns1", "ns2", "router-ns"} {
		netns.DeleteNamed(ns) // ignore errors — may not exist
	}
}
