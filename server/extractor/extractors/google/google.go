// Package google provides an extractor for Google search result pages.
package google

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/asciimoo/hister/server/extractor/sdk"
	"github.com/asciimoo/hister/server/extractor/serputil"
	"github.com/asciimoo/hister/server/extractor/urlutil"
)

const (
	resultMetadataType = "google_result"
	answerMetadataType = "google_answer"

	// keepAnswerOption makes the extractor store the AI Overview answer as the
	// search page's own document instead of dropping it with the page.
	keepAnswerOption = "keep_answer"

	// answerSelector is the container Google fills with the AI Overview answer.
	answerSelector = `div[data-subtree="aimc"]`

	// noiseSelector matches elements whose text is markup or styling rather
	// than content.
	noiseSelector = "script, style, noscript, template"

	// snippetSelector lists the containers Google uses for the description of
	// a result. The first one present in a result block wins.
	snippetSelector = ".VwiC3b, [data-sncf], .IsZvec, .lyLwlc, .st"

	// adSelector matches promoted results and the wrappers around them.
	adSelector = "#tads, #tadsb, #bottomads, .uEierd, [data-text-ad], .commercial-unit-desktop-top"
)

// answerLabels are the interface labels Google puts in front of an AI Overview
// answer. They are stripped so the stored text starts with the answer.
var answerLabels = []string{"AI Mode replied:", "AI Overview:", "AI Overview"}

var (
	// blockSelectors are result containers in order of preference. The closest
	// match scopes snippet extraction for one result.
	blockSelectors = []string{"div.g", "div.MjjYud", "div[data-hveid]"}

	// googleContentHosts are Google owned hosts that appear as outbound links
	// on result pages, such as image proxies and ad click trackers.
	googleContentHosts = []string{
		"gstatic.com",
		"googleusercontent.com",
		"googlesyndication.com",
		"googleadservices.com",
		"google-analytics.com",
		"ggpht.com",
	}
)

// GoogleExtractor indexes the organic results of a Google search page as one
// document per result.
type GoogleExtractor struct {
	cfg *sdk.Config
}

var _ sdk.Extractor = (*GoogleExtractor)(nil)

func (e *GoogleExtractor) Name() string {
	return "Google"
}

func (e *GoogleExtractor) Description() string {
	return "Indexes the organic results of Google search pages as individual documents with their title, snippet, and displayed site."
}

func (e *GoogleExtractor) Capabilities() sdk.Capabilities {
	return sdk.Capabilities{Extract: true, Preview: true}
}

func (e *GoogleExtractor) Match(d *sdk.Document) bool {
	if d == nil {
		return false
	}
	u, err := url.Parse(d.URL)
	if err != nil || u.User != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	if strings.TrimSuffix(u.Path, "/") != "/search" {
		return false
	}
	return isGoogleHost(u.Hostname())
}

// Extract records every organic result as an extra document and leaves the
// result page itself out of the index. A page that yields no result, such as a
// JavaScript shell, a consent screen, or an empty search, is skipped as well:
// none of it is worth indexing as boilerplate.
//
// With the keep_answer option the AI Overview answer is stored as the search
// page's own document, because it is the one part of the page whose content is
// not available anywhere else.
func (e *GoogleExtractor) Extract(d *sdk.Document) sdk.ExtractResult {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(d.HTML))
	if err != nil {
		return sdk.ExtractFallback(err)
	}
	query := searchQuery(doc, d.URL)
	results := parseResults(doc, d.URL)
	answer := ""
	if e.keepAnswer() {
		answer = answerText(doc)
	}
	if answer == "" {
		// The result page is never a document of its own, so the flag covers
		// both the parsed and the empty case.
		d.SkipIndexing = true
	} else {
		d.SkipIndexing = false
		d.Text = answer
		if serputil.CleanText(d.Title) == "" {
			d.Title = query
		}
		d.AddMetadata("type", answerMetadataType)
		if query != "" {
			d.AddMetadata("query", query)
		}
	}
	if len(results) == 0 {
		if answer != "" {
			return sdk.Extracted()
		}
		return sdk.ExtractFallback(errors.New("no google search results found"))
	}
	d.ExtraDocuments = append(d.ExtraDocuments,
		serputil.Documents(results, d.UserID, resultMetadataType, query, e.maxResults())...)
	return sdk.Extracted()
}

