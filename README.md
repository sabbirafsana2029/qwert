# Linux Network Namespace Simulation in Go

This project creates two isolated network namespaces connected via software bridges and a central router namespace — implemented entirely in Go using the `netlink` and `netns` libraries.

---

## 📊 Network Topology

```
  ┌────────┐        ┌──────────────────────────┐        ┌────────┐
  │  ns1   │        │        router-ns          │        │  ns2   │
  │        │        │  veth-r0      veth-r1     │        │        │
  │veth-ns1│        │  10.0.1.1     10.0.2.1   │        │veth-ns2│
  │10.0.1.10│       └────┬──────────────┬───────┘        │10.0.2.10│
  └───┬────┘        veth-br0-r      veth-br1-r       └────┬───┘
      │                  │               │                 │
  veth-br0          ─────┘           ────┘            veth-br1
      │                                                    │
  ───[br0] 10.0.1.254/24 ──────────────── [br1] 10.0.2.254/24───

  Subnet A: 10.0.1.0/24          Subnet B: 10.0.2.0/24
```

---

## 🗂️ IP Addressing Scheme

| Namespace  | Interface  | IP Address      | Gateway      |
|------------|------------|-----------------|--------------|
| ns1        | veth-ns1   | 10.0.1.10/24    | 10.0.1.254   |
| ns2        | veth-ns2   | 10.0.2.10/24    | 10.0.2.254   |
| router-ns  | veth-r0    | 10.0.1.1/24     | —            |
| router-ns  | veth-r1    | 10.0.2.1/24     | —            |
| host       | br0        | 10.0.1.254/24   | —            |
| host       | br1        | 10.0.2.254/24   | —            |

---

## 🚀 How to Run

> All commands require root privileges.

### 1. Setup the network
```bash
make setup
```

### 2. Test connectivity
```bash
make test
```
This runs ping from ns1 → ns2 and ns2 → ns1.

### 3. Manual ping test
```bash
sudo ip netns exec ns1 ping -c 3 10.0.2.10
```

### 4. Clean up everything
```bash
make clean
```

---

## 🔧 What Was Fixed (vs original)

| Issue | Fix Applied |
|---|---|
| IP forwarding written to host namespace | Now written inside `router-ns` by switching namespace before the `WriteFile` call |
| Loopback `lo` never brought up | `lo` is explicitly brought up in ns1, ns2, and router-ns |
| All errors silently ignored | Full error handling with descriptive messages on every `netlink` call |
| No cleanup on crash | `defer cleanup()` registered at start of `main()` |
| README incomplete | Complete documentation added |

---

## 📁 File Structure

```
.
├── main.go       # Full Go implementation
├── go.mod        # Module dependencies
├── Makefile      # setup / test / clean targets
└── README.md     # This file
```

---

## 📦 Dependencies

- [`github.com/vishvananda/netlink`](https://github.com/vishvananda/netlink) — manage network interfaces, addresses, routes
- [`github.com/vishvananda/netns`](https://github.com/vishvananda/netns) — create and switch Linux network namespaces
- Go 1.21+
