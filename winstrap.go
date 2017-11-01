// Copyright 2012 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var files = map[string]string{
	// The "tdm64" one (despite the name) doesn't run on 64-bit Windows.
	// But the tdm-gcc one does, and installs both 32- and 64-bit versions.
	// No clue what tdm64 means.
	// "tdm64-gcc-4.8.1-3.exe": "http://downloads.sourceforge.net/project/tdm-gcc/TDM-GCC%20Installer/tdm64-gcc-4.8.1-3.exe?r=http%3A%2F%2Ftdm-gcc.tdragon.net%2Fdownload&ts=1407729829&use_mirror=ufpr",
	"tdm-gcc-4.9.2.exe": "https://sourceforge.net/projects/tdm-gcc/files/TDM-GCC%20Installer/Previous/1.1309.0/tdm-gcc-4.9.2.exe?r=http%3A%2F%2Ftdm-gcc.tdragon.net%2Fdownload&ts=1420336642&use_mirror=hivelocity",

	"Wix35.msi":          "http://storage.googleapis.com/winstrap/Wix35.msi",
	"Install Git.exe":    "https://github.com/msysgit/msysgit/releases/download/Git-1.9.5-preview20141217/Git-1.9.5-preview20141217.exe",
	"Start Buildlet.exe": "https://storage.googleapis.com/go-builder-data/buildlet-stage0.windows-amd64",
}

var altMain func()

var (
	flagYes = flag.Bool("yes", false, "Run without prompt")
	homeDir = flag.String("home", defaultHome(), "custom home directory")
)

func waitForGo() {
	if !awaitString("go") {
		log.Printf("Canceled.")
		awaitEnter()
		os.Exit(0)
	}
}

func main() {
	if runtime.GOOS != "windows" {
		altMain()
		return
	}
	flag.Parse()
	if !*flagYes {
		log.Printf("This program will first download TDM-GCC, Wix, and Git, then let you optinally install Go.\nType 'go<enter>' to proceed.")
		waitForGo()
	}

	// TODO(bradfitz): also download Go 1.4 into place at C:\Go1.4
	runBatFile := filepath.Join(home(), "Desktop", "run-builder.bat")
	if _, err := os.Stat(runBatFile); os.IsNotExist(err) {
		ioutil.WriteFile(runBatFile, []byte(strings.Replace(runBuilderBatContents, "\n", "\r\n", -1)), 0755)
	}

	log.Printf("Downloading files.")
	var errs []chan error
	for file, url := range files {
		errc := make(chan error)
		errs = append(errs, errc)
		go func(file, url string) {
			errc <- download(file, url)
		}(file, url)
	}
	var anyErr bool
	for _, errc := range errs {
		if err := <-errc; err != nil {
			log.Printf("Download error: %v", err)
			anyErr = true
		}
	}
	if anyErr {
		log.Printf("Download errors. Proceed? Type 'go'")
		waitForGo()
	}

	checkGit()
	checkGcc()

	log.Printf("This program will now check out go. Type 'go' to proceed.")
	waitForGo()

	checkoutGo()

	log.Printf("This program will now compile Go for 386 and amd64. Type 'go' to proceed.")
	waitForGo()
	runGoMakeBat("386")
	runGoMakeBat("amd64")

	log.Printf(`Installed go to %v, please add %v\bin to your PATH`, goroot(), goroot())

	fmt.Println("[ Press enter to exit ]")
	awaitEnter()
}

const gccPath = `C:\TDM-GCC-64\bin`

func runGoMakeBat(arch string) {
	if arch != "386" && arch != "amd64" {
		panic("invalid arch " + arch)
	}

	testFile := filepath.Join(goroot(), "pkg", "tool", "windows_"+arch, "api.exe")
	if fileExists(testFile) {
		log.Printf("Skipping make.bat for windows_%s; already built.", arch)
		return
	}

	log.Printf("Running make.bat for arch %s ...", arch)
	cmd := exec.Command(filepath.Join(goroot(), "src", "make.bat"))
	cmd.Dir = filepath.Join(goroot(), "src")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.Env = append([]string{
		"GOARCH=" + arch,
		"PATH=" + gccPath + ";" + os.Getenv("PATH"),
	}, removeEnvs(os.Environ(), "PATH")...)

	err := cmd.Run()
	if err != nil {
		log.Fatalf("make.bat for arch %s: %v", arch, err)
	}
	log.Printf("ran make.bat for arch %s", arch)
}

func removeEnvs(envs []string, removeKeys ...string) []string {
	var ret []string
	for _, env := range envs {
		include := true
		for _, remove := range removeKeys {
			if strings.HasPrefix(env, remove+"=") {
				include = false
				break
			}
		}
		if include {
			ret = append(ret, env)
		}
	}
	return ret
}

func checkGit() {
	for {
		if _, ok := gitBin(); ok {
			break
		}
		log.Print("Can't find git binary. Install Git and then press enter... (use middle option: make git available to cmd.exe)")
		awaitEnter()
	}
}

const gitDefaultPath = `C:\Program Files (x86)\Git\cmd\git.exe`

func gitBin() (string, bool) {
	b, err := exec.LookPath("git")
	if err != nil {
		b = gitDefaultPath
	}
	return b, fileExists(b)
}

func checkGcc() {
	for !fileExists(gccPath) {
		log.Printf("%s doesn't exist. Install gcc and then press enter...", gccPath)
		awaitEnter()
	}
}

func checkoutGo() {
	if fileExists(goroot()) {
		log.Printf("GOROOT %s already exists; skipping git checkout", goroot())
		return
	}
	log.Printf("Checking out Go source using git")

	git, _ := gitBin()
	cmd := exec.Command(git, "clone", "https://go.googlesource.com/go", "goroot")
	cmd.Dir = home()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("git clone failed. Is Git installed? Re-run later. Error: %v", err)
	}
	log.Printf("Checked out Go.")
}

func awaitEnter() {
	var buf [1]byte
	os.Stdin.Read(buf[:])
}

func awaitString(want string) bool {
	br := bufio.NewReader(os.Stdin)
	ln, _, _ := br.ReadLine()
	return strings.TrimSpace(string(ln)) == want
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func defaultHome() string { return os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH") }

func home() string {
	if *homeDir != "" {
		return *homeDir
	}
	return defaultHome()
}

func goroot() string { return filepath.Join(home(), "goroot") }

func gopath() string { return filepath.Join(home(), "gopath") }

func download(file, url string) error {
	dst := filepath.Join(home(), "Desktop", file)
	if _, err := os.Stat(dst); err == nil {
		log.Printf("%s already on desktop; skipping", file)
		return nil
	}

	res, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("Error fetching %v: %v", url, err)
	}
	tmp := dst + ".tmp"
	os.Remove(tmp)
	os.Remove(dst)
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	n, err := io.Copy(f, res.Body)
	res.Body.Close()
	if err != nil {
		return fmt.Errorf("Error reading %v: %v", url, err)
	}
	f.Close()
	err = os.Rename(tmp, dst)
	if err != nil {
		return err
	}
	log.Printf("Downladed %s (%d bytes) to desktop", file, n)
	return nil
}

const runBuilderBatContents = `echo Running the Go builder:
mkdir \Users\wingopher\gopath

RMDIR /S /Q c:\gobuilder
SET GOROOT_BOOTSTRAP=c:\Go1.4
SET GOPATH=\Users\wingopher\gopath
SET PATH=\Users\wingopher\goroot\bin;%PATH%
go get -u -v golang.org/x/tools/dashboard/builder
%GOPATH%\bin\builder -v -parallel windows-amd64 windows-386
`
