package bombardier

import (
	"fmt"
	"strings"
)

type Header struct {
	key, value string
}

type HeadersList []Header

func (h *HeadersList) String() string {
	return fmt.Sprint(*h)
}

func (h *HeadersList) IsCumulative() bool {
	return true
}

func (h *HeadersList) Set(value string) error {
	res := strings.SplitN(value, ":", 2)
	if len(res) != 2 {
		return errInvalidHeaderFormat
	}
	*h = append(*h, Header{
		res[0], strings.Trim(res[1], " "),
	})
	return nil
}
