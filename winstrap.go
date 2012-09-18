package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
)

var files = map[string]string{
	"ChromeStandaloneSetup.exe": "https://dl.google.com/tag/s/appguid%3D%7B8A69D345-D564-463C-AFF1-A69D9E530F96%7D%26iid%3D%7BAF3A54A1-01DD-6358-922D-9F48BABA316B%7D%26lang%3Den%26browser%3D2%26usagestats%3D0%26appname%3DGoogle%2520Chrome%26needsadmin%3Dfalse%26installdataindex%3Ddefaultbrowser/update2/installers/ChromeStandaloneSetup.exe",
	"Mercurial.exe":             "http://mercurial.selenic.com/release/windows/Mercurial-2.3.1.exe",
	"mingw-inst.exe":            "http://superb-dca3.dl.sourceforge.net/project/mingw/Installer/mingw-get-inst/mingw-get-inst-20120426/mingw-get-inst-20120426.exe",
}

var altMain func()

func main() {
	if runtime.GOOS != "windows" {
		altMain()
		return
	}
	log.Printf("Downloading files.")
	var wg sync.WaitGroup
	for file, url := range files {
		wg.Add(1)
		go download(file, url, &wg)
	}
	wg.Wait()

	checkoutGo()

	checkMingw()

	// TODO(bradfitz): run make.bat, build the builder, ming64
	// overlay, etc

	fmt.Println("[ Press enter to exit ]")
	awaitEnter()
}

func checkMingw() {
	const mingDir = `c:\Mingw\bin`
	for !fileExists(mingDir) {
		log.Printf("%s doesn't exist. Install mingw and then press enter...")
		awaitEnter()
	}
}

func checkoutGo() {
	goroot := filepath.Join(home(), "goroot")
	if fileExists(goroot) {
		log.Printf("GOROOT %s already exists; skipping hg checkout", goroot)
		return
	}
	log.Printf("Checking out Go source using Mercurial (hg)")
	cmd := exec.Command("hg", "clone", "https://code.google.com/p/go", "goroot")
	cmd.Dir = home()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		log.Fatalf("hg clone failed. Is Mercurial installed? Re-run later. Error: %v", err)
	}
	log.Printf("Checked out Go.")
}

func awaitEnter() {
	var buf [1]byte
	os.Stdin.Read(buf[:])
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

func home() string { return os.Getenv("HOMEPATH") }

func download(file, url string, wg *sync.WaitGroup) {
	defer wg.Done()

	dst := filepath.Join(home(), "Desktop", file)
	if _, err := os.Stat(dst); err == nil {
		log.Printf("%s already on desktop; skipping", file)
		return
	}

	res, err := http.Get(url)
	if err != nil {
		log.Fatalf("Error fetching %v: %v", url, err)
	}
	tmp := dst + ".tmp"
	os.Remove(tmp)
	os.Remove(dst)
	f, err := os.Create(tmp)
	if err != nil {
		log.Fatal(err)
	}
	n, err := io.Copy(f, res.Body)
	res.Body.Close()
	if err != nil {
		log.Fatalf("Error reading %v: %v", url, err)
	}
	f.Close()
	err = os.Rename(tmp, dst)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Downladed %s (%d bytes) to desktop", file, n)
}
