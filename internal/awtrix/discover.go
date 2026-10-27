package awtrix

import (
	"context"
	"net"
	"slices"
	"strings"
	"sync"
	"time"
)

// Found is a clock found on the network.
type Found struct {
	Host string `json:"host"` // what to store: a .local name that resolves, else the IP
	IP   string `json:"ip"`
	Info Info   `json:"info"`
}

const (
	discoveryPort = 4210 // clocks listen here for FIND_AWTRIX / FIND_AWTRIXNG
	replyPort     = 4211 // and answer here with their host name
	maxSweep      = 1024 // hosts per network the HTTP sweep tries at most
	sweepWorkers  = 96
)

// Discover looks for clocks on the local networks: the UDP broadcast both
// firmwares answer, plus an HTTP sweep of the local subnets for when a
// firewall swallows the UDP answers. It returns within about timeout.
func Discover(ctx context.Context, timeout time.Duration) []Found {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	nets := localNets()

	var mu sync.Mutex
	cands := map[string]string{} // ip -> announced name
	add := func(ip, name string) {
		mu.Lock()
		if _, ok := cands[ip]; !ok || name != "" {
			cands[ip] = name
		}
		mu.Unlock()
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); broadcast(ctx, nets, add) }()
	go func() { defer wg.Done(); sweep(ctx, nets, add) }()
	wg.Wait()

	// Identify every candidate (the sweep already did, but cheaply again).
	var out []Found
	seen := map[string]bool{}
	pctx, pcancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer pcancel()
	var omu sync.Mutex
	var pw sync.WaitGroup
	for ip, name := range cands {
		pw.Add(1)
		go func(ip, name string) {
			defer pw.Done()
			info, err := newClient(ip, 2*time.Second).Probe(pctx)
			if err != nil {
				return
			}
			if info.Name == "" || info.Firmware == Awtrix3 {
				if name != "" {
					info.Name = name
				}
			}
			omu.Lock()
			defer omu.Unlock()
			key := info.UID
			if key == "" {
				key = ip
			}
			if seen[key] {
				return
			}
			seen[key] = true
			out = append(out, Found{Host: preferName(info.Name, ip), IP: ip, Info: info})
		}(ip, name)
	}
	pw.Wait()
	slices.SortFunc(out, func(a, b Found) int { return strings.Compare(a.IP, b.IP) })
	return out
}

// preferName returns name.local when it resolves to ip (survives a new DHCP
// lease), else the IP.
func preferName(name, ip string) string {
	name = strings.TrimSuffix(strings.TrimSpace(name), ".local")
	if name == "" || strings.ContainsAny(name, " :/") {
		return ip
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupHost(ctx, name+".local")
	if err == nil && slices.Contains(addrs, ip) {
		return name + ".local"
	}
	return ip
}

// localNets are the private IPv4 networks this PC is on.
func localNets() []*net.IPNet {
	var out []*net.IPNet
	ifs, _ := net.Interfaces()
	for _, ifc := range ifs {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			n, ok := a.(*net.IPNet)
			if !ok || n.IP.To4() == nil || !n.IP.IsPrivate() {
				continue
			}
			out = append(out, &net.IPNet{IP: n.IP.To4(), Mask: n.Mask})
		}
	}
	return out
}

func broadcastAddr(n *net.IPNet) net.IP {
	ip := n.IP.To4()
	m := net.IP(n.Mask).To4()
	if m == nil {
		m = net.IP(n.Mask[len(n.Mask)-4:])
	}
	b := make(net.IP, 4)
	for i := range b {
		b[i] = ip[i] | ^m[i]
	}
	return b
}

// broadcast sends the discovery texts and collects the answers.
func broadcast(ctx context.Context, nets []*net.IPNet, add func(ip, name string)) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{Port: replyPort})
	if err != nil {
		return // port taken (another AWTRIX tool): the sweep still works
	}
	defer conn.Close()
	targets := []net.IP{net.IPv4bcast}
	for _, n := range nets {
		targets = append(targets, broadcastAddr(n))
	}
	send := func() {
		for _, t := range targets {
			for _, msg := range []string{"FIND_AWTRIX", "FIND_AWTRIXNG"} {
				conn.WriteToUDP([]byte(msg), &net.UDPAddr{IP: t, Port: discoveryPort})
			}
		}
	}
	send()
	go func() {
		// UDP on Wi-Fi gets lost; ask twice more.
		for _, d := range []time.Duration{400 * time.Millisecond, 1200 * time.Millisecond} {
			select {
			case <-ctx.Done():
				return
			case <-time.After(d):
				send()
			}
		}
	}()
	go func() { <-ctx.Done(); conn.SetReadDeadline(time.Now()) }()
	buf := make([]byte, 256)
	for {
		n, from, err := conn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		name := strings.TrimSpace(string(buf[:n]))
		if i := strings.IndexByte(name, ':'); i >= 0 {
			name = name[:i] // "host:port" when the web port is not 80
		}
		if strings.HasPrefix(name, "FIND_AWTRIX") {
			continue // our own broadcast
		}
		add(from.IP.String(), name)
	}
}

// sweep asks every address of the local subnets for an AWTRIX API.
func sweep(ctx context.Context, nets []*net.IPNet, add func(ip, name string)) {
	ips := make(chan string)
	var wg sync.WaitGroup
	for range sweepWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ip := range ips {
				if !portOpen(ctx, ip) {
					continue
				}
				if _, err := newClient(ip, 1500*time.Millisecond).Probe(ctx); err == nil {
					add(ip, "")
				}
			}
		}()
	}
	own := map[string]bool{}
	for _, n := range nets {
		own[n.IP.String()] = true
	}
feed:
	for _, n := range nets {
		for _, ip := range hosts(n) {
			if own[ip] {
				continue
			}
			select {
			case ips <- ip:
			case <-ctx.Done():
				break feed
			}
		}
	}
	close(ips)
	wg.Wait()
}

// portOpen is a quick TCP check so the sweep spends no HTTP round trip on
// addresses without a web server.
func portOpen(ctx context.Context, ip string) bool {
	d := net.Dialer{Timeout: 400 * time.Millisecond}
	c, err := d.DialContext(ctx, "tcp4", net.JoinHostPort(ip, "80"))
	if err != nil {
		return false
	}
	c.Close()
	return true
}

// hosts lists the usable addresses of n, at most maxSweep around this PC's
// own address.
func hosts(n *net.IPNet) []string {
	ones, bits := n.Mask.Size()
	if bits != 32 || ones < 16 {
		return nil
	}
	size := 1 << (32 - ones)
	base := ipToInt(n.IP.Mask(n.Mask))
	start, end := 1, size-1 // skip network and broadcast address
	if size-2 > maxSweep {
		// A big network: sweep the /22 block this PC sits in.
		own := ipToInt(n.IP) - base
		start = own &^ (maxSweep - 1)
		end = start + maxSweep
		if start == 0 {
			start = 1
		}
	}
	out := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		out = append(out, intToIP(base+i).String())
	}
	return out
}

func ipToInt(ip net.IP) int {
	ip = ip.To4()
	return int(ip[0])<<24 | int(ip[1])<<16 | int(ip[2])<<8 | int(ip[3])
}

func intToIP(v int) net.IP {
	return net.IPv4(byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}
