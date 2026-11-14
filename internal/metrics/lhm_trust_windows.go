package metrics

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// serviceName is the Windows service that serves ServiceURL.
const serviceName = "CaseHUDSensors"

// serviceTrusted tells whether what listens on the service port is the sensor
// service. Two things have to hold:
//   - the listener on 127.0.0.1:8086 belongs to the kernel (PID 4, http.sys),
//     which the service registers its address with; a program that opened the
//     port itself has its own PID, and
//   - the service is running. http.sys also serves registrations of ordinary
//     programs, but not for an address the running service has registered.
//
// If either cannot be established the answer is no.
func serviceTrusted() bool {
	return serviceRunning(serviceName) && listenerOwnedBy(8086, httpSysPID)
}

// httpSysPID is the process that owns the sockets of http.sys.
const httpSysPID = 4

func serviceRunning(name string) bool {
	m, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return false
	}
	defer windows.CloseServiceHandle(m)
	n, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return false
	}
	s, err := windows.OpenService(m, n, windows.SERVICE_QUERY_STATUS)
	if err != nil {
		return false
	}
	defer windows.CloseServiceHandle(s)
	var st windows.SERVICE_STATUS
	if err := windows.QueryServiceStatus(s, &st); err != nil {
		return false
	}
	return st.CurrentState == windows.SERVICE_RUNNING
}

var procGetExtendedTcpTable = windows.NewLazySystemDLL("iphlpapi.dll").NewProc("GetExtendedTcpTable")

const (
	afInet                 = 2
	tcpTableOwnerPIDListen = 3 // TCP_TABLE_OWNER_PID_LISTENER
)

// tcpRow is MIB_TCPROW_OWNER_PID.
type tcpRow struct {
	State, LocalAddr, LocalPort, RemoteAddr, RemotePort, PID uint32
}

// listenerOwnedBy tells whether the only listener on 127.0.0.1:port belongs to
// pid. A listener on another address that also covers loopback (0.0.0.0) makes
// the answer no.
func listenerOwnedBy(port uint16, pid uint32) bool {
	rows, err := tcpListeners()
	if err != nil {
		return false
	}
	return listenerRowsOwnedBy(rows, port, pid)
}

// listenerRowsOwnedBy is the decision of listenerOwnedBy on a listener table.
func listenerRowsOwnedBy(rows []tcpRow, port uint16, pid uint32) bool {
	found := false
	for _, r := range rows {
		// The port is in network byte order in the low 16 bits.
		p := uint16(r.LocalPort&0xff)<<8 | uint16(r.LocalPort>>8&0xff)
		if p != port {
			continue
		}
		loop := r.LocalAddr == 0x0100007f // 127.0.0.1 in memory order
		if !loop && r.LocalAddr != 0 {
			continue // another interface: does not reach loopback
		}
		if r.PID != pid {
			return false
		}
		found = true
	}
	return found
}

func tcpListeners() ([]tcpRow, error) {
	size := uint32(0)
	for range 4 {
		var buf []byte
		var p unsafe.Pointer
		if size > 0 {
			buf = make([]byte, size)
			p = unsafe.Pointer(&buf[0])
		}
		r, _, _ := procGetExtendedTcpTable.Call(uintptr(p), uintptr(unsafe.Pointer(&size)), 0, afInet, tcpTableOwnerPIDListen, 0)
		switch windows.Errno(r) {
		case 0:
			if len(buf) < 4 {
				return nil, nil
			}
			n := int(*(*uint32)(unsafe.Pointer(&buf[0])))
			if 4+n*int(unsafe.Sizeof(tcpRow{})) > len(buf) {
				return nil, windows.ERROR_INSUFFICIENT_BUFFER
			}
			rows := make([]tcpRow, n)
			for i := range rows {
				rows[i] = *(*tcpRow)(unsafe.Pointer(&buf[4+i*int(unsafe.Sizeof(tcpRow{}))]))
			}
			return rows, nil
		case windows.ERROR_INSUFFICIENT_BUFFER:
			continue // size now holds the needed length
		default:
			return nil, windows.Errno(r)
		}
	}
	return nil, windows.ERROR_INSUFFICIENT_BUFFER
}
