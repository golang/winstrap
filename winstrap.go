package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

var files = map[string]string{
	"ChromeStandaloneSetup.exe": "https://dl.google.com/tag/s/appguid%3D%7B8A69D345-D564-463C-AFF1-A69D9E530F96%7D%26iid%3D%7BAF3A54A1-01DD-6358-922D-9F48BABA316B%7D%26lang%3Den%26browser%3D2%26usagestats%3D0%26appname%3DGoogle%2520Chrome%26needsadmin%3Dfalse%26installdataindex%3Ddefaultbrowser/update2/installers/ChromeStandaloneSetup.exe",
	"Mercurial.exe":             "http://mercurial.selenic.com/release/windows/Mercurial-2.3.1.exe",
	"mingw-inst.exe":            "http://superb-dca3.dl.sourceforge.net/project/mingw/Installer/mingw-get-inst/mingw-get-inst-20120426/mingw-get-inst-20120426.exe",
}

var altMain func()

var flagYes = flag.Bool("yes", false, "Run without prompt")

func main() {
	if runtime.GOOS != "windows" {
		altMain()
		return
	}
	flag.Parse()
	if !*flagYes {
		log.Printf("This program will install Go, Mingw, Mercurial, Chrome, etc. Type 'go<enter>' to proceed.")
		if !awaitString("go") {
			log.Printf("Canceled.")
			awaitEnter()
			return
		}
	}

	log.Printf("Downloading files.")
	var wg sync.WaitGroup
	for file, url := range files {
		wg.Add(1)
		go download(file, url, &wg)
	}
	wg.Wait()

	checkMingw()
	checkoutGo()

	runGoMakeBat("386")
	runGoMakeBat("amd64")

	// TODO(bradfitz): run make.bat, build the builder, ming64
	// overlay, etc

	fmt.Println("[ Press enter to exit ]")
	awaitEnter()
}

const (
	mingwBin   = `C:\MingW\bin`
	mingw64Bin = `C:\MingW64\bin`
)

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

	gccPath := mingwBin
	if arch == "amd64" {
		gccPath = mingw64Bin
		if err := initMingw64Bin(); err != nil {
			log.Fatalf("failed to init C:\\mingw64\\bin directory: %v", err)
		}
	}

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

func initMingw64Bin() error {
	dstGcc := filepath.Join(mingw64Bin, "gcc.exe")
	dstAr := filepath.Join(mingw64Bin, "ar.exe")
	srcGcc := filepath.Join(mingwBin, "x86_64-w64-mingw32-gcc.exe")
	srcAr := filepath.Join(mingwBin, "x86_64-w64-mingw32-ar.exe")
	if fileExists(dstGcc) && fileExists(dstAr) {
		return nil
	}
	if err := os.MkdirAll(mingw64Bin, 0755); err != nil {
		return err
	}
	if !fileExists(dstGcc) {
		if err := cp(srcGcc, dstGcc); err != nil {
			return err
		}
	}
	if !fileExists(dstAr) {
		if err := cp(srcAr, dstAr); err != nil {
			return err
		}
	}
	return nil
}

func cp(src, dst string) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("copying from %s to %s: %v", src, dst, err)
		}
	}()
	f, err := os.Open(src)
	if err != nil {
		return
	}
	defer f.Close()
	os.Remove(dst)
	f2, err := os.Create(dst)
	if err != nil {
		return
	}
	_, err = io.Copy(f2, f)
	if err != nil {
		return
	}
	err = f2.Close()
	return
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

func checkMingw() {
	for !fileExists(mingwBin) {
		log.Printf("%s doesn't exist. Install mingw and then press enter...")
		awaitEnter()
	}
}

func checkoutGo() {
	if fileExists(goroot()) {
		log.Printf("GOROOT %s already exists; skipping hg checkout", goroot())
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

func home() string { return os.Getenv("HOMEPATH") }

func goroot() string { return filepath.Join(home(), "goroot") }

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
