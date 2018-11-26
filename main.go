package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

var (
	package_url   string
	package_md5   string
	untar_path    string
	rest_path     string
	update_option string
	script_path   string
)

func init() {
	flag.StringVar(&package_url, "package_url", "", "升级包URL")
	flag.StringVar(&package_md5, "package_md5", "", "升级包MD5")
	flag.StringVar(&untar_path, "untar_path", "/tmp", "升级包解压路径")
	flag.StringVar(&rest_path, "rest_path", "", "设备接口路径")
	flag.StringVar(&update_option, "update_option", "replace", "升级命令")
	flag.StringVar(&script_path, "script_path", "", "升级脚本相对路径")
}

func main() {
	flag.Parse()
	// 参数校验
	if package_url == "" || rest_path == "" || untar_path == "" || rest_path == "" {
		flag.Usage()
		os.Exit(1)
	}

	// 下载升级包
	api_package := "api.tar.gz"
	fn := func() error { return DownloadFile(api_package, package_url) }
	err := retry(3, 1*time.Second, fn)
	if err != nil {
		panic(err)
	}

	// 校验md5
	md5err := CheckMD5(api_package, package_md5)
	if md5err != nil {
		panic(md5err)
	}

	// 替换流程
	tar_err := UnTar(api_package, untar_path)
	if tar_err != nil {
		panic(tar_err)
	}
	UpdateDir(rest_path, filepath.Join(untar_path, "restapi"))
	os.Remove(api_package)
}

func retry(attempts int, sleep time.Duration, fn func() error) error {
	if err := fn(); err != nil {
		if s, ok := err.(stop); ok {
			return s.error
		}

		if attempts--; attempts > 0 {
			fmt.Println("retry!")
			time.Sleep(sleep)
			return retry(attempts, 2*sleep, fn)
		}
		return err
	}

	return nil
}

type stop struct {
	error
}

// DownloadFile will download a url to a local file. It's efficient because it will
// write as it downloads and not load the whole file into memory.
func DownloadFile(dst string, url string) error {

	// Create the file
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Write the body to file
	bodymd5 := md5.New()
	w := io.MultiWriter(bodymd5, out)

	_, err = io.Copy(w, resp.Body)
	if err != nil {
		return err
	}

	fmt.Printf("body md5 is %s\n", hex.EncodeToString(bodymd5.Sum(nil)))

	return nil
}

func CheckMD5(dst string, filemd5 string) error {
	file, err := os.Open(dst)
	if err != nil {
		return err
	}
	defer file.Close()
	md5hash := md5.New()
	if _, err := io.Copy(md5hash, file); err != nil {
		return err
	}
	fmt.Printf("%x\n", md5hash.Sum(nil))
	md5string := hex.EncodeToString(md5hash.Sum(nil))
	if filemd5 != md5string {
		os.Exit(1)
	}
	return nil
}

func UnTar(src string, dst string) error {

	// file read
	fr, err := os.Open(src)
	if err != nil {
		panic(err)
	}
	defer fr.Close()

	// gzip read
	gr, err := gzip.NewReader(fr)
	if err != nil {
		panic(err)
	}
	defer gr.Close()

	// tar read
	tr := tar.NewReader(gr)

	// tar read to file
	for {

		h, err := tr.Next()

		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}
		if h == nil {
			continue
		}

		// display
		fmt.Println(h.Name)

		dstFileDir := filepath.Join(dst, h.Name)

		switch h.Typeflag {
		case tar.TypeDir:
			if b := ExistDir(dstFileDir); !b {
				if err := os.MkdirAll(dstFileDir, 0755); err != nil {
					return err
				}
			}
		case tar.TypeReg:
			file, err := os.OpenFile(dstFileDir, os.O_CREATE|os.O_RDWR, os.FileMode(h.Mode))
			if err != nil {
				return err
			}
			n, err := io.Copy(file, tr)
			if err != nil {
				return err
			}

			fmt.Printf("解压： %s, 处理 %d 字符\n", dstFileDir, n)

			file.Close()
		}
	}
	return err
}

func ExistDir(dirname string) bool {
	fi, err := os.Stat(dirname)
	return (err == nil || os.IsExist(err)) && fi.IsDir()
}

func CopyDir(src, dst string) error {
	cmd := exec.Command("cp", "a")
	log.Printf("Running cp -a")
	return cmd.Run()
}

func UpdateDir(olddir string, newdir string) error {

	os.RemoveAll(olddir + ".bak")

	err_bak := os.Rename(olddir, olddir+".bak")
	if err_bak != nil {
		panic(err_bak)
	}

	err := os.Rename(newdir, olddir)
	if err != nil {
		panic(err)
	}

	return nil

}
