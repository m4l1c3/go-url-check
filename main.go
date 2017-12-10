package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

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

//URLResponse struct for housing an HTTPResponse
type URLResponse struct {
	StatusCode int
	URL        string
}

//URLResponseSet struct for housing a map of Responses
type URLResponseSet struct {
	set map[URLResponse]bool
}

//Add to ResponseSet
func (set *URLResponseSet) Add(r URLResponse) bool {
	_, found := set.set[r]
	set.set[r] = true
	return !found
}

//AddRange add a list of elements to a set
func (set *URLResponseSet) AddRange(rr []URLResponse) {
	for _, r := range rr {
		set.set[r] = true
	}
}

//Contains Test if an element is in a set
func (set *URLResponseSet) Contains(r URLResponse) bool {
	_, found := set.set[r]
	return found
}

//ContainsAny Check if any of the elements exist
func (set *URLResponseSet) ContainsAny(rr []URLResponse) bool {
	for _, r := range rr {
		if set.set[r] {
			return true
		}
	}
	return false
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
	Responses      URLResponseSet
}

//Add to StringSet
func (set *StringSet) Add(s string) bool {
	_, found := set.set[s]
	set.set[s] = true
	return !found
}

//AddRange add a list of elements to a set
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
		defer outputFile.Close()

		for u := range state.Responses.set {
			write, err := outputFile.WriteString(strconv.Itoa(u.StatusCode) + " " + u.URL + "\n")
			if err != nil {
				color.Red("Error writing file %s\n", err)
			}
			if write > 0 {
				continue
			}
		}
		outputFile.Sync()
		return true, nil
	}
	return false, nil
}

//FileExists check if file exists before trying to open it
func FileExists(wordlist string) bool {
	if _, err := os.Stat(wordlist); err == nil {
		if err != nil {
			fmt.Println(err)
			return false
		}
		return true
	}
	return false
}

//ParseWordlist parses a file containing a list of URLs
func ParseWordlist(state *State, wordlist string) (bool, error) {
	if wordlist != "" {
		if FileExists(wordlist) {
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
		Responses:   URLResponseSet{set: map[URLResponse]bool{}},
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
		color.Cyan("--------------------------------------------------------------")
		color.Cyan("Go URL Check by m4l1c3: https://github.com/m4l1c3/go-url-check")
		color.Cyan("--------------------------------------------------------------")
	}
}

//PrintResponse evaluates response status codes and prints in a diff color
func PrintResponse(url string, statusCode int) {
	switch {
	case statusCode > 499:
		color.Red("[!] %s %d\n", url, statusCode)
		break
	case statusCode > 399:
		color.Magenta("[+] %s %d\n", url, statusCode)
		break
	case statusCode > 299:
		color.Yellow("[+] %s %d\n", url, statusCode)
		break
	default:
		color.Green("[+] %s %d\n", url, statusCode)
		break
	}
}

//PrefixURL with http if missing
func PrefixURL(url string) string {
	if !strings.HasPrefix(url, "http") {
		url = "http://" + url
	}
	return url
}

//Request issue web request for given URL
func Request(url string) requests.Response {
	resp, err := requests.Get(url, nil, nil)

	if err != nil {
		color.Red("Error checking URL: %s\n", err)
	}

	return resp
}

//CheckURL does a GET for a URL
func CheckURL(url string) (URLResponse, error) {
	PrefixURL(url)
	resp := Request(url)
	PrintResponse(url, resp.StatusCode())
	r := URLResponse{
		StatusCode: resp.StatusCode(),
		URL:        url,
	}
	return r, nil
}

//Process runtime config and execute
func Process(state *State) {
	for word := range state.Wordlist.set {
		response, error := CheckURL(word)

		if error != nil {
			fmt.Println("Error getting URL ", error)
		}
		state.Responses.Add(response)
	}

	if state.WriteOutput {
		WriteOutput(state)
	}
}

func main() {
	state := ParseArgs()

	if state != nil {
		Process(state)
	}
}
