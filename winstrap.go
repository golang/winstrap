package main

import (
	"fmt"
	"net/http"
	"log"
	"sync"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

var files = map[string]string{
	"ChromeStandaloneupdate2/installers/ChromeStandaloneSetup.exe": "https://dl.google.com/tag/s/appguid%3D%7B8A69D345-D564-463C-AFF1-A69D9E530F96%7D%26iid%3D%7BAF3A54A1-01DD-6358-922D-9F48BABA316B%7D%26lang%3Den%26browser%3D2%26usagestats%3D0%26appname%3DGoogle%2520Chrome%26needsadmin%3Dfalse%26installdataindex%3Ddefaultbrowser/update2/installers/ChromeStandaloneSetup.exe",
	"Mercurial.exe": "http://mercurial.selenic.com/release/windows/Mercurial-2.3.1.exe",
	"mingw-inst.exe": "http://superb-dca3.dl.sourceforge.net/project/mingw/Installer/mingw-get-inst/mingw-get-inst-20120426/mingw-get-inst-20120426.exe",
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
	fmt.Println("TODO: more.")
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func download(file, url string, wg *sync.WaitGroup) {
	defer wg.Done()

	dst := filepath.Join(os.Getenv("HOMEPATH"), "Desktop", file)
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
