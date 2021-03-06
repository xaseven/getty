/******************************************************
# DESC       : getty server
# MAINTAINER : Alex Stocks
# LICENCE    : Apache License 2.0
# EMAIL      : alexstocks@foxmail.com
# MOD        : 2016-08-17 11:21
# FILE       : server.go
******************************************************/

package getty

import (
	"errors"
	"net"
	"sync"
	"time"
)

import (
	log "github.com/AlexStocks/log4go"
)

type SessionCallback func(*Session)

type Server struct {
	// net
	host     string
	port     int
	listener net.Listener

	sync.Once
	done chan struct{}
	wg   sync.WaitGroup
}

func NewServer() *Server {
	return &Server{done: make(chan struct{})}
}

func (this *Server) stop() {
	select {
	case <-this.done:
		return
	default:
		close(this.done)
		this.Once.Do(func() {
			// 把listener.Close放在这里，既能防止多次关闭调用，
			// 又能及时让Server因accept返回错误而从RunEventloop退出
			this.listener.Close()
		})
	}
}

func (this *Server) IsClosed() bool {
	select {
	case <-this.done:
		return true
	default:
		return false
	}

	return false
}

func (this *Server) Bind(network string, host string, port int) error {
	if port <= 0 {
		return errors.New("port<=0 illegal")
	}

	this.host = host
	this.port = port
	listener, err := net.Listen(network, HostAddress(host, port))
	if err != nil {
		return err
	}
	this.listener = listener

	return nil
}

func (this *Server) RunEventloop(newSessionCallback func(*Session)) {
	this.wg.Add(1)
	go func() {
		defer this.wg.Done()
		var (
			err    error
			client *Session
			delay  time.Duration
		)
		for {
			if this.IsClosed() {
				log.Warn("Server{%s:%d} stop acceptting client connect request.", this.host, this.port)
				return
			}
			if delay != 0 {
				time.Sleep(delay)
			}
			client, err = this.Accept(newSessionCallback)
			if err != nil {
				if ne, ok := err.(net.Error); ok && ne.Temporary() {
					if delay == 0 {
						delay = 5 * time.Millisecond
					} else {
						delay *= 2
					}
					if max := 1 * time.Second; delay > max {
						delay = max
					}
					continue
				}
				log.Info("Server{%s:%d}.Accept() = err {%+v}", this.host, this.port, err)
				continue
			}
			delay = 0
			client.RunEventloop()
		}
	}()
}

func (this *Server) Listener() net.Listener {
	return this.listener
}

func (this *Server) Accept(newSessionCallback func(*Session)) (*Session, error) {
	conn, err := this.listener.Accept()
	if err != nil {
		return nil, err
	}

	session := NewSession(conn)
	newSessionCallback(session)

	return session, nil
}

func (this *Server) Close() {
	this.stop()
	this.wg.Wait()
}
