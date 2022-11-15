package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/hl540/grpc-gateway/proto/helloworld"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/naming/resolver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/balancer/roundrobin"
	"google.golang.org/grpc/credentials/insecure"
)

var grpcConn *grpc.ClientConn

func init() {
	// 服务名称解析器
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatalf("etcd客户端创建失败, %s\n", err.Error())
	}
	etcdResolver, err := resolver.NewBuilder(client)
	if err != nil {
		log.Fatalf("名称解析器创建失败, %s\n", err.Error())
	}

	// 创建客户端
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithResolvers(etcdResolver),
		grpc.WithDefaultServiceConfig(fmt.Sprintf(`{"LoadBalancingPolicy": "%s"}`, roundrobin.Name)),
	}
	grpcConn, err = grpc.Dial("etcd:///app/grpc/helloworld", opts...)
	if err != nil {
		log.Fatalf("gRPC服务连接失败, %s\n", err.Error())
	}
}

func main() {
	// 注册gateway服务
	mux := runtime.NewServeMux()
	err := helloworld.RegisterGreeterHandler(
		context.Background(),
		mux,
		grpcConn,
	)
	if err != nil {
		log.Fatalln("Failed to register gateway:", err)
	}
	swServer := http.Server{
		Addr:    ":8090",
		Handler: mux,
	}
	log.Println("Serving gRPC-Gateway on http://0.0.0.0:8090")
	log.Fatalln(swServer.ListenAndServe())
}