// Preview renders the parsed results instead of the raw result page.
func (e *GoogleExtractor) Preview(d *sdk.Document) sdk.PreviewResult {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(d.HTML))
	if err != nil {
		return sdk.PreviewFallback(err)
	}
	answer := ""
	if e.keepAnswer() {
		answer = answerText(doc)
	}
	results := serputil.Limit(parseResults(doc, d.URL), e.maxResults())
	return serputil.PreviewWithAnswer("Google", searchQuery(doc, d.URL), answer, results)
}

// answerText returns the text of the AI Overview answer on a result page, with
// the interface label and the inline scripts Google ships inside the container
// removed.
func answerText(doc *goquery.Document) string {
	container := doc.Find(answerSelector).First()
	if container.Length() == 0 {
		return ""
	}
	clone := container.Clone()
	clone.Find(noiseSelector).Remove()
	text := serputil.CleanText(clone.Text())
	for _, label := range answerLabels {
		if rest, found := strings.CutPrefix(text, label); found {
			text = strings.TrimSpace(rest)
			break
		}
	}
	return text
}

func (e *GoogleExtractor) GetConfig() *sdk.Config {
	if e.cfg != nil {
		return e.cfg
	}
	return &sdk.Config{Enable: true, Options: map[string]any{
		"max_results":    serputil.DefaultMaxResults,
		keepAnswerOption: false,
	}}
}

func (e *GoogleExtractor) SetConfig(config *sdk.Config) error {
	if err := serputil.ValidateOptions(config.Options, keepAnswerOption); err != nil {
		return err
	}
	if _, err := keepAnswerValue(config.Options); err != nil {
		return err
	}
	e.cfg = config
	return nil
}

func (e *GoogleExtractor) maxResults() int {
	maxResults, err := serputil.MaxResults(e.GetConfig().Options)
	if err != nil {
		return serputil.DefaultMaxResults
	}
	return maxResults
}

func (e *GoogleExtractor) keepAnswer() bool {
	keep, err := keepAnswerValue(e.GetConfig().Options)
	if err != nil {
		return false
	}
	return keep
}

// keepAnswerValue returns the keep_answer option, false when it is absent.
func keepAnswerValue(options map[string]any) (bool, error) {
	value, ok := options[keepAnswerOption]
	if !ok {
		return false, nil
	}
	keep, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("%s must be a boolean", keepAnswerOption)
	}
	return keep, nil
}

// parseResults collects the organic results of a result page in display order.
func parseResults(doc *goquery.Document, pageURL string) []serputil.Result {
	base, _ := url.Parse(pageURL)
	var results serputil.Collector
	doc.Find("h3").Each(func(_ int, heading *goquery.Selection) {
		// Current pages wrap the heading in the title link, older markup nests
		// the title link inside the heading.
		anchor := heading.Closest("a[href]")
		if anchor.Length() == 0 {
			anchor = heading.Find("a[href]").First()
		}
		if anchor.Length() == 0 || anchor.Closest(adSelector).Length() > 0 {
			return
		}
		raw, ok := anchor.Attr("href")
		if !ok {
			return
		}
		block := resultBlock(anchor)
		target, ok := resultURL(base, raw)
		if ok && isInternalURL(target) {
			ok = false
		}
		if !ok {
			// Current pages link results through opaque /goto tokens that carry
			// no readable target, so rebuild the address from the URL text
			// Google displays under the title.
			target, ok = citeURL(block)
		}
		if !ok || isInternalURL(target) {
			return
		}
		title := serputil.CleanText(heading.Text())
		if title == "" {
			return
		}
		results.Add(serputil.Result{
			Title:   title,
			URL:     target,
			Snippet: snippetText(block, title),
			Site:    siteName(block, target),
		})
	})
	return results.Results()
}

// resultBlock returns the closest ancestor that scopes a single result.
func resultBlock(anchor *goquery.Selection) *goquery.Selection {
	for _, selector := range blockSelectors {
		if block := anchor.Closest(selector); block.Length() > 0 {
			return block.First()
		}
	}
	node := anchor.Parent()
	for range 3 {
		if node.Length() == 0 {
			break
		}
		if node.Is("div, li") {
			return node
		}
		node = node.Parent()
	}
	return node
}

// resultURL resolves a title link and unwraps Google's outbound redirect.
func resultURL(base *url.URL, raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "#") {
		return "", false
	}
	resolved := raw
	if base != nil {
		resolved = urlutil.ResolveURL(base, raw)
	}
	u, err := url.Parse(resolved)
	if err != nil {
		return "", false
	}
	// Without JavaScript Google wraps outbound links as /url?q=<target>.
	if isGoogleHost(u.Hostname()) && strings.TrimSuffix(u.Path, "/") == "/url" {
		params := u.Query()
		target := params.Get("q")
		if target == "" {
			target = params.Get("url")
		}
		if target == "" {
			return "", false
		}
		return serputil.CanonicalURL(target)
	}
	return serputil.CanonicalURL(resolved)
}

