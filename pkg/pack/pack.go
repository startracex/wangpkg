package pack

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"
	"github.com/startracex/wangpkg/pkg/fmtutil"
	"github.com/startracex/wangpkg/pkg/glob"
	"github.com/startracex/wangpkg/pkg/osutil"
	"github.com/startracex/wangpkg/pkg/specifier"
	"github.com/startracex/wangpkg/pkg/workspace"
)

var AlwaysInclude = []string{
	"README",
	"LICENSE",
	"LICENCE",
}

var AlwaysExclude = []string{
	`.git`,
	`.npmrc`,
	`node_modules`,
	`package-lock.json`,
	`pnpm-lock.yaml`,
	`yarn.lock`,
	`bun.lock`,
	`deno.lock`,
	`wangpkg.lock`,
}

var UsesIgnoreFileNames = []string{
	".npmignore",
	".gitignore",
}

var DefaultIgnore = []string{
	`*.orig`,
	`.*.swp`,
	`.DS_Store`,
	`._*`,
	`.git`,
	`.hg`,
	`.lock-wscript`,
	`.npmrc`,
	`.svn`,
	`.wafpickle-N`,
	`CVS`,
	`config.gypi`,
	`node_modules`,
	`npm-debug.log`,
	`package-lock.json`,
	`pnpm-lock.yaml`,
	`yarn.lock`,
	`bun.lock`,
	`deno.lock`,
	`wangpkg.lock`,
}

func trimExt(path string) string {
	ext := filepath.Ext(path)
	if base := filepath.Base(path); len(base) > 1 && ext == base {
		return path
	}
	return strings.TrimSuffix(path, ext)
}

func containsFold(s []string, t string) bool {
	for _, v := range s {
		if strings.EqualFold(v, t) {
			return true
		}
	}
	return false
}

// FindSimilarFiles returns a file in dir whose name matches one of names.
func FindSimilarFiles(dir string, names []string) (string, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("read dir %s: %w", dir, err)
	}
	found := ""
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		fileName := file.Name()
		if containsFold(names, fileName) {
			return fileName, nil
		}
		if found == "" && containsFold(names, trimExt(fileName)) {
			found = fileName
		}
	}
	return found, nil
}

func PackName(name string) string {
	reg := &specifier.RegistrySpecifier{Name: name}
	if scope, base := reg.SplitName(); scope != "" {
		name = scope + "-" + base
	}
	return name
}

type Init struct {
	*workspace.Workspace
	LicensePath string
	Prefix      string
}

// Create writes the tarball of a package into writer.
func (i *Init) Create(pkg *workspace.Package, writer *tar.Writer) (err error) {
	prefix := i.Prefix
	paths, err := IncludeFileMap(pkg)
	hasLicense := false
	for rel, abs := range paths {
		name := filepath.ToSlash(filepath.Join(prefix, rel))
		if abs == pkg.FilePath {
			if err := writeSyncedManifest(pkg, name, writer); err != nil {
				return fmt.Errorf("write manifest of %s: %w", pkg.Name, err)
			}
			continue
		}

		if err := writeLocalFile(abs, name, writer); err != nil {
			return fmt.Errorf("write %s: %w", abs, err)
		}
		hasLicense = hasLicense || containsFold(AlwaysInclude[1:], trimExt(rel))
	}
	if !hasLicense {
		if filePath := i.LicensePath; filePath != "" {
			fileName := filepath.Base(filePath)
			name := filepath.ToSlash(filepath.Join(prefix, fileName))
			if err := writeLocalFile(filePath, name, writer); err != nil {
				return fmt.Errorf("write license %s: %w", filePath, err)
			}
		}
	}
	if err != nil {
		return fmt.Errorf("collect files of %s: %w", pkg.Name, err)
	}
	return nil
}

