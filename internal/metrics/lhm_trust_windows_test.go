package metrics

import "testing"

func TestListenerOwnership(t *testing.T) {
	// 8086 is 0x1f96; the port sits in network byte order: 0x961f.
	const port8086 = 0x961f
	loop := uint32(0x0100007f)
	for name, c := range map[string]struct {
		rows []tcpRow
		want bool
	}{
		"http.sys on loopback":       {[]tcpRow{{LocalAddr: loop, LocalPort: port8086, PID: 4}}, true},
		"program on loopback":        {[]tcpRow{{LocalAddr: loop, LocalPort: port8086, PID: 4711}}, false},
		"none":                       {nil, false},
		"other port":                 {[]tcpRow{{LocalAddr: loop, LocalPort: 0x951f, PID: 4}}, false},
		"wildcard program beside":    {[]tcpRow{{LocalAddr: loop, LocalPort: port8086, PID: 4}, {LocalAddr: 0, LocalPort: port8086, PID: 99}}, false},
		"other interface is ignored": {[]tcpRow{{LocalAddr: loop, LocalPort: port8086, PID: 4}, {LocalAddr: 0x0a00a8c0, LocalPort: port8086, PID: 99}}, true},
	} {
		if got := listenerRowsOwnedBy(c.rows, 8086, 4); got != c.want {
			t.Errorf("%s: %v, want %v", name, got, c.want)
		}
	}
}

func TestTheTableCanBeRead(t *testing.T) {
	if _, err := tcpListeners(); err != nil {
		t.Fatal(err)
	}
	if serviceRunning("CaseHUDNoSuchService") {
		t.Fatal("a missing service is not running")
	}
}
