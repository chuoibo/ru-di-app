package dieuchinh

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func renderedReview(t *testing.T, answered *bool) (*httptest.ResponseRecorder, *html.Node) {
	t.Helper()
	rec := httptest.NewRecorder()
	writeReviewPage(rec, GuestReview{
		Status: "proposed", Token: "synthetic-review-token", Answered: answered,
		Lines: []GuestLine{{RecipientName: "Người nhận tổng hợp", OldVND: "30.000", NewVND: "20.000"}},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("render status %d, want 200", rec.Code)
	}
	node, err := html.Parse(strings.NewReader(rec.Body.String()))
	if err != nil {
		t.Fatal(err)
	}
	return rec, node
}

func elements(node *html.Node, tag string) []*html.Node {
	var found []*html.Node
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == tag {
			found = append(found, n)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(node)
	return found
}

func attribute(node *html.Node, key string) (string, bool) {
	for _, a := range node.Attr {
		if a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}

func TestReviewPageAllowsOnlyLocalDeferredEnhancement(t *testing.T) {
	rec, page := renderedReview(t, nil)
	policy := map[string]string{}
	for _, directive := range strings.Split(rec.Header().Get("Content-Security-Policy"), ";") {
		fields := strings.Fields(directive)
		if len(fields) > 0 {
			policy[fields[0]] = strings.Join(fields[1:], " ")
		}
	}
	for directive, want := range map[string]string{
		"default-src": "'none'", "script-src": "'self'", "style-src": "'self'",
		"form-action": "'self'", "base-uri": "'none'", "frame-ancestors": "'none'",
	} {
		if policy[directive] != want {
			t.Errorf("CSP %s = %q, want %q", directive, policy[directive], want)
		}
	}
	scripts := elements(page, "script")
	if len(scripts) != 1 {
		t.Fatalf("script count %d, want one local enhancement", len(scripts))
	}
	src, _ := attribute(scripts[0], "src")
	_, deferred := attribute(scripts[0], "defer")
	if src != "/static/guest.js" || !deferred || scripts[0].FirstChild != nil {
		t.Fatal("review enhancement must be external, local and deferred; inline script is not allowed")
	}
}

func TestReviewDecisionsRemainOrdinaryNamedFormSubmits(t *testing.T) {
	_, page := renderedReview(t, nil)
	forms := elements(page, "form")
	if len(forms) != 1 {
		t.Fatalf("form count %d, want one review form", len(forms))
	}
	method, _ := attribute(forms[0], "method")
	action, _ := attribute(forms[0], "action")
	if method != "post" || action != "/g/synthetic-review-token/dieu-chinh" {
		t.Fatalf("native review target = %s %s", method, action)
	}
	buttons := elements(forms[0], "button")
	if len(buttons) != 2 {
		t.Fatalf("decision count %d, want accept and reject", len(buttons))
	}
	for i, want := range []string{"dong_y", "khong_dong_y"} {
		name, _ := attribute(buttons[i], "name")
		value, _ := attribute(buttons[i], "value")
		kind, _ := attribute(buttons[i], "type")
		_, disabled := attribute(buttons[i], "disabled")
		if name != "tra_loi" || value != want || kind != "submit" || disabled {
			t.Errorf("decision %d loses native submission: name=%q value=%q type=%q disabled=%v", i, name, value, kind, disabled)
		}
	}
	if len(elements(forms[0], "input")) != 0 {
		t.Fatal("pending review must not carry a default hidden answer that changes the clicked decision")
	}
}

func TestAnsweredProposedReviewKeepsWaitingAndRemovesDecisions(t *testing.T) {
	for _, accept := range []bool{true, false} {
		rec, page := renderedReview(t, &accept)
		if len(elements(page, "form")) != 0 || !strings.Contains(rec.Body.String(), "Đang chờ những người còn lại.") {
			t.Fatal("an answered but unresolved review must show waiting without another decision form")
		}
	}
}
