package etcd

import (
	"context"
	"fmt"
	"log"

	"github.com/hl540/grpc-gateway/src/naming/registrar"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/naming/endpoints"
)

type etcdRegistrar struct {
	ctx         context.Context
	cancel      context.CancelFunc
	etcd        *clientv3.Client // etcd客户端
	leaseId     clientv3.LeaseID // 续租id
	keepAliveCh <-chan *clientv3.LeaseKeepAliveResponse
	serviceName string
	serviceAddr string
	ttl         int64
	em          endpoints.Manager
}

func NewRegistrar(ctx context.Context, client *clientv3.Client, serviceName, serviceAddr string, ttl int64) registrar.Registrar {
	ctx, cancel := context.WithCancel(ctx)
	return &etcdRegistrar{
		ctx:         ctx,
		cancel:      cancel,
		etcd:        client,
		serviceName: serviceName,
		serviceAddr: serviceAddr,
		ttl:         ttl,
	}
}

func (r *etcdRegistrar) register() error {
	// 创建lease
	lease, err := r.etcd.Grant(r.ctx, 5)
	if err != nil {
		return err
	}
	r.leaseId = lease.ID

	// 创建 Endpoint Manager
	r.em, err = endpoints.NewManager(r.etcd, r.serviceName)
	if err != nil {
		return err
	}

	// 附带lease注册到etcd
	err = r.em.AddEndpoint(
		context.Background(),
		fmt.Sprintf("%s/%s", r.serviceName, r.serviceAddr),
		endpoints.Endpoint{
			Addr:     r.serviceAddr,
			Metadata: nil,
		},
		clientv3.WithLease(r.leaseId),
	)
	if err != nil {
		return err
	}

	// 续租
	r.keepAliveCh, err = r.etcd.KeepAlive(r.ctx, r.leaseId)
	return err
}

// Register 服务注册
func (r *etcdRegistrar) Register() error {
	// 注册服务
	err := r.register()
	if err != nil {
		return err
	}

	// 监听续租状态
	go r.watch()
	return nil
}

// 监听续租
func (r *etcdRegistrar) watch() {
	for {
		select {
		case c := <-r.keepAliveCh:
			if c == nil {
				// 重新注册
				if err := r.register(); err != nil {
					log.Printf("重新注册失败,err:%s\n", err.Error())
				}
				log.Println("重新注册服务")
			} else {
				log.Printf("续租状态,TTL:%d", c.TTL)
			}
		case <-r.ctx.Done():
			log.Println("结束续租")
			return
		}
	}
}

// UnRegister 注销服务
func (r *etcdRegistrar) UnRegister() error {
	// 撤销续租
	_, err := r.etcd.Revoke(r.ctx, r.leaseId)
	if err != nil {
		return err
	}
	r.cancel()
	return nil
}
