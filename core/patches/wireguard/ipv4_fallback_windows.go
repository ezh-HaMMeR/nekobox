//go:build windows

package conn

import (
	"errors"
	"syscall"

	"github.com/sagernet/sing/common/control"
	"golang.org/x/sys/windows"
)

// Windows rejects IPV6_UNICAST_IF with WSAEINVAL when IPv6 is disabled on
// the selected adapter. Keep interface protection on IPv4; never retry a
// protected socket without its control function.
func ipv4CompatibleControl(fn control.Func) control.Func {
	if fn == nil {
		return nil
	}
	return func(network, address string, raw syscall.RawConn) error {
		err := fn(network, address, raw)
		if network == "udp6" && errors.Is(err, windows.WSAEINVAL) {
			return syscall.EAFNOSUPPORT
		}
		return err
	}
}

type ipv4FallbackBind struct {
	Bind
	control control.Func
}

func newIPv4FallbackBind(fn control.Func) Bind {
	fn = ipv4CompatibleControl(fn)
	return &ipv4FallbackBind{Bind: NewWinRingBind(fn), control: fn}
}

func (b *ipv4FallbackBind) Open(port uint16) ([]ReceiveFunc, uint16, error) {
	fns, actualPort, err := b.Bind.Open(port)
	if !errors.Is(err, syscall.EAFNOSUPPORT) && !errors.Is(err, windows.WSAEAFNOSUPPORT) {
		return fns, actualPort, err
	}
	// WinRingBind requires both address families. StdNetBind can retain its
	// IPv4 listener when the IPv6 family is unavailable, and reports the
	// actual ephemeral port. WinRingBind cleans up a failed Open itself.
	b.Bind = NewStdNetBind(b.control)
	return b.Bind.Open(port)
}
