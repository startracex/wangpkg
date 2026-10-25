package publish

import (
	"crypto/sha1"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json/v2"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/startracex/wangpkg/pkg/manifest"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

func checkAudience(token, want string) error {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil
	}
	var claims struct {
		Aud any `json:"aud"`
	}
	if err := json.Unmarshal(raw, &claims); err != nil || claims.Aud == nil {
		return nil
	}

	got := make([]string, 0, 2)
	switch aud := claims.Aud.(type) {
	case string:
		got = append(got, aud)
	case []any:
		for _, v := range aud {
			if s, ok := v.(string); ok {
				got = append(got, s)
			}
		}
	default:
		return nil
	}
	if len(got) == 0 {
		return nil
	}
	if slices.Contains(got, want) {
		return nil
	}
	return fmt.Errorf("OIDC token audience is %q, want %q: the token must be requested with audience=sigstore", got, want)
}

func pickKeys(m map[string]string, keys []string) map[string]string {
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := m[key]; ok {
			result[key] = value
		}
	}
	return result
}

func pkgurl(name, version string) string {
	if strings.HasPrefix(name, "@") {
		name = "%40" + name[1:]
	}
	return "pkg:npm/" + name + "@" + version
}

type sumResult struct {
	Integrity manifest.Integrity
	Base64    string
	SHA1      string
	SHA512    string
	Size      int64
}

func sumReader(r io.Reader) (sumResult, error) {
	var result sumResult

	hash1 := sha1.New()
	hash512 := sha512.New()
	base64Builder := new(strings.Builder)
	base64Encoder := base64.NewEncoder(base64.StdEncoding, base64Builder)

	w := io.MultiWriter(hash1, hash512, base64Encoder)
	size, err := io.Copy(w, r)
	if err != nil {
		return result, err
	}

	if err := base64Encoder.Close(); err != nil {
		return result, err
	}

	sum1 := hash1.Sum(nil)
	sum512 := hash512.Sum(nil)

	result.Size = size
	result.SHA1 = hex.EncodeToString(sum1)
	result.SHA512 = hex.EncodeToString(sum512)
	result.Integrity = manifest.Integrity("sha512-" + base64.StdEncoding.EncodeToString(sum512))
	result.Base64 = base64Builder.String()

	return result, nil
}

func newStruct(v any) (*structpb.Struct, error) {
	var raw []byte
	var err error
	if msg, ok := v.(proto.Message); ok {
		raw, err = (protojson.MarshalOptions{EmitUnpopulated: true}).Marshal(msg)
	} else {
		raw, err = json.Marshal(v)
	}
	if err != nil {
		return nil, fmt.Errorf("encode value: %w", err)
	}
	out := new(structpb.Struct)
	if err := protojson.Unmarshal(raw, out); err != nil {
		return nil, fmt.Errorf("decode value into struct: %w", err)
	}
	return out, nil
}
