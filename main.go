package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	color "github.com/fatih/color"
)

//ProcessorFunc type for delegating operations in state to a function that accepts these parameters
type ProcessorFunc func(entity string, resultChan chan<- URLResponse, state *State)

//PrintResultFunc type for delegating print operations in state to a function that accepts these parameters
type PrintResultFunc func(response *URLResponse)

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
	StatusCode string
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

// Small helper to combine URL with URI then make a
// request to the generated location.
func GoGet(s *State, url, uri, cookie string) (*int, *int64) {
	return MakeRequest(s, url+uri, cookie)
}

// Make a request to the given URL.
func MakeRequest(s *State, fullUrl, cookie string) (*int, *int64) {
	req, err := http.NewRequest("GET", fullUrl, nil)

	if err != nil {
		return nil, nil
	}

	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}

	// if s.UserAgent != "" {
	// 	req.Header.Set("User-Agent", s.UserAgent)
	// }

	// if s.Username != "" {
	// 	req.SetBasicAuth(s.Username, s.Password)
	// }

	resp, err := s.Client.Do(req)

	if err != nil {
		if ue, ok := err.(*url.Error); ok {

			if strings.HasPrefix(ue.Err.Error(), "x509") {
				fmt.Println("[-] Invalid certificate")
			}

			if re, ok := ue.Err.(*RedirectError); ok {
				return &re.StatusCode, nil
			}
		}
		return nil, nil
	}

	defer resp.Body.Close()

	var length *int64 = nil

	if s.IncludeLength {
		length = new(int64)
		if resp.ContentLength <= 0 {
			body, err := ioutil.ReadAll(resp.Body)
			if err == nil {
				*length = int64(utf8.RuneCountInString(string(body)))
			}
		} else {
			*length = resp.ContentLength
		}
	}

	return &resp.StatusCode, length
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
	Verbose        bool
	Threads        int
	OutputFileName string
	Wordlist       StringSet
	StatusCodes    IntSet
	WriteOutput    bool
	Responses      URLResponseSet
	ShouldClose    bool
	SignalChannel  chan os.Signal
	Printer        PrintResultFunc
	Processor      ProcessorFunc
	Client         *http.Client
	FollowRedirect bool
	InsecureSSL    bool
	IncludeLength  bool
}

//RedirectHandler struct for handling http redirects during runtime
type RedirectHandler struct {
	Transport http.RoundTripper
	State     *State
}

//RedirectError struct for status codes in errors
type RedirectError struct {
	StatusCode int
}

//Add to StringSet
func (set *StringSet) Add(s string) bool {
	_, found := set.set[s]
	set.set[s] = true
	return !found
}

// JoinSet join set into single string
func (set *StringSet) JoinSet() string {
	values := []string{}
	for s := range set.set {
		values = append(values, s)
	}
	return strings.Join(values, ",")
}

// JoinSet join the integer set into a single string
func (set *IntSet) JoinSet() string {
	values := []string{}
	for s := range set.set {
		values = append(values, strconv.Itoa(s))
	}
	return strings.Join(values, ",")
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
			write, err := outputFile.WriteString(u.StatusCode + " " + u.URL + "\n")
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

func (e *RedirectError) Error() string {
	return fmt.Sprintf("Redirect code: %d", e.StatusCode)
}

//RoundTrip used to handle following redirects during execution
func (rh *RedirectHandler) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	if rh.State.FollowRedirect {
		return rh.Transport.RoundTrip(req)
	}

	resp, err = rh.Transport.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	switch resp.StatusCode {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther,
		http.StatusNotModified, http.StatusUseProxy, http.StatusTemporaryRedirect:
		return nil, &RedirectError{StatusCode: resp.StatusCode}
	}

	return resp, err
}

func elapsed() func() {
	start := time.Now()
	return func() {
		fmt.Println("")
		color.HiMagenta("Total runtime: %v\n", time.Since(start))
	}
}

//ParseArgs takes runtime arguments and converts into state
func ParseArgs() *State {
	var codes string
	var wordlist string
	var URL string

	valid := true
	s := State{
		StatusCodes:   IntSet{set: map[int]bool{}},
		Wordlist:      StringSet{set: map[string]bool{}},
		Responses:     URLResponseSet{set: map[URLResponse]bool{}},
		Processor:     Check,
		Printer:       PrintResponse,
		IncludeLength: true,
	}

	flag.IntVar(&s.Threads, "t", 10, "Number of concurrent threads")
	flag.BoolVar(&s.Verbose, "v", false, "Verbose output (errors)")
	flag.StringVar(&s.OutputFileName, "o", "", "Output file to write results to (defaults to stdout)")
	flag.StringVar(&URL, "u", "", "The target URL or Domain")
	flag.StringVar(&wordlist, "w", "", "Path to the wordlist")
	flag.StringVar(&codes, "s", "200,204,301,302,307", "Positive status codes")
	flag.BoolVar(&s.FollowRedirect, "r", false, "Follow redirects")
	flag.BoolVar(&s.InsecureSSL, "k", false, "Skip SSL certificate verification")
	flag.Parse()

	if s.Threads < 0 {
		color.Red("[!] Invalid number of threads (-t) %d\n", s.Threads)
		valid = false
	}

	if URL == "" && wordlist == "" {
		color.Red("[!] Unable to start checking both URL (-u): %s and Wordlist are invalid (-w) %s\n", URL, wordlist)
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
		PrintBanner(&s)
		return &s
	}

	return nil
}

