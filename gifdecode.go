package main

import (
	"bytes"
	"crypto/md5"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nfnt/resize"
)

var listenPort int
var ffmpegbin string
var cacheDir string
var cacheExpire int
var downLoadTimeout int

type gifFile struct {
	filename string
	index    int
}

type gifFiles []gifFile

func (s gifFiles) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s gifFiles) Less(i, j int) bool {
	return s[i].index < s[j].index
}
func (s gifFiles) Len() int {
	return len(s)
}

func init() {
	flag.IntVar(&listenPort, "port", 9100, "The Listen Port")
	flag.StringVar(&ffmpegbin, "ffmpeg", "ffmpeg", "The bin of ffmpeg,make sure it is in the PATH")
	flag.StringVar(&cacheDir, "cache", "./cache", "The cache dir of gif and mp4,make sure the directory can be writed.")
	flag.IntVar(&cacheExpire, "expire", 3, "The days expire of cache files")
	flag.IntVar(&downLoadTimeout, "downTimeout", 60, "The timeout (seconds) of download resource file")
}

func removeOldFile(filename string) {

	filepath.Walk(cacheDir, func(path string, fi os.FileInfo, err error) error {
		if nil == fi {
			return err
		}
		if fi.IsDir() {
			return err
		}
		name := fi.Name()
		_, filename = filepath.Split(filename)
		if matched, _ := filepath.Match(filename+"*-*.gif", name); matched {
			os.Remove(cacheDir + "/" + name)
		}
		return err
	})
}

func clearCache() {
	refreshTicker := time.NewTicker(3600 * time.Second)
	for {
		select {
		case <-refreshTicker.C:
			err := filepath.Walk(cacheDir, func(path string, f os.FileInfo, err error) error {
				if f == nil {
					log.Println("Get FileInfo Nil: ", path)
					return nil
				}
				isExpired := int(time.Now().Sub(f.ModTime()).Hours()) > cacheExpire*24
				if !f.IsDir() && isExpired {
					err := os.Remove(path)
					if err != nil {
						log.Println("Remove File Error:", err)
						return nil //删除失败仍旧继续
					} else {
						log.Println("Remove File: ", path)
					}
				}
				return nil
			})
			if err != nil {
				log.Println("Err:", err)
			}
		}
	}
}

func main() {

	runtime.GOMAXPROCS(runtime.NumCPU())
	flag.Parse()

	checkFfmpeg()
	checkCacheDir()

	go clearCache()

	http.Handle("/imgs/", http.StripPrefix("/imgs/", http.FileServer(http.Dir(cacheDir))))

	http.HandleFunc("/gif", gifHandler)
	http.HandleFunc("/mp4", mp4Handler)
	http.ListenAndServe(":"+strconv.Itoa(listenPort), nil)
}

func checkFfmpeg() {
	cmd := exec.Command(ffmpegbin, "-version")
	if err := cmd.Run(); err != nil {
		log.Fatal("ffmpeg is not exists,make sure your setting of -ffmpeg is correct, error detail: ", err)
	}
}

func checkCacheDir() {
	fileinfo, err := os.Stat(cacheDir)
	if err != nil {
		log.Fatal("The directory of Cache is not exists,make sure your setting of -cache is correct, error detail: ", os.IsExist(err))
	}
	if !fileinfo.IsDir() {
		log.Fatal("The directory of Cache is not a directory,make sure your setting of -cache is correct")
	}
}

