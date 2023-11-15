package app

import (
	"net"
	"os"
	"sync"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/google/uuid"
	"google.golang.org/grpc"
)

type Driver struct {
	sync.Mutex
	addr   string
	name   string
	nodeId string
	server *grpc.Server
}

func NewDriver() *Driver {
	endpoint := os.Getenv("CSI_ENDPOINT")
	if endpoint == "" {
		endpoint = "/csi/csi.sock"
	}
	return &Driver{
		addr:   endpoint,
		name:   "csi.test.k8s.io",
		nodeId: uuid.NewString(),
	}
}

func (d *Driver) Run() {
	_ = os.Remove(d.addr)

	listener, err := net.Listen("unix", d.addr)
	if err != nil {
		panic(err)
	}

	d.server = grpc.NewServer()
	csi.RegisterIdentityServer(d.server, d)
	csi.RegisterControllerServer(d.server, d)
	csi.RegisterNodeServer(d.server, d)

	_ = d.server.Serve(listener)
}
