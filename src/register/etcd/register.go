package etcd

import (
	"context"
	"fmt"
	"log"

	"github.com/hl540/grpc-gateway/src/register"
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

func NewEtcdRegister(ctx context.Context, client *clientv3.Client, serviceName, serviceAddr string, ttl int64) register.Registrar {
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

// Register 服务注册
func (r *etcdRegistrar) Register() error {
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
		clientv3.WithLease(lease.ID),
	)
	// 续租
	r.keepAliveCh, err = r.etcd.KeepAlive(r.ctx, r.leaseId)
	if err != nil {
		return err
	}
	go r.watch()
	return nil
}

// 监听续租
func (r *etcdRegistrar) watch() {
	for {
		select {
		case c, ok := <-r.keepAliveCh:
			if !ok {
				log.Println("续租状态,TTL:0")
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
