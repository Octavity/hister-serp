// SPDX-License-Identifier: AGPL-3.0-or-later

package google

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"

	"github.com/asciimoo/hister/server/document"
	"github.com/asciimoo/hister/server/extractor/sdk"
	"github.com/asciimoo/hister/server/extractor/serputil"
)

const searchURL = "https://www.google.com/search?q=hister+search"

// answerPage keeps the AI Overview container of a current result page: the
// interface label and the inline scripts Google ships inside it are what the
// extractor has to strip.
const answerPage = `<!doctype html>
<html>
<head><title>serp - Google Search</title></head>
<body>
  <div role="heading" aria-level="2"><strong class="WJFise">AI Overview</strong></div>
  <div data-subtree="aimc" jscontroller="v48bt">
    <script>(function(){window.sn={};}).call(this);</script>
    <span class="AgI0hc">AI Mode replied:</span>
    <div>A SERP is the page a search engine displays after a query is submitted.</div>
  </div>
  <div class="MjjYud">
    <div class="g">
      <a href="https://github.com/asciimoo/hister"><h3>GitHub - asciimoo/hister</h3></a>
      <div class="VwiC3b">A private search engine for your own web history.</div>
      <cite>https://github.com › asciimoo › hister</cite>
    </div>
  </div>
</body>
</html>`

// answerOnlyPage is an instant answer without any organic result.
const answerOnlyPage = `<!doctype html>
<html>
<head><title>what is a serp - Google Search</title></head>
<body>
  <div data-subtree="aimc">
    <div>A SERP is the page a search engine displays after a query is submitted.</div>
  </div>
</body>
</html>`

const resultPage = `<!doctype html>
<html>
<head><title>hister search - Google Search</title></head>
<body>
  <textarea name="q">hister search</textarea>
  <div id="search">
    <div class="MjjYud">
      <div class="g">
        <div class="yuRUbf">
          <a jsname="UWckNb" href="https://github.com/asciimoo/hister">
            <br><h3 class="LC20lb MBeuO DKV0Md">GitHub - asciimoo/hister</h3>
          </a>
        </div>
        <div class="VwiC3b yXK7lf MUxGbd">A private search engine for your own web history.</div>
        <div class="byrV5b"><cite class="qLRx3b tjvcx">https://github.com › asciimoo › hister</cite></div>
      </div>
    </div>
    <div class="MjjYud">
      <div class="g">
        <a href="/url?q=https://en.wikipedia.org/wiki/Web_scraping&amp;sa=U&amp;ved=2ahUKEwi">
          <h3 class="LC20lb">Web scraping - Wikipedia</h3>
        </a>
        <div class="VwiC3b">Web scraping is the collection of data from the web.</div>
        <cite>https://en.wikipedia.org › wiki › Web_scraping</cite>
      </div>
    </div>
    <div class="MjjYud">
      <div class="g">
        <a href="https://github.com/asciimoo/hister"><h3>GitHub - asciimoo/hister</h3></a>
        <div class="VwiC3b">Duplicate result.</div>
      </div>
    </div>
    <div class="MjjYud">
      <div class="g">
        <a href="https://www.google.com/search?q=hister&amp;tbm=isch"><h3>Images for hister</h3></a>
      </div>
    </div>
    <div id="tads">
      <div class="uEierd">
        <a href="https://ads.example.com/hister"><h3>Sponsored result</h3></a>
        <div class="VwiC3b">Buy things you do not need.</div>
      </div>
    </div>
  </div>
</body>
</html>`

const legacyResultPage = `<!doctype html>
<html>
<head><title>legacy query - Google Search</title></head>
<body>
  <div class="g">
    <h3 class="r"><a href="/url?q=https://example.org/legacy%3Fid%3D1&amp;sa=U">Legacy result</a></h3>
    <div class="s"><span class="st">Legacy description text.</span></div>
  </div>
</body>
</html>`

