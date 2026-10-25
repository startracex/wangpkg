package pack

import "io/fs"

// info overrides the name and size reported for a tar entry.
type info struct {
	fs.FileInfo
	name *string
	size *int64
}

// Size returns the overridden size when set.
func (i info) Size() int64 {
	if i.size != nil {
		return *i.size
	}
	return i.FileInfo.Size()
}

// Name returns the overridden name when set.
func (i info) Name() string {
	if i.name != nil {
		return *i.name
	}
	return i.FileInfo.Name()
}
