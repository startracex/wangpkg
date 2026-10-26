package cli

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/gookit/color"
	"github.com/startracex/wangpkg/pkg/auth"
	"github.com/startracex/wangpkg/pkg/fmtutil"
	"github.com/startracex/wangpkg/pkg/jspath"
	"github.com/startracex/wangpkg/pkg/manifest"
	"github.com/startracex/wangpkg/pkg/npmconfig"
	"github.com/startracex/wangpkg/pkg/specifier"
	"github.com/urfave/cli/v2"
)

var InfoCommand = &cli.Command{
	Name:      "info",
	Usage:     "Get information about a package",
	ArgsUsage: "info <package specifier> [path.to.field]",
	Action: func(ctx *cli.Context) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		args := ctx.Args()
		if args.Len() < 1 {
			return fmt.Errorf("missing package specifier")
		}
		return RunInfo(cwd, args.Get(0), args.Get(1))
	},
}

func RunInfo(cwd, packageSpec, path string) error {
	var lookup []string
	if path != "" {
		fields, err := jspath.Split(path)
		if err != nil {
			return fmt.Errorf("invalid path: %w", err)
		}
		lookup = fields
	}
	spec, err := specifier.Parse(packageSpec)
	if err != nil {
		return fmt.Errorf("invalid package specifier: %w", err)
	}
	regSpec, ok := spec.(*specifier.RegistrySpecifier)
	if !ok {
		return fmt.Errorf("package specifier is not a registry specifier")
	}
	ni, err := npmconfig.LoadMerged(npmconfig.LoadOptions{
		Cwd:      cwd,
		Location: npmconfig.LocationProject,
	})
	if err != nil {
		return err
	}
	d, err := manifest.FetchDocument(regSpec, ni, auth.NewTransport(ni, nil))
	if err != nil {
		return err
	}
	m, err := d.Resolve(regSpec.Version)
	if err != nil {
		return err
	}
	if len(lookup) > 0 {
		target, err := m.Object.Lookup(lookup...)
		if err != nil {
			return err
		}
		return json.MarshalWrite(os.Stdout, target, jsontext.WithIndent("  "))
	}
	fmt.Print(printInfoResult(m, d))
	return nil
}

func maxLen(a []string) int {
	x := 0
	for _, v := range a {
		x = max(x, len(v))
	}
	return x
}

func printInfoResult(m *manifest.Manifest, d *manifest.Document) string {
	out := new(strings.Builder)
	fmt.Fprintf(
		out,
		"%s | %s | dependencies: %s | versions: %s\n",
		color.Cyan.Sprint(m.Name+"@"+m.Version),
		color.Green.Sprint(m.License),
		color.Cyan.Sprint(len(m.Dependencies)),
		color.Cyan.Sprint(len(d.Versions)),
	)
	if v := m.Description; v != "" {
		fmt.Fprintln(out, v)
	}
	if v := m.Homepage; v != "" {
		fmt.Fprintln(out, color.Blue.Sprint(v))
	}
	fmt.Fprintln(out)

	if keywords := m.Keywords; len(keywords) > 0 {
		fmt.Fprintf(out, "keywords: %s\n\n", color.Magenta.Sprint(strings.Join(keywords, " ")))
	}

	fmt.Fprintf(out, "dist\n")
	fmt.Fprintf(out, "  tarball: %s\n", m.Dist.Tarball)
	fmt.Fprintf(out, "  shasum: %s\n", m.Dist.Shasum)
	fmt.Fprintf(out, "  integrity: %s\n", m.Dist.Integrity)
	if m.Dist.FileCount > 0 {
		fmt.Fprintf(out, "  fileCount: %d\n", m.Dist.FileCount)
	}
	if m.Dist.Size > 0 {
		fmt.Fprintf(out, "  size: %s\n", fmtutil.FormatIEC(m.Dist.Size))
	}
	if m.Dist.UnpackedSize > 0 {
		fmt.Fprintf(out, "  unpackedSize: %s\n", fmtutil.FormatIEC(m.Dist.UnpackedSize))
	}
	fmt.Fprintln(out)

	if len(m.Maintainers) > 0 {
		fmt.Fprintf(out, "maintainers:\n")
		for _, p := range m.Maintainers {
			p.Name = color.Blue.Sprint(p.Name)
			p.Email = color.Black.Sprint(p.Email)
			fmt.Fprintf(out, "  - %s\n", p.String())
		}
		fmt.Fprintln(out)
	}

	fmt.Fprintf(out, "dist-tags:\n")
	tags := slices.Collect(maps.Keys(d.DistTags))
	slices.Sort(tags)
	maxTagLen := maxLen(tags)
	for _, tagName := range tags {
		tagVersion := d.DistTags[tagName]
		fmt.Fprintf(
			out, "  %s%s  %s\n",
			color.Yellow.Sprint(tagName),
			strings.Repeat(" ", maxTagLen-len(tagName)),
			tagVersion,
		)
	}
	fmt.Fprintln(out)

	t := d.Time[m.Version]
	publisher := m.Publisher()
	publisher.Name = color.Blue.Sprint(publisher.Name)
	fmt.Fprintf(
		out,
		"published at %s by %s\n",
		color.Cyan.Sprintf(
			"%s (%s)",
			t.Format(time.DateOnly),
			fmtutil.FormatDuration(time.Since(t)),
		),
		publisher.String(),
	)
	return out.String()
}
