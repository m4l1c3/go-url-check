package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"

	requests "github.com/hiroakis/go-requests"
)

type IntSet struct {
	set map[int]bool
}

type StringSet struct {
	set map[string]bool
}

type State struct {
	ProxyURL       string
	Verbose        bool
	Threads        int
	OutputFileName string
	Wordlist       string
	StatusCodes    IntSet
	URL            string
}

func ParseWordlist(wordlist string) ([]string, error) {
	if wordlist != "" {
		if _, err := os.Stat(wordlist); os.IsNotExist(err) {
			file, error := OpenFile(wordlist)
			if error != nil {
				return nil, error
			}
			list, errorTwo := ReadLines(file)
			if errorTwo != nil {
				return nil, errorTwo
			}
			return list, nil
		}
	}
	return nil, nil
}

func ReadLines(file *File) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func OpenFile(filename string) (*File, error) {
	b, err := os.Open(filename)
	defer b.Close()

	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return b, nil
}

func ParseArgs() *State {
	var codes string
	valid := true
	s := State{
		StatusCodes: IntSet{set: map[int]bool{}},
	}

	flag.IntVar(&s.Threads, "t", 10, "Number of concurrent threads")
	flag.BoolVar(&s.Verbose, "v", false, "Verbose output (errors)")
	flag.StringVar(&s.ProxyURL, "p", "", "Proxy to use for requests [http(s)://host:port]")
	flag.StringVar(&s.OutputFileName, "o", "", "Output file to write results to (defaults to stdout)")
	flag.StringVar(&s.URL, "u", "", "The target URL or Domain")
	flag.StringVar(&s.Wordlist, "w", "", "Path to the wordlist")
	flag.StringVar(&codes, "s", "200,204,301,302,307", "Positive status codes")
	flag.Parse()

	if s.Threads < 0 {
		fmt.Println("[!] Invalid number of threads (-t)", s.Threads)
		valid = false
	}

	if s.URL == "" && s.Wordlist == "" {
		fmt.Println("[!] Unable to start checking both URL (-u) and Wordlist are invalid (-w)", s.URL, s.Wordlist)
		valid = false
	}

	if valid {
		Banner(&s)
		ShowOptions(&s)
		return &s
	}

	return nil
}

func Banner(state *State) {
	if state.Verbose {
		fmt.Println("=====================================================================")
		fmt.Println("**                              _            _               _     **")
		fmt.Println("** 	                       | |          | |             | |    **")
		fmt.Println("**  __ _  ___ ______ _   _ _ __| |______ ___| |__   ___  ___| | __ **")
		fmt.Println("** / _` |/ _ \\______| | | | '__| |______/ __| '_ \\ / _ \\/ __| |/ / **")
		fmt.Println("** |(_| |(_)|     | |_| | |  | |     | (__| | | |  __/ (__|   <  **")
		fmt.Println("** \\__, |\\___/       \\__,_|_|  |_|      \\___|_| |_|\\___|\\___|_|\\_\\ **")
		fmt.Println("**  __/ |                                                          **")
		fmt.Println("** |___/                                                           **")
		fmt.Println("=====================================================================")
	}
}

func ShowOptions(s *State) {

}

func main() {
	state := ParseArgs()
	if state != nil {
		resp, err := requests.Get("https://google.com/", nil, nil)

		if err != nil {
			fmt.Println(err)
			return
		}
		// Response body
		fmt.Println(resp.Text())
		// status code
		fmt.Printf("Code: %d\n", resp.StatusCode())
		for k, v := range resp.Headers() {
			// Response Headers
			fmt.Printf("%s: %s\n", k, v)
		}
	}

}
