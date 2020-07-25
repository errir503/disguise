// Package commands provides functions for CLI commands
package commands

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// element is the interface of the element that is added to markdown.
type element interface {
	getMarkDown() string
}

// mdFile is the type of file that is added to markdown.
type mdFile struct {
	href string // the file href
	name string // the file name
	dir  mdDir  // the directory where the file is located
}

// mdFile is the type of dir that is added to markdown.
type mdDir struct {
	name string // the directory name
	href string // the directory href
}

// getMarkDown of mdFile returns markdown specify for files.
func (f mdFile) getMarkDown() string {
	return fmt.Sprintf("- [ ] [%s](%s)\n", f.name, f.href)
}

// getMarkDown of mdDir returns markdown specify for directories.
func (d mdDir) getMarkDown() string {
	return fmt.Sprintf("##### [%s](%s)\n", d.name, d.href)
}

// IsContains checks whether an item is in a list or in a list item.
func IsContains(elements []string, some string) bool {
	if len(elements) == 0 {
		return false
	}
	for _, e := range elements {
		match, err := regexp.MatchString(e, some)
		if err != nil {
			log.Fatal(err)
		}
		if match {
			return true
		}
	}

	return false
}

// ParseHrefAttr get href and file extension and
// returns is this item mdFile or mdDir and dirname.
func ParseHrefAttr(href, extension string) (bool, bool, string) {
	var isDir = false
	var isTrackedFile = false
	var dirname string

	matchFile := regexp.MustCompile(fmt.Sprintf(".+/blob/.+%s$", extension))
	matchDir := regexp.MustCompile(`.+/tree/.+`)

	pathArray := strings.Split(href, "/")
	if matchDir.Match([]byte(href)) {
		isDir = true
		dirname = strings.Join(pathArray[7:], "/")
	} else if matchFile.Match([]byte(href)) {
		isTrackedFile = true
		dirname = strings.Join(pathArray[7:len(pathArray)-1], "/")
	}

	return isDir, isTrackedFile, dirname
}

// GetDirHref gets href to file and returns link to dir where this file placed
func GetDirHref(filehref, dirname string) string {
	path := strings.Split(filehref, "/blob/")
	dirhref := path[0] + "/tree/" + strings.Split(path[1], "/")[0] + "/" + dirname

	return dirhref
}

// checkLink get html node and generate element of type element.
func checkLink(n *html.Node, extension string, ignoreDirs []string) element {
	hasRightStyles := false
	isTrackedFile := false
	isDir := false

	var href string
	var fname string
	var dirname string
	var filedirhref string

	for _, a := range n.Attr {
		switch a.Key {
		case "href":
			href = "https://github.com" + a.Val
			fname = n.FirstChild.Data
		case "class":
			if a.Val == "js-navigation-open link-gray-dark" {
				hasRightStyles = true
			}
		}
	}

	if !hasRightStyles {
		return nil
	}

	if IsContains(ignoreDirs, dirname) {
		return nil
	}

	isDir, isTrackedFile, dirname = ParseHrefAttr(href, extension)

	if isDir {
		return mdDir{
			href: href,
			name: dirname,
		}
	}

	filedirhref = GetDirHref(href, dirname)
	if isTrackedFile {
		return mdFile{
			href: href,
			name: fname[:len(fname)-len(extension)],
			dir: mdDir{
				href: filedirhref,
				name: dirname,
			},
		}
	}

	return nil
}

// extract extracts all directories and files from this link.
func extract(folderURL, extension string, ignoreDirs []string) []element {
	res, err := http.Get(folderURL) //nolint:gosec
	if err != nil {
		log.Fatal(err)
	}

	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		log.Fatalf("getting %s by HTML: %v", folderURL, res.Status)
	}

	doc, err := html.Parse(res.Body)
	if err != nil {
		log.Fatalf("analise %s by HTML: %v", folderURL, err.Error())
	}

	files := make([]element, 0)
	visitNode := func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			link := checkLink(n, extension, ignoreDirs)
			if link != nil {
				files = append(files, link)
			}
		}
	}

	forEachNode(doc, visitNode)

	return files
}

// forEachNode recursively traverses the entire element tree of html node.
func forEachNode(n *html.Node, f func(n *html.Node)) {
	f(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		forEachNode(c, f)
	}
}

// groupByDir groups input array to map where key is directory name.
func groupByDir(files []mdFile) map[mdDir][]mdFile {
	grouped := make(map[mdDir][]mdFile)
	for _, f := range files {
		grouped[f.dir] = append(grouped[f.dir], f)
	}

	return grouped
}

// crawl finds all mdFile's from passed URL and returns them.
func crawl(url, extension string, ignoreDirs []string) []mdFile {
	worklist := make(chan []element)
	results := make([]mdFile, 0)

	// Start with cmd arguments
	go func() {
		worklist <- extract(url, extension, ignoreDirs)
	}()

	for n := 1; n > 0; n-- {
		list := <-worklist
		for _, f := range list {
			switch v := f.(type) {
			case mdDir:
				n++
				go func() { worklist <- extract(v.href, extension, ignoreDirs) }()
			case mdFile:
				results = append(results, v)
			}
		}
	}

	return results
}

// printResult prints markdown of passed results elements into some out.
func printResults(out io.Writer, results []mdFile) {
	for dir, files := range groupByDir(results) {
		_, err := fmt.Fprint(out, dir.getMarkDown())
		if err != nil {
			log.Fatal(err)
		}
		for _, f := range files {
			_, printErr := fmt.Fprint(out, f.getMarkDown())
			if printErr != nil {
				log.Fatal(printErr)
			}
		}

		_, err = fmt.Fprint(out, "\n")

		if err != nil {
			log.Fatal(err)
		}
	}
}

// getIgnoreDirs returns dirs which should be ignored.
func getIgnoreDirs(toIgnore string) []string {
	var ignoreDirs = strings.Split(toIgnore, " ")
	if ignoreDirs[0] == "" {
		return make([]string, 0)
	}

	// todo
	//output := make([]string, 0)
	//for _, d := range ignoreDirs {
	//	if []byte(d)[0] == '/' {
	//		strings.
	//	}
	//}

	return ignoreDirs
}

// GetMarkdown is CLI command which prints into file generated markdown for this repository.
func GetMarkdown(url, extension, toIgnore string) {
	md := crawl(url, extension, getIgnoreDirs(toIgnore))

	fname := strings.Split(url, "/")[len(strings.Split(url, "/"))-1]

	f, err := os.Create(fmt.Sprintf("./results/%s.md", fname))
	if err != nil {
		panic(err)
	}

	printResults(f, md)
}
