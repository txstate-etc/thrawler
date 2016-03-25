// THreaded cRAWLER (thrawler)
package main

import (
	"bufio"
	"flag"
	"fmt"
	log "gopkg.in/inconshreveable/log15.v2"
	"io"
	"os"
	"regexp"
	"strings"
)

type FindReplace struct {
	find    *regexp.Regexp
	replace string
}

type Config struct {
	base     *regexp.Regexp
	matchers []FindReplace
}

type ErrNoBaseDomain struct{}

func (e ErrNoBaseDomain) Error() string {
	return "Config file must contain a base domain filter"
}

type ErrConfigFile struct {
	line string
}

func (e ErrConfigFile) Error() string {
	return fmt.Sprintf("Config file issue with following line '%s'", e.line)
}

var configfile string
var threads int
var proxy string
var headers []Header
var wd string

func init() {
	flag.StringVar(&configfile, "conf", "config", "Path to configuration file used to help canonicalize gathered URLs, and to filter by base domain.")
	flag.IntVar(&threads, "threads", 20, "Number of threads used to crawl site.")
	flag.StringVar(&proxy, "proxy", "", "Proxy to send traffic to. Generally a load balancer.")
	flag.Parse()
	// Handle headers separately as multi arguments so that we
	// can allow for multiple headers:
	// +header="h1:v1" +header="h2:v2" ...
	if flag.NArg() > 0 {
		for _, f := range flag.Args() {
			if strings.HasPrefix(f, "+header=") && len(f) > 9 {
				h := strings.SplitN(f[8:], ":", 2)
				if len(h) == 2 {
					h[0] = strings.TrimSpace(h[0])
					h[1] = strings.TrimSpace(h[1])
					if h[0] != "" && h[1] != "" {
						headers = append(headers, Header{Name: h[0], Val: h[1]})
						//					fmt.Printf("Headers added: '%s: %s'\n", h[0], h[1])
					}
				}
			}
		}
	}
	// http://stackoverflow.com/questions/14661511/setting-up-proxy-for-http-client
	if proxy != "" {
		// EX: proxy = "http://gato-public.its.txstate.edu" or "http://localhost:8080"
		os.Setenv("HTTP_PROXY", proxy)
	}
	wd, _ = os.Getwd()
}