// currentResultPage mirrors the markup Google serves to browsers: result links
// are opaque /goto tokens and the target survives only as the URL text under
// the title.
const currentResultPage = `<!doctype html>
<html>
<head><title>hister search - Google Search</title></head>
<body>
  <div id="rso">
    <div class="MjjYud">
      <div class="N54PNb BToiNc">
        <div class="yuRUbf">
          <a jsname="UWckNb" class="zReHs" href="/goto?url=CAESYQHrOzAV2yCP3lWjgD8vu_dhpoEuqeLzozgogPNJfbJ9Pp602Wc_Cx1TzGcSJVcmvBrsPJDhJyn_hg" data-ved="2ahUKEwjvsNawmdWXAxUKr1YBHbE3JywQFnoECCEQAQ"><h3 class="LC20lb MBeuO DKV0Md">Login</h3><br><div class="notranslate"><cite class="qLRx3b tjvcx GvPZzd cHaqb" role="text">https://serp.uv.edu.ph<span class="ylgVCe ob9lvb" role="text"> › serp › Login</span></cite></div></a>
        </div>
        <div class="VwiC3b yXK7lf MUxGbd lEBKkf"><span>Sign in to the University of the Visayas portal.</span></div>
      </div>
    </div>
    <div class="MjjYud">
      <div class="N54PNb BToiNc">
        <div class="yuRUbf">
          <a jsname="UWckNb" class="zReHs" href="/goto?url=CAESVgHrOzAVxLQgN9IWTS3YbraI0_f8iDqkL77kpVXxWeOxBv7lV600EXgsmpCQogmqdn"><h3 class="LC20lb">SERP-P: Socioeconomic Research Portal</h3><br><div class="notranslate"><cite class="qLRx3b tjvcx">serp-p.pids.gov.ph</cite></div></a>
        </div>
        <div class="VwiC3b"><span>The portal for socioeconomic research in the Philippines.</span></div>
      </div>
    </div>
    <div class="MjjYud">
      <h3>AI Mode replied:</h3>
      <div class="VwiC3b">A heading that belongs to no result.</div>
    </div>
    <div class="MjjYud">
      <div class="N54PNb BToiNc">
        <div class="yuRUbf">
          <a jsname="UWckNb" class="zReHs" href="/goto?url=CAESnoaddress"><h3 class="LC20lb">Result without a displayed address</h3></a>
        </div>
      </div>
    </div>
    <div id="tads">
      <div class="uEierd">
        <a href="/goto?url=CAESsponsored"><h3>Sponsored result</h3></a>
        <div class="VwiC3b">Buy things you do not need.</div>
        <cite>ads.example.com</cite>
      </div>
    </div>
  </div>
</body>
</html>`

func TestMatch(t *testing.T) {
	e := &GoogleExtractor{}
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{name: "www search", url: "https://www.google.com/search?q=hister", want: true},
		{name: "country domain", url: "https://google.de/search?q=hister", want: true},
		{name: "trailing slash", url: "https://www.google.co.uk/search/?q=hister", want: true},
		{name: "maps path", url: "https://maps.google.com/maps?q=hister", want: false},
		{name: "docs path", url: "https://docs.google.com/document/d/abc", want: false},
		{name: "root path", url: "https://www.google.com/", want: false},
		{name: "lookalike host", url: "https://notgoogle.com/search?q=hister", want: false},
		{name: "googleapis host", url: "https://www.googleapis.com/search?q=hister", want: false},
		{name: "other scheme", url: "ftp://www.google.com/search?q=hister", want: false},
		{name: "userinfo", url: "https://user@www.google.com/search?q=hister", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := e.Match(&document.Document{URL: test.url}); got != test.want {
				t.Fatalf("Match(%q) = %v, want %v", test.url, got, test.want)
			}
		})
	}
}

