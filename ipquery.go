package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
)

var (
	ipfile = flag.String("ipfile", "./ip_standard.txt", "Ip file path,default is ./ip_standard.txt ")
)

//IPLib format: 1861807616      1861807871      中国    河北省  邯郸市  永年县  未知    联通
type ipDataStruct struct {
	begin    int
	end      int
	Country  string
	Province string
	City     string
	County   string
	Detail   string
	Isp      string
}

type Ipdata []ipDataStruct

var ipdata_slic Ipdata

func main() {
	flag.Parse()

	if *ipfile == "" || !Exist(*ipfile) {
		log.Println("Ip file can't be null")
		return
	}

	loadIpData()
	fmt.Println("loading done:", len(ipdata_slic))

	http.HandleFunc("/ip", ipQuery)
	err := http.ListenAndServe(":19090", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}

}

func ipQuery(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	ip := r.URL.Query().Get("ip")
	if ip == "" {
		fmt.Fprintf(w, "IP can't be null")
		return
	}
	ipinfo := searchIP(Ip2Long(ip), 0, len(ipdata_slic))

	b, err := json.Marshal(&ipinfo)
	if err != nil {
		fmt.Printf("Error: %s", err)
		return
	}
	fmt.Fprintf(w, string(b))
}

func searchIP(ip, begin, end int) ipDataStruct {

	mid := int(math.Ceil(float64(end-begin)/2.0)) + begin
	current := ipdata_slic[mid]
	if ip >= current.begin && ip <= current.end {
		return current
	}
	if ip < current.begin {
		return searchIP(ip, begin, mid)
	} else if ip > current.end {
		return searchIP(ip, mid, end)
	}
	return current
}

func Ip2Long(ip string) int {
	var bits = strings.Split(ip, ".")
	b0, _ := strconv.Atoi(bits[0])
	b1, _ := strconv.Atoi(bits[1])
	b2, _ := strconv.Atoi(bits[2])
	b3, _ := strconv.Atoi(bits[3])

	var sum int

	sum += int(b0) << 24
	sum += int(b1) << 16
	sum += int(b2) << 8
	sum += int(b3)

	return sum

}

func loadIpData() {

	fi, err := os.Open(*ipfile)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}
	defer fi.Close()

	ch := make(chan int, 10000)

	br := bufio.NewReader(fi)
	for {
		a, _, c := br.ReadLine()
		if c == io.EOF {
			break
		}
		go buildIpdata(string(a), ch)
		<-ch
	}
}

func buildIpdata(str string, ch chan int) {

	strSlic := strings.Split(str, "\t")

	var ip ipDataStruct
	ip.begin, _ = strconv.Atoi(strSlic[0])
	ip.end, _ = strconv.Atoi(strSlic[1])
	ip.Country = strSlic[2]
	ip.Province = strSlic[3]
	ip.City = strSlic[4]
	ip.County = strSlic[5]
	ip.Detail = strSlic[6]
	ip.Isp = strSlic[7]

	ipdata_slic = append(ipdata_slic, ip)
	ch <- 1
}

func Exist(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil || os.IsExist(err)
}
