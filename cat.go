package cat

import (
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("cat")

// NewCat returns a new CatRouter object.
func NewCat() *CatRouter {
	return &CatRouter{}
}

// CatRouter is a router that implements the CAT protocol.
type CatRouter struct {
}