func TestExtractCreatesOneDocumentPerResult(t *testing.T) {
	e := &GoogleExtractor{}
	doc := &document.Document{URL: searchURL, HTML: resultPage, UserID: 7}

	result := e.Extract(doc)
	if decision, err := result.Unpack(); decision != sdk.ExtractorSuccess {
		t.Fatalf("Extract decision = %v (%v), want success", decision, err)
	}
	if !doc.SkipIndexing {
		t.Fatal("result page is indexed, want SkipIndexing")
	}
	if len(doc.ExtraDocuments) != 2 {
		t.Fatalf("extra documents = %d, want 2", len(doc.ExtraDocuments))
	}

	first := doc.ExtraDocuments[0]
	if first.URL != "https://github.com/asciimoo/hister" {
		t.Errorf("first URL = %q", first.URL)
	}
	if first.Title != "GitHub - asciimoo/hister" {
		t.Errorf("first title = %q", first.Title)
	}
	if !strings.Contains(first.Text, "A private search engine") {
		t.Errorf("first text = %q, want snippet", first.Text)
	}
	if first.UserID != 7 {
		t.Errorf("first owner = %d, want 7", first.UserID)
	}
	if !first.SummaryOnly {
		t.Error("result document is not marked as a summary")
	}
	if first.HTML != "" {
		t.Errorf("result document keeps HTML (%d bytes), want none", len(first.HTML))
	}
	if got := first.Metadata["type"]; got != resultMetadataType {
		t.Errorf("first metadata type = %v, want %q", got, resultMetadataType)
	}
	if got := first.Metadata["query"]; got != "hister search" {
		t.Errorf("first metadata query = %v", got)
	}
	if got := first.Metadata["position"]; got != 1 {
		t.Errorf("first metadata position = %v, want 1", got)
	}
	if got := first.Metadata["site"]; got != "github.com" {
		t.Errorf("first metadata site = %v, want github.com", got)
	}

	second := doc.ExtraDocuments[1]
	if second.URL != "https://en.wikipedia.org/wiki/Web_scraping" {
		t.Errorf("second URL = %q, want redirect target", second.URL)
	}
	if got := second.Metadata["position"]; got != 2 {
		t.Errorf("second metadata position = %v, want 2", got)
	}
	if got := second.Metadata["site"]; got != "en.wikipedia.org" {
		t.Errorf("second metadata site = %v, want en.wikipedia.org", got)
	}
	if !strings.Contains(second.Text, "collection of data from the web") {
		t.Errorf("second text = %q, want snippet", second.Text)
	}
}

func TestExtractLegacyMarkup(t *testing.T) {
	e := &GoogleExtractor{}
	doc := &document.Document{URL: "https://www.google.com/search?q=legacy+query", HTML: legacyResultPage}

	result := e.Extract(doc)
	if decision, err := result.Unpack(); decision != sdk.ExtractorSuccess {
		t.Fatalf("Extract decision = %v (%v), want success", decision, err)
	}
	if len(doc.ExtraDocuments) != 1 {
		t.Fatalf("extra documents = %d, want 1", len(doc.ExtraDocuments))
	}
	extra := doc.ExtraDocuments[0]
	if extra.URL != "https://example.org/legacy?id=1" {
		t.Errorf("URL = %q, want unwrapped redirect target", extra.URL)
	}
	if extra.Title != "Legacy result" {
		t.Errorf("title = %q", extra.Title)
	}
	if !strings.Contains(extra.Text, "Legacy description text.") {
		t.Errorf("text = %q, want snippet from .st", extra.Text)
	}
	if got := extra.Metadata["site"]; got != "example.org" {
		t.Errorf("metadata site = %v, want example.org", got)
	}
}

