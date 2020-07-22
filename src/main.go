package main

import (
	"fmt"
	"golang.org/x/net/html"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
)

type Element interface {}

type FileHref struct {
	href string
	name string
	dir string
}

type DirHref struct {
	href string
}

func ForEachNode(n *html.Node, f func(n *html.Node)) {
	f(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		ForEachNode(c, f)
	}
}

func (f *FileHref) PrintMarkDown() {
	fmt.Printf("- [ ] [%s](%s) \n", f.name, f.href)
}

func CheckLink(n *html.Node, extension string) Element {
	var hasRightStyles = false
	var isTrackedFile = false
	var isDir = false

	var href string
	var fname string
	var dirname string

	matchFile := regexp.MustCompile(fmt.Sprintf(".*%s$", extension))
	matchDir := regexp.MustCompile(`.+/tree/.+`)

	for _, a := range n.Attr {
		switch a.Key {
		case "href":
			href = "https://github.com" + a.Val
			fname = strings.Split(href, "/")[len(strings.Split(href, "/")) - 1]

			if matchDir.Match([]byte(href)) {
				isDir = true
			} else if matchFile.Match([]byte(fname)) {
				isTrackedFile = true
				dirname = strings.Join(strings.Split(href, "/")[4:], "")
			}
		case "class":
			if a.Val == "js-navigation-open link-gray-dark" {
				hasRightStyles = true
			}
		}
	}

	if hasRightStyles {
		if isTrackedFile {
			return FileHref {
				href: href,
				name: fname[:len(fname) - len(extension)],
				dir: dirname,
			}
		}

		if isDir {
			return DirHref {
				href: href,
			}
		}
	}

	return nil
}

func Extract(url, extension string) []Element {
	res, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}

	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		log.Fatalf("getting %s by HTML: %v", url, err)
	}

	doc, err := html.Parse(res.Body)
	if err != nil {
		log.Fatalf("analise %s by HTML: %v", url, err)
	}

	files := make([]Element, 0)
	visitNode := func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			files = append(files, CheckLink(n, extension))
		}
	}

	ForEachNode(doc, visitNode)
	return files
}

func main() {
	url := os.Args[1]
	extension := os.Args[2]

	worklist := make(chan []Element)
	results := make([]FileHref, 0)

	var n int
	n++
	go func() {
		worklist <- Extract(url, extension)
	}()

	for ; n > 0; n-- {
		list := <-worklist
		for _, f := range list {
			switch v := f.(type) {
			case DirHref:
				n++
				go func() {
					worklist <- Extract(v.href, extension)
				}()
			case FileHref:
				results = append(results, v)
			}
		}
	}

	for _, r := range results {
		r.PrintMarkDown()
	}
}
