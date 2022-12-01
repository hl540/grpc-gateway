package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hl540/grpc-gateway/proto/helloworld"
	"github.com/hl540/grpc-gateway/src/naming/registrar/etcd"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
)

type Server struct {
	helloworld.UnimplementedGreeterServer
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) SayHello(ctx context.Context, in *helloworld.HelloRequest) (*helloworld.HelloReply, error) {
	log.Println(in.Name)
	return &helloworld.HelloReply{
		Message: fmt.Sprintf("%s world of %s", in.Name, *addr),
	}, nil
}

var etcdClient *clientv3.Client
var name = flag.String("name", "", "服务名称")
var addr = flag.String("addr", "", "服务地址")

func init() {
	// 服务名称解析器
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatalf("etcd客户端创建失败, %s\n", err.Error())
	}
	etcdClient = client
}

func main() {
	flag.Parse()
	if *name == "" || *addr == "" {
		log.Fatalln("参数错误")
	}
	lis, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalln("Failed to listen:", err)
	}

	s := grpc.NewServer()
	helloworld.RegisterGreeterServer(s, &Server{})
	log.Printf("Serving gRPC on %s\n", *addr)
	go func() {
		log.Fatal(s.Serve(lis))
	}()

	registrar := etcd.NewRegistrar(context.Background(), etcdClient, *name, *addr, 5)
	go func() {
		if err := registrar.Register(); err != nil {
			log.Fatalf("服务注册失败, %s\n", err.Error())
		}
	}()

	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-quit
	log.Println("优雅退出")

	registrar.UnRegister()
	s.Stop()
}
