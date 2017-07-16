package client

import (
	"bytes"
	"errors"
	"net"
	"net/rpc"
	"reflect"
	"time"

	"github.com/gokits/gotools"
	"github.com/gokits/netmonitor/proto"
	"github.com/ugorji/go/codec"
)

var (
	mh codec.MsgpackHandle
)

func init() {
	mh.MapType = reflect.TypeOf(map[string]interface{}(nil))
}

type Client struct {
	needAuth bool
	token    string
	addr     string
	conn     net.Conn
	rpccodec rpc.ClientCodec
	rpccli   *rpc.Client
}

type ClientOption func(*Client)

func WithAuth(token string) ClientOption {
	return func(cli *Client) {
		if token != "" {
			cli.token = token
			cli.needAuth = true
		}
	}
}

func NewClient(addr string, opts ...ClientOption) *Client {
	cli := &Client{
		addr: addr,
	}
	for _, opt := range opts {
		opt(cli)
	}
	return cli
}

func (cli *Client) auth() (err error) {
	enc := codec.NewEncoder(cli.conn, &mh)
	dec := codec.NewDecoder(cli.conn, &mh)
	hello := &proto.Hello{
		Ver:       "v1",
		Challenge: gotools.RandomString(16),
	}
	var hellorsp proto.HelloRsp
	err = enc.Encode(hello)
	if err != nil {
		return
	}
	cli.conn.SetReadDeadline(time.Now().Add(time.Second * 5))
	err = dec.Decode(&hellorsp)
	cli.conn.SetReadDeadline(time.Time{})
	if err != nil {
		return
	}
	if bytes.Compare(hellorsp.CheckSum, proto.AuthV1(cli.token, hello.Challenge)) != 0 {
		err = errors.New("Server Auth Failed")
		return
	}
	hi := &proto.Hi{
		CheckSum: proto.AuthV1(cli.token, hellorsp.Challenge),
	}
	err = enc.Encode(hi)
	if err != nil {
		return
	}
	var hirsp proto.HiRsp
	cli.conn.SetReadDeadline(time.Now().Add(time.Second * 5))
	err = dec.Decode(&hirsp)
	cli.conn.SetReadDeadline(time.Time{})
	if err != nil {
		return
	}
	if !hirsp.Welcome {
		err = errors.New("Server Reject")
		return
	}
	return
}

func (cli *Client) Connect() (err error) {
	cli.conn, err = net.DialTimeout("tcp", cli.addr, time.Second*5)
	if err != nil {
		return
	}
	defer func() {
		if err != nil {
			cli.conn.Close()
			cli.conn = nil
		}
	}()
	if cli.needAuth {
		err = cli.auth()
		if err != nil {
			return
		}
	}
	cli.rpccodec = codec.MsgpackSpecRpc.ClientCodec(cli.conn, &mh)
	cli.rpccli = rpc.NewClientWithCodec(cli.rpccodec)
	return
}

func (cli *Client) Echo(ping *proto.Echo, pong *proto.Echo) (err error) {
	err = cli.rpccli.Call("Echo.Echo", ping, pong)
	return
}

func (cli *Client) Close() (err error) {
	if cli.rpccli != nil {
		err = cli.rpccli.Close()
		cli.rpccli = nil
	}
	return
}
