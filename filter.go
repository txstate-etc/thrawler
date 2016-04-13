// Filters html and css pages for links
package main

import (
	"bytes"
	"fmt"
	"golang.org/x/net/html"
	log "gopkg.in/inconshreveable/log15.v2"
	"io"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
)

type FilterType int

const (
	SKIPFILTER FilterType = iota
	EXISTFILTER
	HTMLFILTER
	CSSFILTER
)

type LinkContent struct {
	Url    string
	Tag    string
	Filter FilterType
}

// NOTE: Use of reCSS regular expresson is required to
//   filter out comments; so as to not inadvertently grab
//   commented out urls.
// WARN: Use of reCSS regexp will not be able to determine
//   bad css. e.g. unclosed comments. Will require a
//   css parser for that.
// NOTE: Use of reCSSURL regular expression is used to
//   filter out urls that match a url("...") pattern.
// WARN: No backreferences are available in regexp module;
//   so suboptimal approach is to treat quotes in reCSSURL
//   as optional.
var reCSSURL = regexp.MustCompile(`\burl\(["']?([^"')]+)`)
var reCSS = regexp.MustCompile(`(?s)(?:/\*.*?\*/)?((?:[^/]|/[^*])*)`)
var reSRCSET = regexp.MustCompile(`\b(https?://[^ ,]+)`)
var reFullUrl = regexp.MustCompile(`^https?://`)
var reSplitUrl = regexp.MustCompile(`^(https?)://([^/]+)(/[^?#]*)?(\?[^#]*)?(#.*)?$`)
var reProtocol = regexp.MustCompile(`^[^/:]+:`)

type ErrFragmentUrl struct {
	hash string
}

func (e ErrFragmentUrl) Error() string {
	return fmt.Sprintf("Hash Only URL: '%s'", e.hash)
}

type ErrMalformUrl struct {
	url string
}

func (e ErrMalformUrl) Error() string {
	return fmt.Sprintf("Malformed URL: '%s'", e.url)
}

type ErrMalformHtml struct {
	err string
}

func (e ErrMalformHtml) Error() string {
	return fmt.Sprintf("Malformed or incomplete html '%s'", e.err)
}

type ErrProtocol struct {
	proto string
}

func (e ErrProtocol) Error() string {
	return fmt.Sprintf("Not an http(s) protocol: '%s'", e.proto)
}

type ErrNotBaseDomain struct {
	url string
}

func (e ErrNotBaseDomain) Error() string {
	return fmt.Sprintf("Domain does not match test base: '%s'", e.url)
}

type ErrEmptyUrl struct{}

func (e ErrEmptyUrl) Error() string {
	return "Empty URL"
}

type ErrRedirectTtlExceeded struct{}

func (e ErrRedirectTtlExceeded) Error() string {
	return "Redirect exceeded time to live (TTL) quota."
}

type ErrNon200Status struct {
	status int
}

func (e ErrNon200Status) Error() string {
	return fmt.Sprintf("Non-200 status code: '%d'", e.status)
}

type ErrNotHtmlContent struct {
	content string
}

func (e ErrNotHtmlContent) Error() string {
	return fmt.Sprintf("Not html Content-Type: '%s'", e.content)
}

type Header struct {
	Name string
	Val  string
}

type Env map[string]int

type Envs struct {
	envs    []Env
	headers []Header
	canon   Canon
	crawl   bool
}

func StartHtmlFilterLinks(envn int, headers []Header, canon Canon, crawl bool, urls []string) (pis []ProcInfo) {
	es := make([]Env, envn)
	for i := 0; i < envn; i++ {
		es[i] = make(Env)
	}
	var envs = Envs{envs: es, headers: headers, canon: canon, crawl: crawl}
	for _, url := range urls {
		if reFullUrl.MatchString(url) {
			li, err := canon(LinkInfo{}, url)
			if err == nil {
				if !crawl {
					es[ChannelPicker(li.String(), envn)][li.String()] = -1
				}
				pis = append(pis, ProcInfo(HtmlFilterLink{LinkInfo: li, Envs: envs}))
			}
		}
	}
	return
}

