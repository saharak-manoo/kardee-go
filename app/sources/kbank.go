package sources

import (
	"context"
	"errors"

	"github.com/saharak/kardee-go/app/fetch"
)

// ErrSkipped means the source is switched off on purpose.
var ErrSkipped = errors.New("source skipped")

// KBank: kasikornbank.com answers 403 (Akamai bot protection) to automated requests,
// robots.txt included. We don't work around bot protection, so KBank stays off until
// there is a source we're allowed to read (a public feed, a partner API, or permission).
type KBank struct{}

func (k *KBank) Bank() string { return "kbank" }

func (k *KBank) List(context.Context, *fetch.Client) ([]Listing, error) {
	return nil, errors.Join(ErrSkipped, errors.New("kasikornbank.com blocks automated reading (403)"))
}

func (k *KBank) Detail(context.Context, *fetch.Client, Listing) (Detail, error) {
	return Detail{}, ErrSkipped
}