// IncludeFileMap returns the files to pack, keyed by their path relative to the package directory.
func IncludeFileMap(pkg *workspace.Package) (map[string]string, error) {
	mustIncludes, err := MustIncludeFiles(pkg)
	if err != nil {
		return nil, fmt.Errorf("collect required files of %s: %w", pkg.Name, err)
	}
	var includes []string
	if len(pkg.Files) > 0 {
		includes, err = PatternsIncludeFiles(pkg.Dir, pkg.Files)
	} else {
		includes, err = CollectFiles(pkg.Dir, CollectOptions{
			FileOnly:        true,
			Absolute:        true,
			IgnoreFileNames: UsesIgnoreFileNames,
			GlobalIgnore:    ignore.CompileIgnoreLines(append(UsesIgnoreFileNames, DefaultIgnore...)...),
		})
	}
	if err != nil {
		return nil, fmt.Errorf("collect files of %s: %w", pkg.Name, err)
	}
	paths := append(mustIncludes, includes...)
	result := make(map[string]string, len(paths))
	for _, path := range paths {
		rel, err := filepath.Rel(pkg.Dir, path)
		if err != nil {
			return nil, fmt.Errorf("relativize %s against %s: %w", path, pkg.Dir, err)
		}
		rel = filepath.ToSlash(rel)
		result[rel] = path
	}
	return result, nil
}

// MustIncludeFiles returns the files npm always ships.
func MustIncludeFiles(pkg *workspace.Package) ([]string, error) {
	result := []string{pkg.FilePath}
	files, err := os.ReadDir(pkg.Dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", pkg.Dir, err)
	}
	add := func(name string) {
		result = append(result, filepath.Join(pkg.Dir, name))
	}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		fileName := file.Name()
		if containsFold(AlwaysInclude, trimExt(fileName)) {
			add(fileName)
		}
	}
	for _, binFile := range pkg.Bin() {
		add(binFile)
	}
	if main := string(pkg.Main); main != "" {
		add(main)
	}
	return result, nil
}

// PatternsIncludeFiles expands the "files" field of a manifest.
func PatternsIncludeFiles(dir string, files []string) ([]string, error) {
	patterns := make([]string, 0, len(UsesIgnoreFileNames)+len(AlwaysExclude)+len(files))
	for _, s := range UsesIgnoreFileNames {
		patterns = append(patterns, "!**/"+s)
	}
	for _, s := range AlwaysExclude {
		patterns = append(patterns, "!**/"+s)
	}
	patterns = append(patterns, files...)
	return glob.MatchFiles(patterns, glob.Options{
		Dir:             dir,
		GlobStar:        true,
		FileOnly:        true,
		Absolute:        true,
		ExpandDirs:      true,
		CaseInsensitive: osutil.CaseInsensitive(),
	})
}

// writeSyncedManifest writes the synced manifest into the tarball.
func writeSyncedManifest(pkg *workspace.Package, name string, writer *tar.Writer) error {
	b, err := fmtutil.FormatJSON(pkg.FilePath, pkg.Manifest)
	if err != nil {
		return fmt.Errorf("format manifest %s: %w", pkg.FilePath, err)
	}
	i, err := os.Stat(pkg.FilePath)
	if err != nil {
		return fmt.Errorf("stat %s: %w", pkg.FilePath, err)
	}
	size := int64(len(b))
	fi := info{
		FileInfo: i,
		name:     &name,
		size:     &size,
	}
	hdr, err := tar.FileInfoHeader(fi, "")
	if err != nil {
		return fmt.Errorf("build tar header of %s: %w", name, err)
	}
	if err := writer.WriteHeader(hdr); err != nil {
		return fmt.Errorf("write tar header of %s: %w", name, err)
	}
	if _, err := writer.Write(b); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	return nil
}

// writeLocalFile copies a file into the tarball.
func writeLocalFile(path, name string, writer *tar.Writer) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}

	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return fmt.Errorf("build tar header of %s: %w", name, err)
	}
	header.Name = name

	if err := writer.WriteHeader(header); err != nil {
		return fmt.Errorf("write tar header of %s: %w", name, err)
	}

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()

	if _, err = io.Copy(writer, file); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	return nil
}
