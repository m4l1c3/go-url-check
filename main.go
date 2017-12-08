package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"

	color "github.com/fatih/color"
	requests "github.com/hiroakis/go-requests"
)

//IntSet struct for map of integers
type IntSet struct {
	set map[int]bool
}

//StringSet struct for map of strings
type StringSet struct {
	set map[string]bool
}

//State struct for running state
type State struct {
	ProxyURL       string
	Verbose        bool
	Threads        int
	OutputFileName string
	Wordlist       StringSet
	StatusCodes    IntSet
	WriteOutput    bool
}

//Add to StringSet
func (set *StringSet) Add(s string) bool {
	_, found := set.set[s]
	set.set[s] = true
	return !found
}

//AddRand add a list of elements to a set
func (set *StringSet) AddRange(ss []string) {
	for _, s := range ss {
		set.set[s] = true
	}
}

//Contains Test if an element is in a set
func (set *StringSet) Contains(s string) bool {
	_, found := set.set[s]
	return found
}

//ContainsAny Check if any of the elements exist
func (set *StringSet) ContainsAny(ss []string) bool {
	for _, s := range ss {
		if set.set[s] {
			return true
		}
	}
	return false
}

//Add an element to a set
func (set *IntSet) Add(i int) bool {
	_, found := set.set[i]
	set.set[i] = true
	return !found
}

//Contains Test if an element is in a set
func (set *IntSet) Contains(i int) bool {
	_, found := set.set[i]
	return found
}

//WriteOutput writes program output to a file when configured to do so
func WriteOutput(state *State) (bool, error) {
	if state.OutputFileName != "" {
		outputFile, err := os.Create(state.OutputFileName)
		if err != nil {
			color.Red("[!] Unable to write to %s, falling back to stdout.\n", state.OutputFileName)
			return false, err
		}
		if outputFile != nil {
			return true, nil
		}
	}
	return false, nil
}

//ParseWordlist parses a file containing a list of URLs
func ParseWordlist(state *State, wordlist string) (bool, error) {
	if wordlist != "" {
		if _, err := os.Stat(wordlist); err == nil {
			if err != nil {
				fmt.Println(err)
				return false, err
			}

			file, error := os.Open(wordlist)

			if error != nil {
				return false, error
			}
			defer file.Close()

			var lines []string
			scanner := bufio.NewScanner(file)

			for scanner.Scan() {
				lines = append(lines, scanner.Text())
			}
			state.Wordlist.AddRange(lines)
			return true, nil
		}
	}
	return false, nil
}

//ParseArgs takes runtime arguments and converts into state
func ParseArgs() *State {
	var codes string
	var wordlist string
	var URL string
	valid := true
	s := State{
		StatusCodes: IntSet{set: map[int]bool{}},
		Wordlist:    StringSet{set: map[string]bool{}},
	}

	flag.IntVar(&s.Threads, "t", 10, "Number of concurrent threads")
	flag.BoolVar(&s.Verbose, "v", false, "Verbose output (errors)")
	flag.StringVar(&s.ProxyURL, "p", "", "Proxy to use for requests [http(s)://host:port]")
	flag.StringVar(&s.OutputFileName, "o", "", "Output file to write results to (defaults to stdout)")
	flag.StringVar(&URL, "u", "", "The target URL or Domain")
	flag.StringVar(&wordlist, "w", "", "Path to the wordlist")
	flag.StringVar(&codes, "s", "200,204,301,302,307", "Positive status codes")
	flag.Parse()

	if s.Threads < 0 {
		fmt.Println("[!] Invalid number of threads (-t)", s.Threads)
		valid = false
	}

	if URL == "" && wordlist == "" {
		fmt.Println("[!] Unable to start checking both URL (-u) and Wordlist are invalid (-w)", URL, wordlist)
		valid = false
	}

	if *&s.OutputFileName != "" {
		s.WriteOutput = true
	}

	if wordlist != "" {
		ParseWordlist(&s, wordlist)
	} else {
		s.Wordlist.Add(URL)
	}

	if valid {
		Banner(&s)
		return &s
	}

	return nil
}

//Banner prints the app banner if verbose is true
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

//checkStatusCode evaluates response status codes and prints in a diff color
func checkStatusCode(url string, statusCode int) {
	switch {
	case statusCode > 499:
		color.Red("[!] %s %d\n", url, statusCode)
		break
	case statusCode > 399:
		color.Magenta("[!] %s %d\n", url, statusCode)
		break
	case statusCode > 299:
		color.Yellow("[!] %s %d\n", url, statusCode)
		break
	default:
		color.Green("[!] %s %d\n", url, statusCode)
		break
	}
}

//checkURL does a GET for a URL
func checkURL(url string) (int, error) {
	resp, err := requests.Get(url, nil, nil)

	if err != nil {
		return 0, err
	}

	checkStatusCode(url, resp.StatusCode())
	return resp.StatusCode(), nil
}

//Process runtime config and execute
func Process(state *State) {
	for word := range state.Wordlist.set {
		responseStatus, error := checkURL(word)

		if error != nil {
			fmt.Println("Error getting URL ", error)
		}
		state.StatusCodes.Add(responseStatus)
	}
}

func main() {
	state := ParseArgs()

	if state != nil {
		Process(state)
	}
}
