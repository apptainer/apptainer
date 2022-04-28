module github.com/apptainer/apptainer

go 1.16

require (
	github.com/Netflix/go-expect v0.0.0-20220104043353-73e0943537d2
	github.com/ProtonMail/go-crypto v0.0.0-20220113124808-70ae35bab23f
	github.com/adigunhammedolalekan/registry-auth v0.0.0-20200730122110-8cde180a3a60
	github.com/apex/log v1.9.0
	github.com/apptainer/container-key-client v0.7.3
	github.com/apptainer/container-library-client v1.3.1
	github.com/apptainer/sif/v2 v2.4.2
	github.com/blang/semver/v4 v4.0.0
	github.com/buger/jsonparser v1.1.1
	github.com/cenkalti/backoff/v4 v4.1.3
	github.com/containerd/containerd v1.6.3
	github.com/containernetworking/cni v1.1.0
	github.com/containernetworking/plugins v1.1.1
	github.com/containers/image/v5 v5.21.0
	github.com/creack/pty v1.1.18
	github.com/cyphar/filepath-securejoin v0.2.3
	github.com/docker/docker v20.10.14+incompatible
	github.com/fatih/color v1.13.0
	github.com/fsnotify/fsnotify v1.5.1 // indirect
	github.com/go-log/log v0.2.0
	github.com/google/uuid v1.3.0
	github.com/mattn/go-colorable v0.1.12 // indirect
	github.com/moby/sys/mount v0.3.0 // indirect
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.3-0.20211202193544-a5463b7f9c84
	github.com/opencontainers/runc v1.1.1
	github.com/opencontainers/runtime-spec v1.0.3-0.20210326190908-1c3f411f0417
	github.com/opencontainers/runtime-tools v0.9.1-0.20210326182921-59cdde06764b
	github.com/opencontainers/selinux v1.10.1
	github.com/opencontainers/umoci v0.4.7
	github.com/pelletier/go-toml v1.9.5
	github.com/pkg/errors v0.9.1
	github.com/seccomp/containers-golang v0.6.0
	github.com/seccomp/libseccomp-golang v0.9.2-0.20210429002308-3879420cc921
	github.com/spf13/cobra v1.4.0
	github.com/spf13/pflag v1.0.5
	github.com/sylabs/json-resp v0.8.1
	github.com/urfave/cli v1.22.5 // indirect
	github.com/vbauerster/mpb/v7 v7.4.1
	github.com/yvasiyarov/go-metrics v0.0.0-20150112132944-c25f46c4b940 // indirect
	github.com/yvasiyarov/gorelic v0.0.6 // indirect
	github.com/yvasiyarov/newrelic_platform_go v0.0.0-20160601141957-9c099fbc30e9 // indirect
	golang.org/x/sys v0.0.0-20220227234510-4e6760a101f9
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211
	gopkg.in/yaml.v2 v2.4.0
	gotest.tools/v3 v3.2.0
	mvdan.cc/sh/v3 v3.4.3
	oras.land/oras-go v1.1.1
)

// Temporarily force an image-spec that has the main branch commits not in 1.0.2 which is being brought in by oras-go
// These commits are needed by containers/image/v5 and the replace is necessary given how image-spec v1.0.2 has been
// tagged / rebased.
replace github.com/opencontainers/image-spec => github.com/opencontainers/image-spec v1.0.2-0.20211117181255-693428a734f5
