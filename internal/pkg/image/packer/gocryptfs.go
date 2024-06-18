package packer

import (
	"bytes"
	// #nosec G501
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/apptainer/apptainer/internal/pkg/image/driver"
	"github.com/apptainer/apptainer/internal/pkg/util/bin"
	"github.com/apptainer/apptainer/pkg/image"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/apptainer/pkg/util/apptainerconf"
	"github.com/apptainer/apptainer/pkg/util/cryptkey"
)

type Gocryptfs struct {
	*Squashfs
	gocryptfsPath string
	driver        image.Driver
	Pass          string
	keyInfo       *cryptkey.KeyInfo
}

type cryptInfo struct {
	cipherDir, plainDir, pass, confPath string
}

func newCryptInfo() *cryptInfo {
	return &cryptInfo{
		cipherDir: "",
		plainDir:  "",
		pass:      "",
		confPath:  "",
	}
}

func NewGocryptfs(keyInfo *cryptkey.KeyInfo) *Gocryptfs {
	g := &Gocryptfs{
		Squashfs: NewSquashfs(),
	}
	appfile := apptainerconf.GetCurrentConfig()
	driver.InitImageDrivers(true, true, appfile, image.GocryptFeature)
	g.gocryptfsPath, _ = bin.FindBin("gocryptfs")
	g.driver = image.GetDriver(driver.DriverName)
	g.keyInfo = keyInfo
	return g
}

func (g *Gocryptfs) HasGocryptfs() bool {
	return g.driver.Features()&image.GocryptFeature != 0
}

func (g *Gocryptfs) init(tmpDir string) (cryptInfo *cryptInfo, err error) {
	if !g.HasGocryptfs() {
		return nil, fmt.Errorf("imagedriver is not initialized")
	}

	cryptInfo = newCryptInfo()
	parentDir, err := os.MkdirTemp(tmpDir, "gocryptfs-")
	if err != nil {
		return nil, err
	}
	cipherDir := filepath.Join(parentDir, "cipher")
	plainDir := filepath.Join(parentDir, "plain")

	err = os.Mkdir(cipherDir, 0o700)
	if err != nil {
		return nil, err
	}
	cryptInfo.cipherDir = cipherDir
	err = os.Mkdir(plainDir, 0o700)
	if err != nil {
		return nil, err
	}
	cryptInfo.plainDir = plainDir

	buf := make([]byte, 32)
	_, err = rand.Read(buf)
	if err != nil {
		return nil, err
	}

	switch g.keyInfo.Format {
	case cryptkey.PEM, cryptkey.ENV:
		// #nosec G401
		hash := md5.Sum(buf)
		cryptInfo.pass = hex.EncodeToString(hash[:])
	case cryptkey.Passphrase:
		cryptInfo.pass = g.keyInfo.Material
	default:
		err = errors.New("cryptkey type is unknown")
		return nil, err
	}
	cryptInfo.confPath = filepath.Join(cipherDir, "gocryptfs.conf")

	sylog.Debugf("Start initializing gocryptfs, cipher: %s, plain: %s", cipherDir, plainDir)
	var stderr bytes.Buffer
	pass := fmt.Sprintf("%s\n", cryptInfo.pass)
	cmd := exec.Command(g.gocryptfsPath, "-init", "-deterministic-names", "-plaintextnames", cipherDir)
	cmd.Stdin = strings.NewReader(pass + pass)
	cmd.Stderr = &stderr
	if err = cmd.Run(); err != nil {
		err = fmt.Errorf("initialize gocryptfs encounters error, err: %w, stderr: %s", err, &stderr)
		return nil, err
	}

	mountParams := image.MountParams{
		Source:           cipherDir,
		Target:           plainDir,
		Filesystem:       "gocryptfs",
		Key:              []byte(cryptInfo.pass),
		DontElevatePrivs: true,
	}

	err = g.driver.Start(nil, 0, false)
	if err != nil {
		return nil, err
	}
	err = g.driver.Mount(&mountParams, nil)
	if err != nil {
		return nil, err
	}

	// Trap SIGINT/SIGTERM signals
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		<-ch
		sylog.Debugf("Received SIGINT/SIGTERM signal")
		g.driver.Stop(cryptInfo.plainDir)
	}()
	return cryptInfo, nil
}

func (g *Gocryptfs) create(files []string, dest string, opts []string, tmpDir string) error {
	cryptInfo, err := g.init(tmpDir)
	if err != nil {
		return err
	}
	g.Pass = cryptInfo.pass

	defer g.driver.Stop(cryptInfo.plainDir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	// First mksquashfs, which will squash rootfs
	fileName := filepath.Base(dest)
	newDest := filepath.Join(cryptInfo.plainDir, fileName)
	cmds := []string{g.MksquashfsPath}
	cmds = append(cmds, files...)
	cmds = append(cmds, newDest)
	cmds = append(cmds, opts...)
	err = cmdRun(cmds, &stdout, &stderr)
	if err != nil {
		return fmt.Errorf("calling mksquashfs encounters error: %w, stderr: %s", err, &stderr)
	}

	// Second mksquashfs, which will squash encrypted image and gocryptfs.conf
	encryptFile := filepath.Join(cryptInfo.cipherDir, fileName)
	cmds = []string{g.MksquashfsPath}
	cmds = append(cmds, []string{encryptFile, cryptInfo.confPath}...)
	cmds = append(cmds, dest)
	cmds = append(cmds, opts...)
	stdout.Reset()
	stderr.Reset()
	err = cmdRun(cmds, &stdout, &stderr)
	if err != nil {
		return fmt.Errorf("calling mksquashfs encounters error: %w, stderr: %s", err, &stderr)
	}

	return nil
}

func (g *Gocryptfs) Create(src []string, dest string, opts []string, tmpDir string) error {
	return g.create(src, dest, opts, tmpDir)
}

func cmdRun(commands []string, stdout, stderr io.Writer) error {
	sylog.Debugf("Executing commands: %v", commands)
	cmd := exec.Command(commands[0], commands[1:]...)
	if stdout != nil {
		cmd.Stdout = stdout
	}
	if stderr != nil {
		cmd.Stderr = stderr
	}
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}
