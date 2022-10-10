package upstream

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"

	"github.com/miekg/dns"
)

type DOHUpstream struct {
	Remote string
}

func (u DOHUpstream) Forward(request *dns.Msg) (*dns.Msg, error) {
	content, err := request.Pack()
	if err != nil {
		return nil, err
	}

	result, err := http.Get(fmt.Sprintf("%s?dns=%s", u.Remote, base64.RawURLEncoding.EncodeToString(content)))
	if err != nil {
		return nil, err
	}
	defer result.Body.Close()
	bytes, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, err
	}

	response := new(dns.Msg)
	if err := response.Unpack(bytes); err != nil {
		return nil, err
	}
	return response, nil
}
