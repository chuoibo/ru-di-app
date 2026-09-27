package guard

import "golang.org/x/text/unicode/norm"

// DinhDang runs only the DATA-FORMAT checks of the output guard: an email
// address, a phone number, a bank account or payment card number (a long
// digit run). It is format validation for privacy, which the owner's rule
// keeps (docs/architecture/03-ai-engine-hop-dong.md §8), not language
// understanding: it reads no meaning from words. The tools call it on text a
// model wants to STORE (remember_fact), so a number or an address never
// reaches long-term memory whatever the model classified it as.
func DinhDang(text string) LoaiRa {
	text = norm.NFKC.String(text)
	if email.MatchString(text) {
		return RaEmail
	}
	return soLienLac(text)
}