type LinkInfo struct {
	Protocol string
	Host     string
	Path     string
	Query    string
	Fragment string
	Tag      string
	Initial  string
	FullUrl  string
}

// Remove Query and Fragments from url
func (li LinkInfo) String() string {
	if li.Protocol != "" && li.Host != "" {
		return li.Protocol + "://" + li.Host + li.Path
	}
	return ""
}

func (li LinkInfo) GetUrl() string {
	return li.FullUrl
}

type Links struct {
	LinkInfo
	Envs
	source string
	list   []LinkInfo
	htm    *html.Tokenizer
	log    log.Logger
}

// A links require use of MIME to determine what
// content-type flag should be returned. Not sure
// how strict we wish to follow MIME designations.
// A tags with an htm/html/php/asp or lack of
// an .xxxx? extension should return
// "content-type: text/html", may contain
// "...;charset=utf-8", and are the only
// extensions that are expected to return html
// and should be filtered for links and
// considered a failure if they do not have
// "content-type: text/html" header. All other
// links should be classified as an ExistOnlyLink
type HtmlFilterLink struct {
	LinkInfo
	Envs
	source string
}

func (link HtmlFilterLink) Fn(l log.Logger, i int) []ProcInfo {
	ls := &Links{LinkInfo: link.LinkInfo, Envs: link.Envs, source: link.source, log: l}
	return ls.Request(i, HTMLFILTER)
}

// link tag and rel="stylesheet" href="<url>"
// attributes should return "content-type: text/css"
// header that should be filtered for image links.
type CssFilterLink struct {
	LinkInfo
	Envs
	source string
}

func (link CssFilterLink) Fn(l log.Logger, i int) []ProcInfo {
	ls := &Links{LinkInfo: link.LinkInfo, Envs: link.Envs, source: link.source, log: l}
	return ls.Request(i, CSSFILTER)
}

// We should use only HEAD requests for the following links
// to determine if they exist.
// 1) A tags without htm/html/php/asp or lack of an extension
// 2) Image links:
//   a) img tag src or srcset attributes,
//   b) style css url() property
//   c) link tag with rel="icon" or rel="shortcut icon" attribute
//   Such links should return the appropriate
//   "content-type: image/<png/jpg/gif/x-ico...>"
//   headers.
// 3) Script link from script tag type="text/javascript"
//   attribute should also return the
//   "content-type:application/javascript" header
type ExistOnlyLink struct {
	LinkInfo
	Envs
	source string
}

func (link ExistOnlyLink) Fn(l log.Logger, i int) []ProcInfo {
	ls := &Links{LinkInfo: link.LinkInfo, Envs: link.Envs, source: link.source, log: l}
	return ls.Request(i, EXISTFILTER)
}

// Currently no redirects are taken
// TODO: submit redirect location header
// information as new requests with
// decrementing TTL as we do not grow
// via variable:
// https://golang.org/src/net/http/client.go
func redirectPolicyFunc(_ *http.Request, _ []*http.Request) error {
	return ErrRedirectTtlExceeded{}
}

