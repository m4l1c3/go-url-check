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
type ProcessorFunc func(entity string, state *State) []URLResponse

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
	Responses      []URLResponse
	ShouldClose    bool
	SignalChannel  chan os.Signal
	// Printer        PrintResultFunc
	Processor      ProcessorFunc
	Client         *http.Client
	FollowRedirect bool
	InsecureSSL    bool
	IncludeLength  bool
	Throttle       bool
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
			color.HiRed("[!] Unable to write to %s, falling back to stdout.\n", state.OutputFileName)
			return false, err
		}
		defer outputFile.Close()

		for u := range state.Responses {
			write, err := outputFile.WriteString(fmt.Sprintf("%s %s\n", state.Responses[u].URL, state.Responses[u].StatusCode))
			if err != nil {
				color.HiRed("Error writing file %s\n", err)
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
	var responses []URLResponse

	valid := true
	s := State{
		StatusCodes: IntSet{set: map[int]bool{}},
		Wordlist:    StringSet{set: map[string]bool{}},
		Responses:   responses,
		Processor:   Check,
		// Printer:       PrintResponse,
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
	flag.BoolVar(&s.Throttle, "-r", false, "Enable throttling or rate limiting")
	flag.Parse()

	if s.Threads < 0 {
		color.HiRed("[!] Invalid number of threads (-t) %d\n", s.Threads)
		valid = false
	}

	if URL == "" && wordlist == "" {
		color.HiRed("[!] Unable to start checking both URL (-u): %s and Wordlist are invalid (-w) %s\n", URL, wordlist)
		valid = false
	}

	if *&s.OutputFileName != "" {
		s.WriteOutput = true
	}

	if wordlist != "" {
		ParseWordlist(&s, wordlist)
		if len(s.Wordlist.set) < 1 {
			valid = false
			color.HiRed("[!] Unable to start checking unable to parse wordlist (-w) %s\n", wordlist)
		}
	} else {
		s.Wordlist.Add(URL)
	}

	if valid {
		PrintBanner(&s)
		return &s
	}

	return nil
}

//func PrintRuler prints a horizontal ruler
func PrintRuler() {
	color.HiCyan("-----------------------------------------------------------------------")
}

//PrintBanner prints the app banner if verbose is true
func PrintBanner(state *State) {
	if state.Verbose {
		PrintRuler()
		color.HiCyan("--               _               _                  _                --")
		color.HiCyan("--              | |             | |                | |               --")
		color.HiCyan("--  _   _  _ __ | | ______  ___ | |__    ___   ___ | | __ ___  _ __  --")
		color.HiCyan("-- | | | || '__|| ||______|/ __|| '_ \\  / _ \\ / __|| |/ // _ \\| '__| --")
		color.HiCyan("-- | |_| || |   | |       | (__ | | | ||  __/| (__ |   <|  __/| |    --")
		color.HiCyan("--  \\__,_||_|   |_|        \\___||_| |_| \\___| \\___||_|\\_\\\\___||_|    --")
		color.HiCyan("--                                                                   --")
		color.HiCyan("-- Go URL Check by m4l1c3: https://github.com/m4l1c3/go-url-check    --")
		PrintRuler()
		fmt.Println("")
		PrintOptions(state)
		fmt.Println("")
	}
}

//PrintOptions print enabled options to user
func PrintOptions(state *State) {
	if state.Verbose {
		if state.Threads > 0 {
			color.HiCyan("[+] Number of threads: %d\n", state.Threads)
		}

		if state.OutputFileName != "" {
			color.HiCyan("[+] Output file: %s\n", state.OutputFileName)
		}

		// if len(state.Wordlist.set) > 0 {
		// 	color.HiCyan("[+] Wordlist: %s\n", state.Wordlist.JoinSet())
		// }

		if len(state.StatusCodes.set) > 0 {
			color.HiCyan("-- [+] StatusCodes: %s\n", state.StatusCodes.JoinSet())
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
			color.HiRed("[!] %s %s\n", url, statusCode)
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
		url = "https://" + url
	}
	return url
}

//Request request using http library
func Request(url string) *http.Response {
	resp, err := http.Get(url)

	if err != nil {
		return nil
	}

	if err == nil {
		defer resp.Body.Close()
	}

	return resp
}

//Check does a GET for a URL
func Check(url string, state *State) []URLResponse {
	url = PrefixURL(url)
	resp := Request(url)
	var r URLResponse

	if resp != nil {
		r = URLResponse{
			StatusCode: resp.Status,
			URL:        url,
		}
		PrintResponse(&r)
		return append(state.Responses, r)
	}
	return state.Responses

	// responseChannel <- r
}

//StartSignalHandler creates a handler to watch for CTRL+C
func StartSignalHandler(state *State) {
	state.SignalChannel = make(chan os.Signal, 1)
	signal.Notify(state.SignalChannel, os.Interrupt)
	go func() {
		for _ = range state.SignalChannel {
			// caught CTRL+C
			if state.Verbose {
				color.HiCyan("[!] Keyboard interrupt detected, terminating.")
				state.ShouldClose = true
			}
		}
	}()
}

//Process runtime config and execute
func Process(state *State) {
	// channels used for comms
	urlChannel := make(chan string, state.Threads)
	// responseChannel := make(chan URLResponse)

	// Use a wait group for waiting for all threads
	// to finish
	processorGroup := new(sync.WaitGroup)
	processorGroup.Add(state.Threads)
	// printerGroup := new(sync.WaitGroup)
	// printerGroup.Add(1)

	for i := 0; i < state.Threads; i++ {
		go func() {
			for {
				url := <-urlChannel

				// Did we reach the end? If so break.
				if url == "" {
					break
				}

				// Mode-specific processing
				state.Responses = state.Processor(url, state)
			}

			// Indicate to the wait group that the thread
			// has finished.
			processorGroup.Done()
		}()
	}

	// Single goroutine which handles the results as they
	// appear from the worker threads.
	// go func() {
	// 	for r := range responseChannel {
	// 		state.Printer(&r)
	// 	}
	// 	printerGroup.Done()
	// }()
	var i int
	sleepTime := time.Duration(5) * time.Second

	for word := range state.Wordlist.set {
		if i > 100 && i%100 == 0 {
			if state.Verbose {
				color.HiGreen("%d out of %d URLs checked.", i, len(state.Wordlist.set))
			}
			if state.Throttle {
				color.HiGreen("Pausing for %d ... seconds\n", sleepTime/time.Second)
				time.Sleep(sleepTime)
			}
		}

		if state.ShouldClose {
			break
		}
		urlChannel <- word
		i++
	}

	close(urlChannel)
	processorGroup.Wait()
	// close(responseChannel)
	// printerGroup.Wait()

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
