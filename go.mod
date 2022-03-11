module github.com/apptainer/apptainer

go 1.16

require (
	github.com/Netflix/go-expect v0.0.0-20220104043353-73e0943537d2
	github.com/ProtonMail/go-crypto v0.0.0-20220113124808-70ae35bab23f
	github.com/adigunhammedolalekan/registry-auth v0.0.0-20200730122110-8cde180a3a60
	github.com/apex/log v1.9.0
	github.com/apptainer/container-key-client v0.7.2
	github.com/apptainer/container-library-client v1.2.2
	github.com/apptainer/sif/v2 v2.3.2
	github.com/blang/semver/v4 v4.0.0
	github.com/buger/jsonparser v1.1.1
	github.com/cenkalti/backoff/v4 v4.1.2
	github.com/containerd/cgroups v1.0.3
	github.com/containerd/containerd v1.6.0
	github.com/containernetworking/cni v1.0.1
	github.com/containernetworking/plugins v1.1.0
	github.com/containers/image/v5 v5.19.1
	github.com/creack/pty v1.1.17
	github.com/cyphar/filepath-securejoin v0.2.3
	github.com/docker/docker v20.10.12+incompatible
	github.com/fatih/color v1.13.0
	github.com/go-log/log v0.2.0
	github.com/google/uuid v1.3.0
	github.com/moby/sys/mount v0.3.0 // indirect
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.3-0.20211202193544-a5463b7f9c84
	github.com/opencontainers/runtime-spec v1.0.3-0.20210326190908-1c3f411f0417
	github.com/opencontainers/runtime-tools v0.9.1-0.20210326182921-59cdde06764b
	github.com/opencontainers/selinux v1.10.0
	github.com/opencontainers/umoci v0.4.7
	github.com/pelletier/go-toml v1.9.4
	github.com/pkg/errors v0.9.1
	github.com/seccomp/containers-golang v0.6.0
	github.com/seccomp/libseccomp-golang v0.9.2-0.20210429002308-3879420cc921
	github.com/spf13/cobra v1.4.0
	github.com/spf13/pflag v1.0.5
	github.com/sylabs/json-resp v0.8.0
	github.com/urfave/cli v1.22.5 // indirect
	github.com/vbauerster/mpb/v7 v7.4.1
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/yvasiyarov/go-metrics v0.0.0-20150112132944-c25f46c4b940 // indirect
	github.com/yvasiyarov/gorelic v0.0.6 // indirect
	github.com/yvasiyarov/newrelic_platform_go v0.0.0-20160601141957-9c099fbc30e9 // indirect
	golang.org/x/sys v0.0.0-20220209214540-3681064d5158
	golang.org/x/term v0.0.0-20210916214954-140adaaadfaf
	gopkg.in/yaml.v2 v2.4.0
	gotest.tools/v3 v3.1.0
	mvdan.cc/sh/v3 v3.4.3-0.20220202175809-113ed667a8a7
	oras.land/oras-go v1.1.0
)

// Temporarily force an image-spec that has the main branch commits not in 1.0.2 which is being brought in by oras-go
// These commits are needed by containers/image/v5 and the replace is necessary given how image-spec v1.0.2 has been
// tagged / rebased.
replace github.com/opencontainers/image-spec => github.com/opencontainers/image-spec v1.0.2-0.20211117181255-693428a734f5