// Request method handles all logging of results
// and as a result handles all errors as well.
//func Request(l log.Logger, i int, e Envs, src string, li LinkInfo, filter func(log.Logger, io.Reader, Envs, LinkInfo, func(LinkInfo, string) (LinkInfo, error)) ([]ProcInfo, error)) []ProcInfo {
func (ls *Links) Request(i int, f FilterType) []ProcInfo {
	var pis = []ProcInfo{}
	stat, ok := ls.envs[i][ls.String()]
	if !ls.crawl { // Non-crawling modified behavior
		if ok && stat == -1 { // First time handling submitted page; always parse submitted pages.
			ok = false
			f = HTMLFILTER
		} else if !ok && f == HTMLFILTER { // Non-submitted normally to be parsed pages; do not parse
			f = EXISTFILTER
		}
		// let rest fall through such as:
		// - Submitted pages that have already been parsed
		// - Non-submitted pages that normally do not require parsing
	}
	method := "HEAD"
	if f == HTMLFILTER || f == CSSFILTER {
		method = "GET"
	}
	if ok { // Only log pages that have already been handled
		ls.log.Info("req", "src", ls.source, "tag", ls.Tag, "url", ls.String(), "initial", ls.Initial, "err", "", "code", stat, "type", method, "net", false)
		return pis
	}
	req, err := http.NewRequest(method, ls.String(), nil)
	if err != nil {
		ls.envs[i][ls.String()] = 0
		ls.log.Info("req", "src", ls.source, "tag", ls.Tag, "url", ls.String(), "initial", ls.Initial, "err", err.Error(), "code", 0, "type", method, "net", true)
		return pis
	}

	// Magnolia CMS gzip responses have a 2GB limit;
	// so do not accept gzip content to avoid issue.
	// WARNING: Also it seems that apache is not
	// filtering some of the mj marked links when
	// compression is used. TODO: Verify this issue.
	tr := &http.Transport{
		DisableCompression: true,
	}

	if ls.source != "" {
		req.Header.Add("referer", ls.source)
	}
	if len(ls.headers) > 0 {
		for _, h := range ls.headers {
			req.Header.Add(h.Name, h.Val)
		}
	}
	client := &http.Client{
		CheckRedirect: redirectPolicyFunc,
		Transport:     tr,
	}

	res, err := client.Do(req)
	if res != nil {
		defer res.Body.Close()
	}

	if res.StatusCode == -1 {
		ls.envs[i][ls.String()] = 0
	} else {
		ls.envs[i][ls.String()] = res.StatusCode
	}
	if err != nil || res.StatusCode != 200 {
		// could be redirect error ErrRedirectTtlExceeded
		// which loging should handle
		if err != nil {
			ls.log.Info("req", "src", ls.source, "tag", ls.Tag, "url", ls.String(), "initial", ls.Initial, "err", err.Error(), "code", res.StatusCode, "type", method, "net", true)
		} else {
			ls.log.Info("req", "src", ls.source, "tag", ls.Tag, "url", ls.String(), "initial", ls.Initial, "err", "", "code", res.StatusCode, "type", method, "net", true)
		}
		return pis
	}

	if f == EXISTFILTER { // Implies HEAD Request Method
		ls.log.Info("req", "src", ls.source, "tag", ls.Tag, "url", ls.String(), "initial", ls.Initial, "err", "", "code", res.StatusCode, "type", method, "net", true)
	} else if f != SKIPFILTER {
		var err error
		if f == HTMLFILTER { // Implies GET Request Method with HTML Filter
			// Only process response responses with "Content-Type: text/html;charset=UTF-8"
			// As Gato seems to include links to images in a tags.
			content := res.Header.Get("Content-Type")
			if strings.HasPrefix(content, "text/html") {
				pis, err = ls.FilterHtml(res.Body)
			} else {
				err = ErrNotHtmlContent{content: content}
			}
		} else { // Implies GET Request Method with CSS Filter
			// NOTE: html document specifies
			// <link rel=stylesheet type=text/css ...>;
			// so checking the response Content-Type
			// header is redundant and not required
			// for browsers.
			pis, err = ls.FilterCss(res.Body)
		}
		if err != nil {
			ls.log.Info("req", "src", ls.source, "tag", ls.Tag, "url", ls.String(), "initial", ls.Initial, "err", err.Error(), "code", res.StatusCode, "type", method, "net", true)
		} else {
			ls.log.Info("req", "src", ls.source, "tag", ls.Tag, "url", ls.String(), "initial", ls.Initial, "err", "", "code", res.StatusCode, "type", method, "net", true)
		}
	}
	return pis
}