// citeURL rebuilds the address of a result from the URL text Google displays
// under its title. Result links in current pages are opaque /goto tokens, which
// leave the cite element as the only readable target. Breadcrumb segments
// separated by "›" are path segments, so "https://example.com › docs › intro"
// becomes "https://example.com/docs/intro".
func citeURL(block *goquery.Selection) (string, bool) {
	if block.Length() == 0 {
		return "", false
	}
	text := serputil.CleanText(block.Find("cite").First().Text())
	if text == "" {
		return "", false
	}
	text = strings.ReplaceAll(text, "›", "/")
	text = strings.ReplaceAll(text, " ", "")
	if !strings.Contains(text, "://") {
		text = "https://" + strings.TrimPrefix(text, "//")
	}
	if idx := strings.Index(text, "://"); idx >= 0 {
		scheme, rest := text[:idx+3], text[idx+3:]
		for strings.Contains(rest, "//") {
			rest = strings.ReplaceAll(rest, "//", "/")
		}
		text = scheme + rest
	}
	return serputil.CanonicalURL(text)
}

// isInternalURL reports whether a result link stays inside Google.
func isInternalURL(rawURL string) bool {
	host := serputil.HostOf(rawURL)
	if isGoogleHost(host) {
		return true
	}
	for _, domain := range googleContentHosts {
		if serputil.IsHost(host, domain) {
			return true
		}
	}
	return false
}

// isGoogleHost reports whether host belongs to a Google domain. Only the
// domain shapes Google serves results from are accepted: google.<tld>,
// www.google.<tld>, google.<sld>.<tld>, and their subdomains such as
// maps.google.com. Lookalikes such as google.example.com stay out.
func isGoogleHost(host string) bool {
	labels := strings.Split(strings.ToLower(strings.TrimSuffix(host, ".")), ".")
	for i, label := range labels {
		if label != "google" {
			continue
		}
		switch rest := labels[i+1:]; {
		case len(rest) == 1 && len(rest[0]) >= 2:
			return true
		case len(rest) == 2 && len(rest[0]) == 2:
			return true
		}
	}
	return false
}

// snippetText returns the description of one result, falling back to the text
// of the result block without its title and displayed URL.
func snippetText(block *goquery.Selection, title string) string {
	if block.Length() > 0 {
		for _, selector := range strings.Split(snippetSelector, ",") {
			selector = strings.TrimSpace(selector)
			if candidate := block.Find(selector).First(); candidate.Length() > 0 {
				if text := serputil.CleanText(candidate.Text()); text != "" {
					return serputil.ClipSnippet(text)
				}
			}
		}
	}
	text := serputil.CleanText(block.Text())
	text = strings.Replace(text, title, "", 1)
	if cite := serputil.CleanText(block.Find("cite").First().Text()); cite != "" {
		text = strings.Replace(text, cite, "", 1)
	}
	return serputil.ClipSnippet(text)
}

// siteName prefers the host Google displays under the result.
func siteName(block *goquery.Selection, target string) string {
	if block.Length() > 0 {
		if cite := serputil.CleanText(block.Find("cite").First().Text()); cite != "" {
			if site := serputil.DisplayedHost(cite); site != "" {
				return site
			}
		}
	}
	return serputil.DisplayedHost(target)
}

// searchQuery returns the query of the result page from its URL, its search
// field, or its title.
func searchQuery(doc *goquery.Document, pageURL string) string {
	if query := serputil.Query(pageURL); query != "" {
		return query
	}
	for _, selector := range []string{"textarea[name='q']", "input[name='q']"} {
		field := doc.Find(selector).First()
		if field.Length() == 0 {
			continue
		}
		if text := serputil.CleanText(field.Text()); text != "" {
			return text
		}
		if value, ok := field.Attr("value"); ok {
			if text := serputil.CleanText(value); text != "" {
				return text
			}
		}
	}
	title := serputil.CleanText(doc.Find("title").First().Text())
	for _, suffix := range []string{" - Google Search", " - Google"} {
		if trimmed := strings.TrimSuffix(title, suffix); trimmed != title {
			return trimmed
		}
	}
	return ""
}
