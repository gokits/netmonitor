package server

import (
	"bytes"
	"errors"
	"net"
	"net/rpc"
	"reflect"
	"time"

	"github.com/gokits/gotools"
	"github.com/gokits/netmonitor/proto"
	"github.com/sirupsen/logrus"
	"github.com/ugorji/go/codec"
)

var (
	mh codec.MsgpackHandle
)

func init() {
	mh.MapType = reflect.TypeOf(map[string]interface{}(nil))
	rpc.Register(new(Echo))
}

type Echo int

func (e *Echo) Echo(args *proto.Echo, reply *proto.Echo) (err error) {
	*reply = *args
	return
}

type Server struct {
	needAuth bool
	token    string
	listen   string
	listener net.Listener
	log      *logrus.Entry
	done     chan struct{}
}

type ServerOption func(*Server)

func WithAuth(token string) ServerOption {
	return func(s *Server) {
		if token != "" {
			s.needAuth = true
			s.token = token
		}
	}
}

func NewServer(listen string, log *logrus.Entry, opts ...ServerOption) *Server {
	s := &Server{
		listen: listen,
		log:    log,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Server) auth(conn net.Conn) (err error) {
	enc := codec.NewEncoder(conn, &mh)
	dec := codec.NewDecoder(conn, &mh)
	var hello proto.Hello
	conn.SetReadDeadline(time.Now().Add(time.Second * 5))
	err = dec.Decode(&hello)
	conn.SetReadDeadline(time.Time{})
	if err != nil {
		return
	}
	hellorsp := &proto.HelloRsp{
		CheckSum:  proto.AuthV1(s.token, hello.Challenge),
		Challenge: gotools.RandomString(16),
	}
	err = enc.Encode(hellorsp)
	if err != nil {
		return
	}
	var hi proto.Hi
	conn.SetReadDeadline(time.Now().Add(time.Second * 5))
	err = dec.Decode(&hi)
	conn.SetReadDeadline(time.Time{})
	if err != nil {
		return
	}
	var hirsp proto.HiRsp
	hirsp.Welcome = true
	if bytes.Compare(hi.CheckSum, proto.AuthV1(s.token, hellorsp.Challenge)) != 0 {
		err = errors.New("Client Auth Failed")
		hirsp.Welcome = false
	}
	err = enc.Encode(&hirsp)
	if err != nil {
		return
	}
	return
}

func (s *Server) StartAndServe() (err error) {
	s.listener, err = net.Listen("tcp", s.listen)
	if err != nil {
		return
	}
	for {
		select {
		case <-s.done:
			s.listener.Close()
			return
		default:
			conn, err1 := s.listener.Accept()
			if err1 != nil {
				s.log.WithError(err1).Warn("accept failed")
				continue
			}
			go func() {
				if s.needAuth {
					if err1 = s.auth(conn); err1 != nil {
						s.log.WithField("remote", conn.RemoteAddr().String()).WithError(err1).Warn("auth failed")
						conn.Close()
						return
					}
				}
				rpcCodec := codec.MsgpackSpecRpc.ServerCodec(conn, &mh)
				rpc.ServeCodec(rpcCodec)
			}()
		}
	}
}

func (s *Server) Close() {
	close(s.done)
}