// ---- Filter for URLs
// TODO: Verify relative links in CSS files are based off the parent page
//   they where loaded in and not relative to the location of the CSS file.
//   If they can be relative to the parent page they were loaded in, then
//   we need to push css link along with html parent onto CSS thread that
//   manages a cache of relative links for CSS pages. CSS thread performs
//   the following:
//   If CSS page relative links are not cached:
//     1) push images with full URLs and absolute paths onto the queue.
//     2) generate cache of relative links for CSS file full url.
//   If they are cached then:
//     1) Pull relative links from cache
//   Finally process any relative links with respect to source page
func (ls *Links) FilterHtml(doc io.Reader) ([]ProcInfo, error) {
	// The HTML parser does not handle Reader interfaces; so we
	// must first turned the reader into a string, thus
	// requiring us to retrieve the full page before we can
	// start parsing.
	var procs []ProcInfo
	ls.htm = html.NewTokenizer(doc)
	for {
		if tokenType := ls.htm.Next(); tokenType == html.ErrorToken {
			if err := ls.htm.Err(); err == io.EOF {
				return procs, nil
			} else {
				return procs, ErrMalformHtml{err: err.Error()}
			}
		} else {
			switch tokenType {
			case html.StartTagToken, html.SelfClosingTagToken:
				tag, moreAttr := ls.htm.TagName()
				var list []LinkContent
				if bytes.Equal(tag, []byte("style")) {
					// <style type="text/css"> ExistOnlyLink
					text := ls.htm.Text()
					links := parseCss(string(text))
					for _, l := range links {
						list = append(list, LinkContent{Url: l, Tag: "html/style", Filter: EXISTFILTER})
					}
				} else if moreAttr {
					var attr []byte
					var val []byte
					var css string
					var text_css bool
					for moreAttr {
						attr, val, moreAttr = ls.htm.TagAttr()
						switch {
						case bytes.Equal(attr, []byte("type")):
							// <link rel="stylesheet" type="text/css" href="theme.css">
							//   if type="text/css"  -> CssFilterLink
							//   else -> ExistOnlyLink
							if bytes.Equal(tag, []byte("link")) {
								text_css = true
							}
							// <script type="text/javascript" src="....js" ExistOnlyLink
						case bytes.Equal(attr, []byte("href")):
							// <a href="..."> HtmlFilterLink
							// <link rel="stylesheet" type="text/css" href="theme.css">
							//   if type="text/css"  -> CssFilterLink
							//   else -> ExistOnlyLink
							if bytes.Equal(tag, []byte("a")) {
								list = append(list, LinkContent{Url: string(val), Tag: "html/a", Filter: HTMLFILTER})
							} else if bytes.Equal(tag, []byte("link")) {
								css = string(val)
							}
						case bytes.Equal(attr, []byte("src")):
							// <iframe src="..."></iframe> HtmlFilterLink
							// <img src="http://ih.com/b.png" ExistOnlyLink
							// <script type="text/javascript" src="....js" ExistOnlyLink
							if bytes.Equal(tag, []byte("iframe")) {
								list = append(list, LinkContent{Url: string(val), Tag: "html/iframe", Filter: HTMLFILTER})
							} else {
								list = append(list, LinkContent{Url: string(val), Tag: "html/" + string(tag), Filter: EXISTFILTER})
							}
						case bytes.Equal(attr, []byte("srcset")):
							// <img srcset="http://ih.com/b.png?... 960w, http://ih.com/b.png?... 480w"> ExistOnlyLink
							if bytes.Equal(tag, []byte("img")) {
								links := reSRCSET.FindAllStringSubmatch(string(val), -1)
								for _, link := range links {
									list = append(list, LinkContent{Url: link[1], Tag: "html/srcset", Filter: EXISTFILTER})
								}
							}
						case bytes.Equal(attr, []byte("action")):
							// <form action="submit.htm" method="post"> Skip/Log only
							if bytes.Equal(tag, []byte("form")) {
								list = append(list, LinkContent{Url: string(val), Tag: "html/form", Filter: SKIPFILTER})
							}
						case bytes.Equal(attr, []byte("style")):
							// a img form iframe ExistOnlyLink
							links := parseCss(string(val))
							for _, l := range links {
								list = append(list, LinkContent{Url: l, Tag: "html/" + string(tag) + "style", Filter: EXISTFILTER})
							}
						}
					}
					if text_css && css != "" {
						list = append(list, LinkContent{Url: css, Tag: "html/link", Filter: CSSFILTER})
					} else if css != "" {
						list = append(list, LinkContent{Url: css, Tag: "html/link", Filter: EXISTFILTER})
					}
				}
				for _, lc := range list {
					li, err := ls.canon(ls.LinkInfo, lc.Url)
					if err != nil {
						ls.log.Info("req", "src", ls.String(), "tag", lc.Tag, "url", lc.Url, "initial", lc.Url, "err", err.Error(), "code", 0, "type", "", "net", false)
					} else {
						li.Tag = lc.Tag
						switch lc.Filter {
						case SKIPFILTER:
							ls.log.Info("req", "src", ls.String(), "tag", lc.Tag, "url", li.String(), "initial", lc.Url, "err", "", "code", 0, "type", "SKIP", "net", false)
						case EXISTFILTER:
							procs = append(procs, ProcInfo(ExistOnlyLink{LinkInfo: li, source: ls.String(), Envs: ls.Envs}))
						case HTMLFILTER:
							procs = append(procs, ProcInfo(HtmlFilterLink{LinkInfo: li, source: ls.String(), Envs: ls.Envs}))
						case CSSFILTER:
							procs = append(procs, ProcInfo(CssFilterLink{LinkInfo: li, source: ls.String(), Envs: ls.Envs}))
						}
					}
				}
			}
		}
	}
}

