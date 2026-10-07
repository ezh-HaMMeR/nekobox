//go:build windows

package conn

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"syscall"
	"testing"
	"time"

	"github.com/sagernet/sing/common/control"
	"golang.org/x/sys/windows"
)

func TestIPv4OnlyAdapterBind(t *testing.T) {
	controls := 0
	b := NewDefaultBind(func(network, address string, raw syscall.RawConn) error {
		if network == "udp6" {
			return fmt.Errorf("IPv6 disabled: %w", windows.WSAEINVAL)
		}
		controls++
		return nil
	})
	defer b.Close()
	fns, port, err := b.Open(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(fns) != 1 || port == 0 || controls == 0 {
		t.Fatalf("receivers=%d port=%d protected IPv4 sockets=%d", len(fns), port, controls)
	}
	if _, _, err := b.Open(0); !errors.Is(err, ErrBindAlreadyOpen) {
		t.Fatalf("second Open: %v", err)
	}
	// Prove that fallback creates a usable socket, rather than only hiding
	// the startup error. Exchange a datagram with a local UDP receiver.
	peer, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	peer.SetReadDeadline(time.Now().Add(3 * time.Second))
	ep, err := b.ParseEndpoint(peer.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Send([][]byte{[]byte("awg-ipv4")}, ep, 0); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 128)
	n, sender, err := peer.ReadFromUDPAddrPort(buf)
	if err != nil || string(buf[:n]) != "awg-ipv4" || sender.Port() != port {
		t.Fatalf("receive: %q sender=%v err=%v", buf[:n], sender, err)
	}
	if _, err := peer.WriteToUDPAddrPort([]byte("reply"), netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), port)); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		packets := [][]byte{make([]byte, 128)}
		sizes := make([]int, 1)
		endpoints := make([]Endpoint, 1)
		n, err := fns[0](packets, sizes, endpoints)
		if err == nil && (n != 1 || string(packets[0][:sizes[0]]) != "reply") {
			err = fmt.Errorf("unexpected reply")
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("receive timeout")
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := b.Open(port); err != nil {
		t.Fatalf("reopen: %v", err)
	}
}

func TestIPv4ProtectionErrorIsNotSuppressed(t *testing.T) {
	b := NewDefaultBind(func(network, address string, raw syscall.RawConn) error {
		return windows.WSAEINVAL
	})
	defer b.Close()
	if _, _, err := b.Open(0); !errors.Is(err, windows.WSAEINVAL) {
		t.Fatalf("IPv4 control error must be preserved: %v", err)
	}
}

func TestIPv6OtherErrorsAreNotSuppressed(t *testing.T) {
	fn := ipv4CompatibleControl(func(string, string, syscall.RawConn) error { return windows.WSAEACCES })
	if err := fn("udp6", "[::]:0", nil); !errors.Is(err, windows.WSAEACCES) {
		t.Fatalf("IPv6 access error must be preserved: %v", err)
	}
}

func TestDualStackBindStillWorks(t *testing.T) {
	b := NewDefaultBind(nil)
	defer b.Close()
	fns, port, err := b.Open(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(fns) != 2 || port == 0 {
		t.Fatalf("receivers=%d port=%d", len(fns), port)
	}
}

func TestIPv4OnlyPhysicalAdapter(t *testing.T) {
	interfaces, err := net.Interfaces()
	if err != nil {
		t.Fatal(err)
	}
	for _, iif := range interfaces {
		if iif.Flags&net.FlagUp == 0 || iif.Flags&net.FlagLoopback != 0 {
			continue
		}
		fn := control.BindToInterface(nil, iif.Name, iif.Index)
		probe, err := net.ListenUDP("udp6", &net.UDPAddr{})
		if err != nil {
			continue
		}
		raw, err := probe.SyscallConn()
		if err != nil {
			probe.Close()
			continue
		}
		err = fn("udp6", "[::]:0", raw)
		probe.Close()
		if !errors.Is(err, windows.WSAEINVAL) {
			continue
		}
		b := NewDefaultBind(fn)
		fns, port, err := b.Open(0)
		b.Close()
		if err != nil {
			t.Fatalf("adapter %s: %v", iif.Name, err)
		}
		if len(fns) != 1 || port == 0 {
			t.Fatalf("adapter %s: receivers=%d port=%d", iif.Name, len(fns), port)
		}
		t.Logf("adapter %s, index %d: IPv4 startup succeeds with IPv6 disabled", iif.Name, iif.Index)
		return
	}
	t.Skip("no IPv4-only adapter available")
}
