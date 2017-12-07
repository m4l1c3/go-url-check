package main

import (
	"flag"
	"fmt"
	requests "github.com/hiroakis/go-requests"
	"io/ioutil"
)

func main() {
	resp, err := requests.Get("https://google.com", nil, nil)

	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(resp.Text())

	fmt.Printf("Code: %d\n", resp.StatusCode())
	for k, v := range resp.Headers() {
		fmt.Printf("%s: %s\n", k, v)
	}
}

func readWordList() {
	b, err := ioutil.ReadFile()

}
