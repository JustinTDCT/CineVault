package dlna

import (
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	ssdpAddr     = "239.255.255.250:1900"
	ssdpMaxAge   = 1800
	serverHeader = "CineVault/1.0 UPnP/1.0 DLNADOC/1.50"
)

// SSDPServer handles UPnP discovery via multicast.
type SSDPServer struct {
	udn         string
	serverAddr  string // e.g. "http://192.168.1.100:8080"
	mu          sync.Mutex
	running     bool
	stopCh      chan struct{}
}

// NewSSDPServer creates a new SSDP discovery server.
func NewSSDPServer(serverAddr string) *SSDPServer {
	return &SSDPServer{
		udn:        "uuid:" + uuid.New().String(),
		serverAddr: strings.TrimRight(serverAddr, "/"),
		stopCh:     make(chan struct{}),
	}
}

// UDN returns the unique device name.
func (s *SSDPServer) UDN() string {
	return s.udn
}

// Start begins listening for SSDP discovery requests and sends periodic advertisements.
func (s *SSDPServer) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = true
	s.mu.Unlock()

	addr, err := net.ResolveUDPAddr("udp4", ssdpAddr)
	if err != nil {
		return fmt.Errorf("resolve SSDP addr: %w", err)
	}

	conn, err := net.ListenMulticastUDP("udp4", nil, addr)
	if err != nil {
		return fmt.Errorf("listen multicast: %w", err)
	}
	conn.SetReadBuffer(8192)

	log.Printf("[DLNA] SSDP server started on %s (UDN: %s)", ssdpAddr, s.udn)

	// Listener goroutine
	go func() {
		defer conn.Close()
		buf := make([]byte, 4096)
		for {
			select {
			case <-s.stopCh:
				return
			default:
			}
			conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			n, remoteAddr, err := conn.ReadFromUDP(buf)
			if err != nil {
				continue
			}
			msg := string(buf[:n])
			if strings.Contains(msg, "M-SEARCH") {
				go s.handleMSearch(msg, remoteAddr)
			}
		}
	}()

	// Periodic NOTIFY advertisements
	go func() {
		// Initial announce
		s.sendNotify("ssdp:alive")
		ticker := time.NewTicker(time.Duration(ssdpMaxAge/2) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.sendNotify("ssdp:alive")
			case <-s.stopCh:
				s.sendNotify("ssdp:byebye")
				return
			}
		}
	}()

	return nil
}

// Stop halts the SSDP server.
func (s *SSDPServer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		close(s.stopCh)
		s.running = false
	}
}

func (s *SSDPServer) handleMSearch(msg string, remoteAddr *net.UDPAddr) {
	// Respond to root device, MediaServer, and ContentDirectory searches
	st := extractHeader(msg, "ST")
	if st == "" {
		return
	}

	targets := []string{
		"ssdp:all",
		"upnp:rootdevice",
		"urn:schemas-upnp-org:device:MediaServer:1",
		"urn:schemas-upnp-org:service:ContentDirectory:1",
		"urn:schemas-upnp-org:service:ConnectionManager:1",
	}

	shouldRespond := false
	for _, t := range targets {
		if st == t {
			shouldRespond = true
			break
		}
	}
	if !shouldRespond {
		return
	}

	response := fmt.Sprintf("HTTP/1.1 200 OK\r\n"+
		"CACHE-CONTROL: max-age=%d\r\n"+
		"ST: %s\r\n"+
		"USN: %s::%s\r\n"+
		"LOCATION: %s/dlna/description.xml\r\n"+
		"SERVER: %s\r\n"+
		"EXT:\r\n"+
		"\r\n", ssdpMaxAge, st, s.udn, st, s.serverAddr, serverHeader)

	conn, err := net.DialUDP("udp4", nil, remoteAddr)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.Write([]byte(response))
}

func (s *SSDPServer) sendNotify(nts string) {
	addr, err := net.ResolveUDPAddr("udp4", ssdpAddr)
	if err != nil {
		return
	}
	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		return
	}
	defer conn.Close()

	for _, nt := range []string{
		"upnp:rootdevice",
		s.udn,
		"urn:schemas-upnp-org:device:MediaServer:1",
		"urn:schemas-upnp-org:service:ContentDirectory:1",
		"urn:schemas-upnp-org:service:ConnectionManager:1",
	} {
		usn := s.udn + "::" + nt
		if nt == s.udn {
			usn = s.udn
		}
		msg := fmt.Sprintf("NOTIFY * HTTP/1.1\r\n"+
			"HOST: %s\r\n"+
			"CACHE-CONTROL: max-age=%d\r\n"+
			"NT: %s\r\n"+
			"NTS: %s\r\n"+
			"USN: %s\r\n"+
			"LOCATION: %s/dlna/description.xml\r\n"+
			"SERVER: %s\r\n"+
			"\r\n", ssdpAddr, ssdpMaxAge, nt, nts, usn, s.serverAddr, serverHeader)
		conn.Write([]byte(msg))
		time.Sleep(50 * time.Millisecond) // Small delay between notifications
	}
}

func extractHeader(msg, header string) string {
	for _, line := range strings.Split(msg, "\r\n") {
		if strings.HasPrefix(strings.ToUpper(line), strings.ToUpper(header)+":") {
			return strings.TrimSpace(line[len(header)+1:])
		}
	}
	return ""
}
