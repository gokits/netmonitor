package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/YueHonghui/rfw"
	"github.com/gokits/netmonitor/client"
	"github.com/gokits/netmonitor/proto"
	"github.com/gokits/netmonitor/server"
	"github.com/sirupsen/logrus"
)

func init() {
	// Log as JSON instead of the default ASCII formatter.
	logrus.SetFormatter(&logrus.JSONFormatter{})

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	logrus.SetOutput(os.Stdout)

	// Only log the warning severity or above.
	logrus.SetLevel(logrus.InfoLevel)
}

func serverClient(cli *client.Client, interval time.Duration, logentry *logrus.Entry) {
	e := &proto.Echo{
		Now: time.Now().UnixNano(),
	}
	var r proto.Echo
	for {
		if err := cli.Connect(); err != nil {
			logentry.WithError(err).Error("Connect failed")
			time.Sleep(30 * time.Second)
			continue
		}
		tick := time.NewTicker(interval)
		for _ = range tick.C {
			e.Now = time.Now().UnixNano()
			if err := cli.Echo(e, &r); err != nil {
				logentry.WithError(err).Error("Echo failed")
				break
			} else {
				now := time.Now().UnixNano()
				if now >= e.Now {
					logentry.WithField("latency", (now-e.Now)/int64(time.Millisecond)).Info("success response")
				} else {
					logentry.Warn("timestamp reversed, ignore")
				}
			}
		}
		cli.Close()
		tick.Stop()
	}
}

func main() {
	var mode string
	var listen string
	var remotes string
	var servertoken string
	var clienttoken string
	var logpath string
	var interval time.Duration
	flag.StringVar(&mode, "mode", "server", "client|server|both")
	flag.StringVar(&listen, "listen", ":5353", "echo server to listen at")
	flag.DurationVar(&interval, "interval", time.Second*10, "interval for client to ping server")
	flag.StringVar(&remotes, "remotes", "", "remote addrs of echo server, '1.1.1.1:5353,2.2.2.2:5353' for example")
	flag.StringVar(&servertoken, "server_token", "", "token used by auth for server, empty means not auth incoming connections")
	flag.StringVar(&clienttoken, "client_token", "", "token used by auth for client")
	flag.StringVar(&logpath, "logpath", "./netmonitor", "log base path")
	flag.Parse()
	logwriter, err := rfw.New(logpath)
	if err != nil {
		fmt.Printf("open log path failed. err=%v\n", err)
		os.Exit(-1)
		return
	}
	logrus.SetOutput(logwriter)
	if mode != "client" && mode != "server" && mode != "both" {
		fmt.Println("mode invalid")
		os.Exit(-1)
		return
	}
	if mode == "server" || mode == "both" {
		if listen == "" {
			fmt.Println("listen is needed in server or both mode")
			os.Exit(-1)
			return
		}
		s := server.NewServer(listen, logrus.WithField("module", "server"), server.WithAuth(servertoken))
		go s.StartAndServe()
	}
	remote_addrs := strings.Split(remotes, ",")
	if mode == "client" || mode == "both" {
		if len(remote_addrs) == 0 {
			fmt.Println("remote addrs of echo server is needed in client or both mode")
			os.Exit(-1)
			return
		}
		for _, ra := range remote_addrs {
			cli := client.NewClient(ra, client.WithAuth(clienttoken))
			go serverClient(cli, interval, logrus.WithField("module", "client").WithField("remote", ra))
		}
	}
	ch := make(chan int)
	<-ch
}
