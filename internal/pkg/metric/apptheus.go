package metric

import (
	"net"
	"path/filepath"

	"github.com/apptainer/apptainer/pkg/syfs"
	"github.com/apptainer/apptainer/pkg/util/apptainerconf"
)

const protocol = "unix"

type Apptheus struct {
	net.Conn
}

func New() (*Apptheus, error) {
	socketPath := ""
	configFile := apptainerconf.GetCurrentConfig()
	if configFile != nil {
		socketPath = apptainerconf.GetCurrentConfig().ApptheusSocketPath
	}
	if socketPath == "" {
		socketPath = filepath.Join(syfs.ConfigDir(), "gateway.sock")
	}
	conn, err := net.Dial(protocol, socketPath)
	if err != nil {
		return nil, err
	}

	return &Apptheus{conn}, nil
}