func down(url string, fname string) error {

	_, err := os.Stat(fname)
	if err == nil || os.IsExist(err) {
		return nil
	}

	response, err := getUrl(url, time.Duration(downLoadTimeout)*time.Second, "", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/49.0.2623.110 Safari/537.36", "")

	if err != nil {
		log.Printf("%s", err)
		return err
	} else if response.StatusCode != 200 {
		return errors.New(response.Status)
	} else {
		defer response.Body.Close()
		contents, err := ioutil.ReadAll(response.Body)
		if err != nil {
			log.Printf("%s", err)
			return err
		}
		file, err := os.Create(fname)
		_, err = file.Write(contents)
		return err
	}
}

func getFileName(src string, ext string) string {
	u, _ := url.Parse(src)
	return fmt.Sprintf("%x.%s", md5.Sum([]byte(u.Host+u.Path)), ext)
}

func mp4Handler(w http.ResponseWriter, req *http.Request) {
	mp4 := req.URL.Query().Get("src")
	u, err := url.Parse(mp4)
	if mp4 == "" || err != nil {
		w.Write([]byte("The mp4 is not a url."))
		return
	}

	if path.Ext(u.Path) != ".mp4" {
		w.Write([]byte("The mp4 is not a mp4 postfix."))
		return
	}
	refresh := req.URL.Query().Get("refresh")
	preheat := req.URL.Query().Get("preheat")

	rate, err := strconv.ParseFloat(req.URL.Query().Get("fps"), 10)

	if err != nil || rate <= 0 {
		rate = 1
	}

	quality, err := strconv.Atoi(req.URL.Query().Get("quality"))

	if err != nil || quality <= 0 || quality > 100 {
		quality = 80
	}

	xzoom, err := strconv.Atoi(req.URL.Query().Get("width"))

	if err != nil || xzoom <= 1000 {
		xzoom = 2048
	}

	w.Header().Set("Content-Type", "text/html")

	if preheat != "1" {
		filename := cacheDir + "/" + getFileName(mp4, "mp4")
		_, err = os.Stat(filename + "-0.gif")
		if err != nil || os.IsExist(err) || refresh == "1" {
			if err := down(mp4, filename); err != nil {
				w.Write([]byte("Down Faild, err:" + err.Error()))
				return
			}
			if refresh != "1" && showCache(filename, w) {
				return
			}
			processMp4(filename, quality, rate, xzoom)
		}
		showCache(filename, w)
	} else {
		w.Write([]byte("preheat ok"))
		go func() {
			filename := cacheDir + "/" + getFileName(mp4, "mp4")
			_, err = os.Stat(filename + "-0.gif")
			if err != nil || os.IsExist(err) || refresh == "1" {
				if err := down(mp4, filename); err != nil {
					w.Write([]byte("Down Faild, err:" + err.Error()))
					return
				}
				if refresh != "1" && showCache(filename, w) {
					return
				}
				processMp4(filename, quality, rate, xzoom)
			}
		}()
	}

}

func processMp4(filename string, quality int, rate float64, xzoom int) {

	removeOldFile(filename)
	cmd := exec.Command(ffmpegbin, "-i", filename, "-r", fmt.Sprintf("%.2f", rate), "-f", "image2pipe", "-codec:v", "png", "-")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}

	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}

	var imgList []image.Image
	for i := 0; i < 1000; i++ {
		im, err := png.Decode(stdout)
		if err == io.ErrUnexpectedEOF {
			break
		} else if err != nil {
			fmt.Println(err)
		}
		imgList = append(imgList, im)
	}

	imgWidth, imgHeight := imgList[0].Bounds().Dx(), imgList[0].Bounds().Dy()
	count := len(imgList)
	rows, cols := getRowsCols(count, imgWidth, xzoom)
	taskNum := int(math.Ceil(float64(rows) / float64(runtime.NumCPU())))

	var ch = make(chan int)
	for i := 0; i < count; i += taskNum * cols {
		last := i + taskNum*cols
		if last > count {
			last = count
		}

		go func(i, last int) {
			buffer := mergeImage(last-i, quality, taskNum, cols, imgWidth, imgHeight, imgList[i:last])
			file, _ := os.Create(fmt.Sprintf("%s-%d.gif", filename, i))
			_, err = file.Write([]byte(buffer.String()))
			ch <- 1
		}(i, last)
	}

	for i := 0; i < count; i += taskNum * cols {
		<-ch
	}
}

func mergeImage(count, quality, rows, cols, imgWidth, imgHeight int, imgList interface{}) bytes.Buffer {

	var buffer bytes.Buffer
	newImage := image.NewRGBA(image.Rect(0, 0, cols*imgWidth, rows*imgHeight))
	for y := 0; y < rows; y++ {
		for x := 0; x < cols; x++ {
			if x+y*cols >= count {
				break
			}
			index := x + y*cols
			for py := 0; py < imgHeight; py++ {
				for px := 0; px < imgWidth; px++ {
					if reflect.TypeOf(imgList).String() == "[]*image.Paletted" {
						newImage.Set(px+imgWidth*x, py+imgHeight*y, imgList.([]*image.Paletted)[index].At(px, py))
					} else if reflect.TypeOf(imgList).String() == "[]image.Image" {
						newImage.Set(px+imgWidth*x, py+imgHeight*y, imgList.([]image.Image)[index].At(px, py))
					}
				}
			}

		}
	}

	err := jpeg.Encode(&buffer, newImage, &jpeg.Options{quality})
	if err != nil {
		log.Println(err)
		err = png.Encode(&buffer, newImage)
		log.Println(err)
	}
	return buffer
}

