package npmini

import (
	"bytes"
	"os"
	"strings"

	"gopkg.in/ini.v1"
)

type NpmINI ini.File

func newOptions() ini.LoadOptions {
	return ini.LoadOptions{
		AllowShadows:             true,
		PreserveSurroundedQuote:  true,
		SpaceBeforeInlineComment: true,
		IgnoreInlineComment:      true,
		KeyValueDelimiters:       "=",
	}
}

func New() *NpmINI {
	ni, _ := Load([]byte(""))
	return ni
}

func Load(source any) (*NpmINI, error) {
	ni, err := ini.LoadSources(newOptions(), source)
	if err != nil {
		return nil, err
	}
	return (*NpmINI)(ni), nil
}

func (n *NpmINI) INI() *ini.File {
	return (*ini.File)(n)
}

func (n *NpmINI) Section(name string) *ini.Section {
	return (*ini.File)(n).Section(name)
}

func (n *NpmINI) rootSection() *ini.Section {
	return n.Section("")
}

func (n *NpmINI) getKey(name string) (*ini.Key, bool) {
	sec := n.rootSection()
	key, _ := sec.GetKey(name)
	return key, key != nil
}

func (n *NpmINI) Get(name string) (string, bool) {
	key, ok := n.getKey(name)
	if !ok {
		return "", false
	}
	return os.ExpandEnv(key.String()), true
}

func (n *NpmINI) GetArray(name string) []string {
	key, ok := n.getKey(name + "[]")
	if ok {
		vals := key.ValueWithShadows()
		var out []string
		for _, v := range vals {
			out = append(out, os.ExpandEnv(v))
		}
		return out
	}
	return nil
}

func (n *NpmINI) Set(name, value string) bool {
	key, ok := n.getKey(name)
	if ok {
		if key.String() == value {
			return false
		}
		key.SetValue(value)
	} else {
		_, _ = n.rootSection().NewKey(name, value)
	}
	return true
}

func (n *NpmINI) SetArray(key string, values []string) {
	sec := n.rootSection()
	sec.DeleteKey(key)
	sec.DeleteKey(key + "[]")

	for _, v := range values {
		_, _ = sec.NewKey(key+"[]", v)
	}
}

func (n *NpmINI) Delete(key string) bool {
	sec := n.rootSection()
	result := false
	if sec.HasKey(key) {
		sec.DeleteKey(key)
		result = true
	}
	if sec.HasKey(key + "[]") {
		sec.DeleteKey(key + "[]")
		result = true
	}
	return result
}

func (n *NpmINI) Bytes() []byte {
	b := new(bytes.Buffer)
	_, _ = n.INI().WriteTo(b)
	return b.Bytes()
}

func (n *NpmINI) String() string {
	b := new(strings.Builder)
	_, _ = n.INI().WriteTo(b)
	return b.String()
}

func (n *NpmINI) Merge(m ...*NpmINI) {
	sec := n.rootSection()
	for _, f := range m {
		if f == nil {
			continue
		}
		src := f.rootSection()
		mergeSection(src, sec)
	}
}

func (n *NpmINI) Clone() *NpmINI {
	return (*NpmINI)(cloneFile(n.INI()))
}

func cloneFile(f *ini.File) *ini.File {
	dst := ini.Empty()
	for _, secName := range f.SectionStrings() {
		srcSec := f.Section(secName)
		dstSec, _ := dst.NewSection(secName)
		dstSec.Comment = srcSec.Comment
		for _, k := range srcSec.Keys() {
			newKey, _ := dstSec.NewKey(k.Name(), k.Value())
			newKey.Comment = k.Comment
		}
	}
	return dst
}

func mergeSection(src *ini.Section, dst *ini.Section) {
	for _, k := range src.Keys() {
		name := k.Name()
		dst.DeleteKey(name)

		for _, v := range k.ValueWithShadows() {
			_, _ = dst.NewKey(name, v)
		}
	}
}
