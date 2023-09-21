package metric

import (
	"net"
	"path/filepath"

	"github.com/apptainer/apptainer/pkg/syfs"
	"github.com/apptainer/apptainer/pkg/util/apptainerconf"
)

const protocol = "unix"

type Pushgateway struct {
	net.Conn
}

func New() (*Pushgateway, error) {
	var socketPath string
	socketPath = apptainerconf.GetCurrentConfig().PushgatewaySocketPath
	if socketPath == "" {
		socketPath = filepath.Join(syfs.ConfigDir(), "pushgateway.socket")
	}
	conn, err := net.Dial(protocol, socketPath)
	if err != nil {
		return nil, err
	}

	return &Pushgateway{conn}, nil
}
