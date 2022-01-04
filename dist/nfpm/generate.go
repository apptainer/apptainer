package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"text/template"
)

const (
	appName = "apptainer"
)

type TemplateData struct {
	ConfDir    string
	LibExecDir string
	BinDir     string
	SessionDir string

	Platform string
	Arch     string

	Version     string
	AppName     string
	PackageName string

	Rootless bool
}

func main() {
	flagSet := flag.NewFlagSet("generate", flag.ExitOnError)
	version := flagSet.String("version", "", "generate package with version")
	prefix := flagSet.String("prefix", "/usr/local", "generate package with prefix")
	rootless := flagSet.Bool("rootless", false, "generate rootless package")
	packageName := flagSet.String("name", appName, "package name")

	if err := flagSet.Parse(os.Args[1:]); err != nil {
		log.Fatal(err)
	}

	if version == nil || *version == "" {
		log.Fatalf("a package version is required")
	}
	if prefix == nil || *prefix == "" {
		log.Fatalf("a package prefix is required")
	}

	var td TemplateData

	td.AppName = appName
	td.PackageName = *packageName
	td.BinDir = filepath.Join(*prefix, "bin")
	td.LibExecDir = filepath.Join(*prefix, "libexec")
	td.ConfDir = filepath.Join(*prefix, "etc", appName)
	td.SessionDir = filepath.Join(*prefix, "var", appName, "mnt/session")
	td.Rootless = *rootless
	td.Version = *version
	td.Platform = runtime.GOOS
	td.Arch = runtime.GOARCH

	if err := os.MkdirAll("builddir/nfpm/scripts", 0o755); err != nil {
		log.Fatalf("while creating builddir/nfpm/scripts: %s", err)
	}

	file := "dist/nfpm/nfpm.yaml"
	t, err := ioutil.ReadFile(file)
	if err != nil {
		log.Fatalf("while reading %s: %s", file, err)
	}

	buf := new(bytes.Buffer)
	tmpl, err := template.New("nfpm.yaml").Parse(string(t))
	if err != nil {
		log.Fatalf("while parsing template nfpm.yaml: %s", err)
	}
	if err := tmpl.Execute(buf, &td); err != nil {
		log.Fatalf("while executing template nfpm.yaml: %s", err)
	}

	fmt.Println(buf.String())
}
