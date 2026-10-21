package deps

import "iter"

type Mode uint8

const (
	ModeDev Mode = 1 << iota
	ModeOptional
	ModePeer
	ModeProd
)

var allModes = [4]Mode{ModeDev, ModeOptional, ModeProd, ModePeer}

func (m Mode) Iterator() iter.Seq2[Mode, string] {
	return func(yield func(m Mode, s string) bool) {
		for _, key := range allModes {
			if key&m != 0 {
				if !yield(key, key.String()) {
					return
				}
			}
		}
	}
}

func (m Mode) String() string {
	switch m {
	case ModeDev:
		return "devDependencies"
	case ModeOptional:
		return "optionalDependencies"
	case ModeProd:
		return "dependencies"
	case ModePeer:
		return "peerDependencies"
	default:
		panic("unknown mode")
	}
}

func NewMode(dev, optional, peer, prod bool) Mode {
	var mode Mode
	if dev {
		mode |= ModeDev
	}
	if optional {
		mode |= ModeOptional
	}
	if peer {
		mode |= ModePeer
	}
	if prod || mode == 0 {
		mode |= ModeProd
	}
	return mode
}