//PrintBanner prints the app banner if verbose is true
func PrintBanner(state *State) {
	if state.Verbose {
		color.Cyan("--------------------------------------------------------------")
		PrintOptions(state)
		fmt.Println("")
		color.Cyan("Go URL Check by m4l1c3: https://github.com/m4l1c3/go-url-check")
		color.Cyan("--------------------------------------------------------------")
		fmt.Println("")
	}
}

//PrintOptions print enabled options to user
func PrintOptions(state *State) {
	if state.Verbose {
		if state.Threads > 0 {
			color.Cyan("[+] Number of threads: %d\n", state.Threads)
		}

		if state.OutputFileName != "" {
			color.Cyan("[+] Output file: %s\n", state.OutputFileName)
		}

		// if len(state.Wordlist.set) > 0 {
		// 	color.Cyan("[+] Wordlist: %s\n", state.Wordlist.JoinSet())
		// }

		if len(state.StatusCodes.set) > 0 {
			color.Cyan("[+] StatusCodes: %s\n", state.StatusCodes.JoinSet())
		}
	}
}

//PrintResponse evaluates response status codes and prints in a diff color
func PrintResponse(response *URLResponse) {
	var url = response.URL
	var statusCode = response.StatusCode
	status, err := strconv.Atoi(statusCode[:strings.Index(statusCode, " ")])

	if err == nil {
		switch {
		case status > 499:
			color.Red("[!] %s %s\n", url, statusCode)
			break
		case status > 399:
			color.Magenta("[+] %s %s\n", url, statusCode)
			break
		case status > 299:
			color.Yellow("[+] %s %s\n", url, statusCode)
			break
		default:
			color.Green("[+] %s %s\n", url, statusCode)
			break
		}
	}
}

//PrefixURL with http if missing
func PrefixURL(url string) string {
	if !strings.HasPrefix(url, "http") {
		url = "http://" + url
	}
	return url
}

//Request request using http library
func Request(url string) *http.Response {
	resp, err := http.Get(url)

	if err != nil {
		color.Red("Error checking URL: %s\n", err)
	}
	defer resp.Body.Close()

	return resp
}

//Check does a GET for a URL
func Check(url string, responseChannel chan<- URLResponse, state *State) {
	PrefixURL(url)
	resp := Request(url)

	// PrintResponse(url, resp.Status)
	r := URLResponse{
		StatusCode: resp.Status,
		URL:        url,
	}
	state.Responses.Add(r)
	responseChannel <- r
}

//StartSignalHandler creates a handler to watch for CTRL+C
func StartSignalHandler(state *State) {
	state.SignalChannel = make(chan os.Signal, 1)
	signal.Notify(state.SignalChannel, os.Interrupt)
	go func() {
		for _ = range state.SignalChannel {
			// caught CTRL+C
			if state.Verbose {
				color.Cyan("[!] Keyboard interrupt detected, terminating.")
				state.ShouldClose = true
			}
		}
	}()
}

//Process runtime config and execute
func Process(state *State) {
	// channels used for comms
	urlChannel := make(chan string, state.Threads)
	responseChannel := make(chan URLResponse)

	// Use a wait group for waiting for all threads
	// to finish
	processorGroup := new(sync.WaitGroup)
	processorGroup.Add(state.Threads)
	printerGroup := new(sync.WaitGroup)
	printerGroup.Add(1)

	for i := 0; i < state.Threads; i++ {
		go func() {
			for {
				url := <-urlChannel

				// Did we reach the end? If so break.
				if url == "" {
					break
				}

				// Mode-specific processing
				state.Processor(url, responseChannel, state)
			}

			// Indicate to the wait group that the thread
			// has finished.
			processorGroup.Done()
		}()
	}

	// Single goroutine which handles the results as they
	// appear from the worker threads.
	go func() {
		for r := range responseChannel {
			state.Printer(&r)
		}
		printerGroup.Done()
	}()

	for word := range state.Wordlist.set {
		if state.ShouldClose {
			break
		}
		urlChannel <- word
	}

	close(urlChannel)
	processorGroup.Wait()
	close(responseChannel)
	printerGroup.Wait()

	if state.WriteOutput {
		WriteOutput(state)
	}
}

func main() {
	defer elapsed()()
	state := ParseArgs()

	if state != nil {
		StartSignalHandler(state)
		Process(state)
	}
}
