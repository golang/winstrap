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
	"ChromeStandaloneSetup.exe": "https://dl.google.com/tag/s/appguid%3D%7B8A69D345-D564-463C-AFF1-A69D9E530F96%7D%26iid%3D%7BC159FD9F-6827-8E7E-0CC8-7783A1335AA5%7D%26lang%3Den%26browser%3D4%26usagestats%3D0%26appname%3DGoogle%2520Chrome%26needsadmin%3Dprefers%26installdataindex%3Ddefaultbrowser/update2/installers/ChromeStandaloneSetup.exe",
	"Mercurial.exe":             "http://mercurial.selenic.com/release/windows/Mercurial-3.1-x64.exe",
	"tdm64-gcc-4.8.1-3.exe":     "http://downloads.sourceforge.net/project/tdm-gcc/TDM-GCC%20Installer/tdm64-gcc-4.8.1-3.exe?r=http%3A%2F%2Ftdm-gcc.tdragon.net%2Fdownload&ts=1407729829&use_mirror=ufpr",
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

	checkGcc()
	checkoutGo()

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

func checkGcc() {
	for !fileExists(gccPath) {
		log.Printf("%s doesn't exist. Install gcc and then press enter...", gccPath)
		awaitEnter()
	}
}

const hgDefaultPath = `C:\Program Files\Mercurial\hg.exe`

func checkoutGo() {
	if fileExists(goroot()) {
		log.Printf("GOROOT %s already exists; skipping hg checkout", goroot())
		return
	}
	log.Printf("Checking out Go source using Mercurial (hg)")

	hgBin, err := exec.LookPath("hg")
	if err != nil {
		hgBin = hgDefaultPath
		log.Printf("Couldn't find hg binary in PATH, trying %v", hgBin)
	}
	cmd := exec.Command(hgBin, "clone", "https://code.google.com/p/go", "goroot")
	cmd.Dir = home()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
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
