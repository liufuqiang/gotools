package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

var (
	pod = flag.String("pod", "", "The pod name")
	c   = flag.String("c", "", "The container name")
)

func system(s string) string {
	cmd := exec.Command("/bin/sh", "-c", s) 
	var out bytes.Buffer                    

	cmd.Stdout = &out 
	err := cmd.Run()  
	if err != nil {
		fmt.Println("maybe your input pod name error")
		log.Fatal(err)
	}
	return out.String()
}

func system2(s string) {
	cmd := exec.Command("/bin/sh", "-c", s) 

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run() 
	if err != nil {
		log.Fatal(err)
	}
}

func main() {

	flag.Parse()

	if *pod == "" {
		fmt.Println("The pod name can't be null")
		return
	}

	cmdStr := "kubectl get po |grep " + *pod

	str := system(cmdStr)
	for _, line := range strings.Split(str, "\n") {
		if line == "" {
			continue
		}
		cols := strings.Split(line, " ")

		cmdStr = "kubectl logs  -f --tail=1 " + cols[0]
		if *c != "" {
			cmdStr += " -c " + *c
		}
		go func(cmd string) {
			system2(cmd)
		}(cmdStr)
	}

	for {
	}

}