// FilterCss may be used to scrape for links from CSS files matching url("...") patterns:
//   a:hover
//   {
//     text-decoration:none;
//     background: transparent url("http://gato-docs.its.txstate.edu/xiphophorus-genetic-stock-center/images/bg/logo-b.png") top center no-repeat fixed;
//   }
func (ls *Links) FilterCss(body io.Reader) ([]ProcInfo, error) {
	var procs []ProcInfo
	c, err := ioutil.ReadAll(body)
	if err != nil {
		return procs, err
	}
	links := parseCss(string(c))
	for _, link := range links {
		if li, err := ls.canon(ls.LinkInfo, link); err == nil {
			li.Tag = "css/url"
			procs = append(procs, ProcInfo(ExistOnlyLink{LinkInfo: li, source: ls.String(), Envs: ls.Envs}))
		} else {
			ls.log.Info("request", "source", ls.String(), "tag", "css/url", "url", link, "error", err.Error(), "status", 0, "method", "")
		}
	}
	return procs, nil
}

// parseCSS may be used to scrape for links from style tags and attributes with CSS content for matching url("...") patterns:
// <a
//   href="/mjdf38i3tv0b56vz/xiphophorus-genetic-stock-center/about.html"
//   class="ddmenu-menubaritem"
//   style="background: url(http://gato-staging-mainsite2012.its.txstate.edu/cache4fd6ce1ad313e4f1182370ce8ddb9b97/imagehandler/khanmenuactive/AboutUs.gif?text=About%20Us)"
// >...</a>
func parseCss(body string) (links []string) {
	csss := reCSS.FindAllStringSubmatch(body, -1)
	for _, css := range csss {
		if css[1] != "" {
			urls := reCSSURL.FindAllStringSubmatch(css[1], -1)
			for _, url := range urls {
				links = append(links, url[1])
			}
		}
	}
	return
}

// ---------------------------------------------------------------------------
// NOTE: Can add configuration options for Generating canonical URLs, by
// encapulating the Canonicalization function; This allows for this
// functionality to be expand upon and thus applied to specific sites
// by tailoring the resulting fuction to their needs.
type Canon func(LinkInfo, string) (LinkInfo, error)

