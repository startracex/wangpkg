package lockfile

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"strings"

	"github.com/startracex/wangpkg/pkg/deps"
)

type Resolution struct {
	Mode     deps.Mode
	Resolved string
}

func (r Resolution) String() string {
	sb := new(strings.Builder)
	sb.Grow(len(r.Resolved) + 18)
	if r.Mode&deps.ModeDev != 0 {
		sb.WriteString("dev+")
	}
	if r.Mode&deps.ModeOptional != 0 {
		sb.WriteString("optional+")
	}
	if r.Mode&deps.ModePeer != 0 {
		sb.WriteString("peer+")
	}
	sb.WriteString(r.Resolved)
	return sb.String()
}

func FromString(s string) Resolution {
	var (
		dev      bool
		optional bool
		peer     bool
	)
	s, dev = strings.CutPrefix(s, "dev+")
	s, optional = strings.CutPrefix(s, "optional+")
	s, peer = strings.CutPrefix(s, "peer+")

	return Resolution{
		Resolved: s,
		Mode:     deps.NewMode(dev, optional, peer, false),
	}
}

func (r *Resolution) MarshalJSONTo(enc *jsontext.Encoder) error {
	return json.MarshalEncode(enc, r.String())
}

func (r *Resolution) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	s := new(string)
	if err := json.UnmarshalDecode(dec, s); err != nil {
		return err
	}
	*r = FromString(*s)
	return nil
}