func TestExtractOpaqueRedirectLinks(t *testing.T) {
	e := &GoogleExtractor{}
	doc := &document.Document{URL: searchURL, HTML: currentResultPage}

	result := e.Extract(doc)
	if decision, err := result.Unpack(); decision != sdk.ExtractorSuccess {
		t.Fatalf("Extract decision = %v (%v), want success", decision, err)
	}
	if len(doc.ExtraDocuments) != 2 {
		t.Fatalf("extra documents = %d, want 2 (ad, unlinked heading, and result without address dropped)", len(doc.ExtraDocuments))
	}

	first := doc.ExtraDocuments[0]
	if first.URL != "https://serp.uv.edu.ph/serp/Login" {
		t.Errorf("first URL = %q, want path rebuilt from the displayed address", first.URL)
	}
	if first.Title != "Login" {
		t.Errorf("first title = %q", first.Title)
	}
	if !strings.Contains(first.Text, "University of the Visayas") {
		t.Errorf("first text = %q, want snippet", first.Text)
	}
	if got := first.Metadata["site"]; got != "serp.uv.edu.ph" {
		t.Errorf("first metadata site = %v, want serp.uv.edu.ph", got)
	}

	second := doc.ExtraDocuments[1]
	if second.URL != "https://serp-p.pids.gov.ph" {
		t.Errorf("second URL = %q, want scheme added to the displayed host", second.URL)
	}
	if got := second.Metadata["position"]; got != 2 {
		t.Errorf("second metadata position = %v, want 2", got)
	}

	for _, extra := range doc.ExtraDocuments {
		if strings.Contains(extra.URL, "google") {
			t.Errorf("result URL = %q, want no Google redirect link", extra.URL)
		}
	}
}

func TestExtractWithoutResultsFallsBack(t *testing.T) {
	e := &GoogleExtractor{}
	doc := &document.Document{
		URL:  "https://www.google.com/search?q=hister",
		HTML: `<html><head><title>Before you continue</title></head><body><div>Before you continue to Google</div></body></html>`,
	}
	result := e.Extract(doc)
	if decision, _ := result.Unpack(); decision != sdk.ExtractorFallback {
		t.Fatalf("Extract decision = %v, want fallback", decision)
	}
	if !doc.SkipIndexing {
		t.Error("SkipIndexing is not set, the result page would be indexed as boilerplate")
	}
	if len(doc.ExtraDocuments) != 0 {
		t.Errorf("extra documents = %d, want 0", len(doc.ExtraDocuments))
	}

	if _, decision, _ := e.Preview(doc).Unpack(); decision != sdk.ExtractorFallback {
		t.Fatalf("Preview decision = %v, want fallback", decision)
	}
}

func TestExtractKeepsAnswerWhenConfigured(t *testing.T) {
	t.Run("disabled by default", func(t *testing.T) {
		e := &GoogleExtractor{}
		doc := &document.Document{URL: searchURL, HTML: answerPage}
		if decision, err := e.Extract(doc).Unpack(); decision != sdk.ExtractorSuccess {
			t.Fatalf("Extract decision = %v (%v), want success", decision, err)
		}
		if !doc.SkipIndexing {
			t.Error("result page is indexed by default, want SkipIndexing")
		}
		if doc.Text != "" {
			t.Errorf("Text = %q, want no page text", doc.Text)
		}
		if len(doc.ExtraDocuments) != 1 {
			t.Fatalf("extra documents = %d, want 1", len(doc.ExtraDocuments))
		}
	})

	t.Run("keeps the answer", func(t *testing.T) {
		e := keepAnswerExtractor(t)
		doc := &document.Document{URL: searchURL, HTML: answerPage}
		if decision, err := e.Extract(doc).Unpack(); decision != sdk.ExtractorSuccess {
			t.Fatalf("Extract decision = %v (%v), want success", decision, err)
		}
		if doc.SkipIndexing {
			t.Error("result page with a kept answer is skipped, want it indexed")
		}
		if !strings.HasPrefix(doc.Text, "A SERP is the page a search engine displays") {
			t.Fatalf("Text = %q, want the answer text", doc.Text)
		}
		for _, unwanted := range []string{"AI Mode replied", "window.sn", "AI Overview"} {
			if strings.Contains(doc.Text, unwanted) {
				t.Errorf("Text contains %q, want it stripped: %q", unwanted, doc.Text)
			}
		}
		if doc.Title != "hister search" {
			t.Errorf("Title = %q, want the query for a page without a title", doc.Title)
		}
		if got := doc.Metadata["type"]; got != answerMetadataType {
			t.Errorf("metadata type = %v, want %q", got, answerMetadataType)
		}
		if got := doc.Metadata["query"]; got != "hister search" {
			t.Errorf("metadata query = %v, want the search query", got)
		}
		if len(doc.ExtraDocuments) != 1 {
			t.Fatalf("extra documents = %d, want 1", len(doc.ExtraDocuments))
		}
		if !doc.ExtraDocuments[0].SummaryOnly {
			t.Error("result document is not a summary, it would replace an indexed page")
		}
	})

	t.Run("answer without results", func(t *testing.T) {
		e := keepAnswerExtractor(t)
		doc := &document.Document{URL: searchURL, HTML: answerOnlyPage}
		if decision, err := e.Extract(doc).Unpack(); decision != sdk.ExtractorSuccess {
			t.Fatalf("Extract decision = %v (%v), want success for a kept answer", decision, err)
		}
		if doc.SkipIndexing {
			t.Error("answer only page is skipped, want it indexed")
		}
		if !strings.HasPrefix(doc.Text, "A SERP is the page") {
			t.Fatalf("Text = %q, want the answer text", doc.Text)
		}
		if len(doc.ExtraDocuments) != 0 {
			t.Errorf("extra documents = %d, want 0", len(doc.ExtraDocuments))
		}
	})

	t.Run("no answer on the page", func(t *testing.T) {
		e := keepAnswerExtractor(t)
		doc := &document.Document{URL: searchURL, HTML: resultPage}
		if decision, err := e.Extract(doc).Unpack(); decision != sdk.ExtractorSuccess {
			t.Fatalf("Extract decision = %v (%v), want success", decision, err)
		}
		if !doc.SkipIndexing {
			t.Error("result page without an answer is indexed, want SkipIndexing")
		}
		if doc.Text != "" {
			t.Errorf("Text = %q, want no page text", doc.Text)
		}
	})
}