func gifHandler(w http.ResponseWriter, req *http.Request) {

	w.Header().Set("Content-Type", "text/html")
	gif := req.URL.Query().Get("src")
	u, err := url.Parse(gif)
	if err != nil {
		w.Write([]byte("The gif is not a url."))
		return
	}

	if path.Ext(u.Path) != ".gif" {
		w.Write([]byte("The image is not a Gif."))
		return
	}

	refresh := req.URL.Query().Get("refresh")
	preheat := req.URL.Query().Get("preheat")

	quality, err := strconv.Atoi(req.URL.Query().Get("quality"))

	if err != nil || quality <= 0 || quality > 100 {
		quality = 80
	}

	xzoom, err := strconv.Atoi(req.URL.Query().Get("width"))

	if err != nil || xzoom <= 1000 {
		xzoom = 2048
	}

	if preheat != "1" {
		filename := cacheDir + "/" + getFileName(gif, "gif")

		_, err = os.Stat(filename + "-0.gif")
		if err != nil || os.IsExist(err) || refresh == "1" {
			err = down(gif, filename)

			if refresh != "1" && showCache(filename, w) {
				return
			}
			splitGif(filename, quality, xzoom)
		}
		showCache(filename, w)
	} else {
		w.Write([]byte("preheat ok"))
		go func() {
			filename := cacheDir + "/" + getFileName(gif, "gif")
			_, err = os.Stat(filename + "-0.gif")
			if err != nil || os.IsExist(err) || refresh == "1" {
				err = down(gif, filename)

				if refresh != "1" && showCache(filename, w) {
					return
				}
				splitGif(filename, quality, xzoom)
			}
		}()
	}
}

func showCache(filename string, w http.ResponseWriter) bool {
	var images gifFiles
	filepath.Walk(cacheDir, func(path string, fi os.FileInfo, err error) error {
		if nil == fi {
			return err
		}
		if fi.IsDir() {
			return err
		}
		name := fi.Name()
		_, filename = filepath.Split(filename)
		if matched, _ := filepath.Match(filename+"*-*.gif", name); matched {
			index, err := strconv.Atoi(strings.Split(strings.TrimRight(name, ".gif"), "-")[1])
			if err != nil {
				index = 0
			}
			images = append(images, gifFile{name, index})
		}
		return err
	})

	if len(images) == 0 {
		return false
	}
	w.Write([]byte("<html><body bgcolor='#000'>\n"))

	sort.Sort(images)
	for _, image := range images {
		w.Write([]byte("<img src='/imgs/" + image.filename + "' width='100%'>\n"))
	}
	w.Write([]byte("</body></html>"))
	return true
}

// Decode reads and analyzes the given reader as a GIF image
func splitGif(filename string, quality int, xzoom int) {

	removeOldFile(filename)

	f, err := os.Open(filename)
	if err != nil {
		return
	}
	defer f.Close()

	im, err := gif.DecodeAll(f)

	if err != nil {
		return
	}

	imgWidth, imgHeight := getGifDimensions(im)

	count := len(im.Image)
	rows, cols := getRowsCols(count, imgWidth, xzoom)
	taskNum := int(math.Ceil(float64(rows) / float64(runtime.NumCPU())))

	var ch = make(chan int)
	for i := 0; i < count; i += taskNum * cols {
		last := i + taskNum*cols
		if last > count {
			last = count
		}

		go func(i, last int) {
			buffer := mergeImage(last-i, quality, taskNum, cols, imgWidth, imgHeight, im.Image[i:last])
			file, _ := os.Create(fmt.Sprintf("%s-%d.gif", filename, i))
			_, err = file.Write([]byte(buffer.String()))
			ch <- 1
		}(i, last)
	}

	for i := 0; i < count; i += taskNum * cols {
		<-ch
	}

}

func getRowsCols(count, imgWidth, xzoom int) (int, int) {
	cols := int(math.Ceil(float64(xzoom / imgWidth)))
	rows := int(math.Ceil(float64(float64(count) / float64(cols))))
	return rows, cols
}

func ProcessImage(img image.Image, width int, height int) image.Image {
	return resize.Resize(uint(width), uint(height), img, resize.Lanczos2)
}

func ImageToPaletted(img image.Image) *image.Paletted {
	b := img.Bounds()
	pm := image.NewPaletted(b, palette.Plan9)
	draw.FloydSteinberg.Draw(pm, b, img, image.ZP)
	return pm
}

func getGifDimensions(im *gif.GIF) (x, y int) {
	firstFrame := im.Image[0].Bounds()
	return firstFrame.Dx(), firstFrame.Dy()
}

func checkFrameWidthHeight(imPaltetted []*image.Paletted) bool {
	firstFrame := imPaltetted[0].Bounds()
	for _, frame := range imPaltetted {
		bounds := frame.Bounds()
		if bounds.Dx() != firstFrame.Dx() || bounds.Dy() != firstFrame.Dy() {
			return true
		}
	}
	return false
}

func getUrl(url string, timeout time.Duration, host, ua, refer string) (*http.Response, error) {

	tr := &http.Transport{
		TLSClientConfig:    &tls.Config{InsecureSkipVerify: true},
		DisableCompression: true,
	}

	client := &http.Client{
		Transport: tr,
		Timeout:   timeout,
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if host != "" {
		req.Header.Add("host", host)
	}
	if ua != "" {
		req.Header.Add("User-Agent", ua)
	}
	if refer != "" {
		req.Header.Add("Referer", refer)
	}

	return client.Do(req)
}