func NewConfig(config io.Reader) (Config, error) {
	var base *regexp.Regexp
	var matchers []FindReplace
	scanner := bufio.NewScanner(config)
	for scanner.Scan() {
		conf := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(conf, "#") {
			if base == nil {
				base = regexp.MustCompile(conf)
			} else {
				fr := strings.SplitN(conf, "\t", 2)
				if len(fr) == 2 {
					matchers = append(matchers, FindReplace{find: regexp.MustCompile(fr[0]), replace: fr[1]})
				} else {
					return Config{}, ErrConfigFile{line: conf}
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return Config{}, err
	}
	if base == nil {
		return Config{}, ErrNoBaseDomain{}
	}
	return Config{base: base, matchers: matchers}, nil
}

// Gato specific wrapper for canonicalize
// NOTE: For starters we will not be working with the webcaches
// Convert imagehandler aliases and query strings to direct links to images (except for text requests which imagehandlers generates)
// Imagehandler links thru webcache:
//   text: <img src="http://gato-staging-mainsite2012.its.txstate.edu/cache4fd6ce1ad313e4f1182370ce8ddb9b97/imagehandler/khanmenu/AboutUs.gif?text=About%20Us"/>
//   prod: testing-site-destroyer.its.txstate.edu:
//     http://www.txstate.edu/cachef47227c7c5a65d247bf8bec3ab5cc265/imagehandler/scaler/gato-docs.its.txstate.edu/jcr:67f57bf9-6774-479c-985d-cd0eb105fd77/menu_penguin.jpg?mode=clip&width=336&height=336
//     http://gato-docs.its.txstate.edu/jcr:67f57bf9-6774-479c-985d-cd0eb105fd77/menu_penguin.jpg
//   stag: http://gato-staging-testingsite.its.txstate.edu/
//     http://gato-staging-mainsite2012.its.txstate.edu/cachef47227c7c5a65d247bf8bec3ab5cc265/imagehandler/scaler/gato-staging-docs.its.txstate.edu/jcr:67f57bf9-6774-479c-985d-cd0eb105fd77/menu_penguin.jpg?mode=clip&width=336&height=336
//     gato-staging-docs.its.txstate.edu/jcr:67f57bf9-6774-479c-985d-cd0eb105fd77/menu_penguin.jpg
// ImageHandler links behind webcache or apache
//   prod: http://gato-edit2.its.txstate.edu:8081/testing-site-destroyer
//     http://image-handlers.its.txstate.edu/imageeandler/scaler/gato-edit2.its.txstate.edu:8081/dam/jcr:67f57bf9-6774-479c-985d-cd0eb105fd77/menu_penguin.jpg?mode=clip&width=336&height=336
//     http://gato-edit2.its.txstate.edu:8081/dam/jcr:67f57bf9-6774-479c-985d-cd0eb105fd77/menu_penguin.jpg
//   stag: http://staging.gato-edit-01.tr.txstate.edu:8081/testing-site-destroyer
//     http://image-handlers.its.txstate.edu/imagehandler/scaler/staging.gato-edit-01.tr.txstate.edu:8081/dam/jcr:67f57bf9-6774-479c-985d-cd0eb105fd77/menu_penguin.jpg?mode=clip&width=336&height=336
//     http://staging.gato-edit-01.tr.txstate.edu:8081/dam/jcr:67f57bf9-6774-479c-985d-cd0eb105fd77/menu_penguin.jpg
// vhost replacement:
//  e.g. alkek-library, www.txstate.edu
//func NewCanonicalize(config io.Reader) (func(LinkInfo, string) (LinkInfo, error), error) {
//var reIMGHNDLR = regexp.MustCompile(`^(https?:)?//image-handlers.its.txstate.edu/imagehandler/scaler/([^?]+)`)
//url := reIMGHNDLR.FindStringSubmatch(link)
//if len(url) > 0 {
//	link = url[1] + "//" + url[2]
//}
func NewCanonicalize(config Config) (Canon, error) {
	return func(source LinkInfo, link string) (LinkInfo, error) {
		// Canonicalize
		li, err := canonicalize(source, link)
		if err != nil {
			return LinkInfo{}, err
		}
		// Normalize Image Handler links
		link = li.GetUrl()
		for _, matcher := range config.matchers {
			link = matcher.find.ReplaceAllString(link, matcher.replace)
		}
		li, err = SplitUrl(li, link)
		if err != nil {
			return li, err
		}
		// Return error if does not match base domain to be crawled
		if !config.base.MatchString(li.String()) {
			return LinkInfo{}, ErrNotBaseDomain{url: li.String()}
		}
		return li, nil
	}, nil
}

func main() {
	if !strings.HasPrefix(configfile, "/") {
		configfile = wd + "/" + configfile
	}
	f, err := os.Open(configfile)
	if err != nil {
		panic("Error opening '" + configfile + "' configuration file: " + err.Error())
	}
	conf, err := NewConfig(f)
	if err != nil {
		f.Close()
		panic("Error processing config file: " + err.Error())
	}
	canon, err := NewCanonicalize(conf)
	f.Close()
	if err != nil {
		panic("Error processing configuration file: " + err.Error())
	}
	mainlog := log.New("app", "thrawler")
	mainlog.SetHandler(
		log.LvlFilterHandler(
			log.LvlDebug,
			log.StreamHandler(os.Stdout, log.JsonFormat())))
	var sites []string
	in := bufio.NewScanner(os.Stdin)
	for in.Scan() {
		sites = append(sites, in.Text())
	}
	if err := in.Err(); err != nil {
		panic("Error reading site list from standard input:" + err.Error())
	}
	Run(mainlog, threads, StartHtmlFilterLinks(threads, headers, canon, sites))
}