func keepAnswerExtractor(t *testing.T) *GoogleExtractor {
	t.Helper()
	e := &GoogleExtractor{}
	if err := e.SetConfig(&sdk.Config{Enable: true, Options: map[string]any{keepAnswerOption: true}}); err != nil {
		t.Fatalf("SetConfig: %v", err)
	}
	return e
}

func TestPreviewRendersResults(t *testing.T) {
	e := &GoogleExtractor{}
	doc := &document.Document{URL: searchURL, HTML: resultPage}
	response, decision, err := e.Preview(doc).Unpack()
	if decision != sdk.ExtractorSuccess {
		t.Fatalf("Preview decision = %v (%v), want success", decision, err)
	}
	content := response.Content
	if !strings.Contains(content, "Google search: hister search") {
		t.Errorf("preview misses the query: %q", content)
	}
	if !strings.Contains(content, `<a href="https://github.com/asciimoo/hister">GitHub - asciimoo/hister</a>`) {
		t.Errorf("preview misses the linked title: %q", content)
	}
	if !strings.Contains(content, "Web scraping - Wikipedia") {
		t.Errorf("preview misses the second result: %q", content)
	}
	if strings.Contains(content, "Sponsored result") || strings.Contains(content, "Images for hister") {
		t.Errorf("preview contains an ad or an internal link: %q", content)
	}
	if strings.Contains(content, "textarea") {
		t.Errorf("preview contains page markup: %q", content)
	}
}

func TestPreviewRendersAnswer(t *testing.T) {
	e := keepAnswerExtractor(t)
	response, decision, err := e.Preview(&document.Document{URL: searchURL, HTML: answerPage}).Unpack()
	if decision != sdk.ExtractorSuccess {
		t.Fatalf("Preview decision = %v (%v), want success", decision, err)
	}
	content := response.Content
	if !strings.Contains(content, "a search engine displays after a query is submitted") {
		t.Errorf("preview misses the answer: %q", content)
	}
	if !strings.Contains(content, `<a href="https://github.com/asciimoo/hister">GitHub - asciimoo/hister</a>`) {
		t.Errorf("preview misses the linked title: %q", content)
	}
	if strings.Contains(content, "AI Mode replied:") {
		t.Errorf("preview contains the interface label: %q", content)
	}
	if strings.Index(content, "a search engine displays") > strings.Index(content, "<ol>") {
		t.Errorf("preview renders the answer below the results: %q", content)
	}
}