// ---- URL Canonicalization
// Requirements for Generating canonical URLs:
// 1) Condense path by adjusting ../../ relative positioning from left to right
// 2) Drop unwanted URLs
// 3) Add proper protocol
func canonicalize(source LinkInfo, link string) (LinkInfo, error) {
	var linkinfo LinkInfo
	link = strings.TrimSpace(link)
	linkinfo.Initial = link
	if link == "" {
		// Skip empty links (link=="")
		return linkinfo, ErrEmptyUrl{}
	} else if proto := reProtocol.FindString(link); proto != "" && !strings.HasPrefix(proto, "http") {
		// can only process links with http(s) protocols
		return linkinfo, ErrProtocol{proto: proto}
	} else if strings.HasPrefix(link, "#") {
		// Skip anchors (link=="#...")
		return linkinfo, ErrFragmentUrl{hash: link}
	} else if strings.HasPrefix(link, ":") {
		// No link should start with colon (:)
		return linkinfo, ErrMalformUrl{url: link}
	} else if !reFullUrl.MatchString(link) {
		// Not full URL so fill in missing pieces using source
		if strings.HasPrefix(link, "//") {
			// Full URL excluding protocol
			// Add protocol (used by parent) if missing from links such as "//gato-staging-mainsite2012.its.txstate.edu"
			link = source.Protocol + ":" + link
		} else if strings.HasPrefix(link, "/") {
			// Absolute URL
			link = source.Protocol + "://" + source.Host + link
		} else {
			// Relative URL
			// Turn relative links into absolute links
			// NOTE: need to remove source.Path filename from end before appending link
			link = source.Protocol + "://" + source.Host + basePath(source.Path) + "/" + link
		}
	}
	return SplitUrl(linkinfo, link)
}

// path such as /mjdf38i3tv0b56vz/.resources/gato-template-txstate2015/css/txstate2015.compiled.css
// returns /mjdf38i3tv0b56vz/.resources/gato-template-txstate2015/css
func basePath(p string) string {
	if i := strings.LastIndex(p, "/"); i > 0 {
		return p[:i]
	}
	return ""
}

// if split, err := splitUrl(absolute); err != nil {
// LinkInfo{RefType: reftype, Protocol: split[1], Host: split[2], Path: split[3][:strings.LastIndex(split[3], "/")+1]}
// WARN: Path: split[3][:strings.LastIndex(split[3], "/")+1]
//   leaves some bytes on the end that cannot be freed up until
//   parent is GC. This generally should be small.
// splitUrl breaks a url up into the following 6 sections:
// Example: "http://test.com:8080/sub/pages/?test=1&case=test#fragment-marker"
// [0] http://test.com:8080/sub/pages/?test=1&case=test#fragment-marker
// [1] http              = protocol
// [2] test.com:8080     = host:port
// [3] /sub/pages/       = path
// [4] ?test=1&case=test = query
// [5] #fragment-marker  = fragment
// If a section is not present then it will be represented by an empty string "".
// If the url does not match the reSplitUrl regular expression,
// we assume it is invalid and return a malformed url error.
func SplitUrl(linkinfo LinkInfo, link string) (LinkInfo, error) {
	split := reSplitUrl.FindStringSubmatch(link)
	if len(split) == 0 {
		return linkinfo, ErrMalformUrl{url: link}
	}

	linkinfo.Protocol = split[1]
	linkinfo.Host = split[2]
	linkinfo.Path = condensePath(split[3])
	linkinfo.Query = split[4]
	linkinfo.Fragment = split[5]
	linkinfo.FullUrl = link
	return linkinfo, nil
}

// condensePath removes double slashes "//" and
// normalizes the path by removing "<previous path>/.." sections
// TODO: Should also remove "./" same path sections
func condensePath(path string) string {
	var p []string
	var s string
	if strings.HasSuffix(path, "/") {
		s = "/"
	}
	ps := strings.Split(path, "/")
	for _, v := range ps {
		if v != ".." && v != "." && v != "" {
			p = append(p, v)
		} else if v == ".." && len(p) != 0 {
			p = p[:len(p)-1]
		} // treat rest of cases as v == ""
	}
	if len(p) == 0 {
		return "/"
	} else {
		return fmt.Sprint("/" + strings.Join(p, "/") + s)
	}
}
