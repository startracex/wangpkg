package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gookit/color"
	"github.com/startracex/wangpkg/pkg/auth"
	"github.com/startracex/wangpkg/pkg/fmtutil"
	"github.com/startracex/wangpkg/pkg/npmconfig"
	"github.com/startracex/wangpkg/pkg/search"
	"github.com/urfave/cli/v2"
)

var SearchCommand = &cli.Command{
	Name:      "search",
	Usage:     "Search for packages",
	ArgsUsage: "search [term...]",
	Flags: []cli.Flag{
		flagSize,
	},
	Action: func(ctx *cli.Context) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		args := ctx.Args()
		if args.Len() < 1 {
			return fmt.Errorf("missing search term")
		}
		return RunSearch(cwd, search.Parameters{
			Texts: args.Slice(),
			Size:  ctx.Int(flagSize.Name),
		})
	},
}

func RunSearch(cwd string, p search.Parameters) error {
	ni, err := npmconfig.LoadMerged(npmconfig.LoadOptions{
		Cwd:      cwd,
		Location: npmconfig.LocationProject,
	})
	if err != nil {
		return err
	}
	res, err := search.Search(p, ni, auth.NewTransport(ni, nil))
	if err != nil {
		return err
	}
	fmt.Println(printSearchResult(res))
	return nil
}

func ellipsis(s string, maxLen int) string {
	t := strings.TrimRight(s, ".")
	if len(t) > maxLen && maxLen > 3 {
		return t[:maxLen-3] + "..."
	}
	return s
}

func printSearchResult(res *search.Response) string {
	out := new(strings.Builder)
	for index, obj := range res.Objects {
		if index != 0 {
			out.WriteString("\n")
		}

		fmt.Println(out, color.Blue.Sprint(obj.Package.Name))
		if obj.Package.Description != "" {
			fmt.Fprintln(out, ellipsis(obj.Package.Description, 120))
		}
		p := obj.Package.Publisher
		p.Username = color.Blue.Sprint(p.Username)
		fmt.Fprintf(
			out, "Version %s published at %s by %s",
			color.Blue.Sprint(obj.Package.Version),
			color.Cyan.Sprint(
				fmt.Sprintf("%s (%s)", obj.Package.Date.Format(time.DateOnly),
					fmtutil.FormatDuration(time.Since(obj.Package.Date))),
			),
			p.String(),
		)
		out.WriteString("\n")

		if len(obj.Package.Keywords) > 0 {
			fmt.Fprintf(out, "Keywords: %s\n", color.Magenta.Sprint(strings.Join(obj.Package.Keywords, " ")))
		}
		fmt.Fprintln(out, color.Blue.Sprint("https://npmx.dev/package/"+obj.Package.Name))
	}
	return out.String()
}