func TestPreviewRendersAnswerWithoutResults(t *testing.T) {
	e := keepAnswerExtractor(t)
	response, decision, err := e.Preview(&document.Document{URL: searchURL, HTML: answerOnlyPage}).Unpack()
	if decision != sdk.ExtractorSuccess {
		t.Fatalf("Preview decision = %v (%v), want success", decision, err)
	}
	if !strings.Contains(response.Content, "a search engine displays after a query is submitted") {
		t.Errorf("preview misses the answer: %q", response.Content)
	}
	if strings.Contains(response.Content, "<ol>") {
		t.Errorf("preview renders an empty result list: %q", response.Content)
	}
}

func TestSearchQueryFallbacks(t *testing.T) {
	tests := []struct {
		name string
		url  string
		html string
		want string
	}{
		{
			name: "URL query wins",
			url:  "https://www.google.com/search?q=from+url",
			html: `<html><body><textarea name="q">from field</textarea></body></html>`,
			want: "from url",
		},
		{
			name: "search field",
			url:  "https://www.google.com/search",
			html: `<html><body><input name="q" value="from field"></body></html>`,
			want: "from field",
		},
		{
			name: "title",
			url:  "https://www.google.com/search",
			html: `<html><head><title>from title - Google Search</title></head><body></body></html>`,
			want: "from title",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			doc, err := goquery.NewDocumentFromReader(strings.NewReader(test.html))
			if err != nil {
				t.Fatal(err)
			}
			if got := searchQuery(doc, test.url); got != test.want {
				t.Fatalf("searchQuery = %q, want %q", got, test.want)
			}
		})
	}
}

func TestSetConfig(t *testing.T) {
	e := &GoogleExtractor{}
	if got := e.GetConfig().Options["max_results"]; got != serputil.DefaultMaxResults {
		t.Errorf("default max_results = %v, want %d", got, serputil.DefaultMaxResults)
	}
	if err := e.SetConfig(&sdk.Config{Enable: true, Options: map[string]any{"unknown": true}}); err == nil {
		t.Fatal("SetConfig accepted an unknown option")
	}
	for _, value := range []any{0, -1, 1.5, "10", true} {
		if err := e.SetConfig(&sdk.Config{Enable: true, Options: map[string]any{"max_results": value}}); err == nil {
			t.Errorf("SetConfig accepted max_results %v", value)
		}
	}
	if err := e.SetConfig(&sdk.Config{Enable: true, Options: map[string]any{"max_results": 1}}); err != nil {
		t.Fatalf("SetConfig rejected max_results: %v", err)
	}
	doc := &document.Document{URL: searchURL, HTML: resultPage}
	if decision, err := e.Extract(doc).Unpack(); decision != sdk.ExtractorSuccess {
		t.Fatalf("Extract decision = %v (%v)", decision, err)
	}
	if len(doc.ExtraDocuments) != 1 {
		t.Fatalf("extra documents = %d, want 1 with max_results=1", len(doc.ExtraDocuments))
	}
	if got := e.GetConfig().Options[keepAnswerOption]; got != nil {
		t.Errorf("keep_answer = %v after a config without the option, want it absent", got)
	}
	if err := e.SetConfig(&sdk.Config{Enable: true, Options: map[string]any{keepAnswerOption: true}}); err != nil {
		t.Fatalf("SetConfig rejected keep_answer: %v", err)
	}
	if err := e.SetConfig(&sdk.Config{Enable: true, Options: map[string]any{keepAnswerOption: "yes"}}); err == nil {
		t.Fatal("SetConfig accepted a non boolean keep_answer")
	}
}

func TestSetConfigDefaults(t *testing.T) {
	e := &GoogleExtractor{}
	if got := e.GetConfig().Options[keepAnswerOption]; got != false {
		t.Errorf("default keep_answer = %v, want false", got)
	}
	if got := e.GetConfig().Options["max_results"]; got != serputil.DefaultMaxResults {
		t.Errorf("default max_results = %v, want %d", got, serputil.DefaultMaxResults)
	}
}
