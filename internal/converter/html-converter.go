package converter

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/google/uuid"
	"golang.org/x/net/html"
	"path/filepath"
	"strconv"
	"strings"
	"yuque-sync-confluence/internal/confluence"
	"yuque-sync-confluence/internal/httputil"
	"yuque-sync-confluence/internal/yuque"
)

type HtmlConverter struct {
	yuqueDoc      *yuque.DocTree
	confluenceDoc *confluence.DocTree

	document *goquery.Document
}

func NewHtmlConverter(yuqueDoc *yuque.DocTree, confluenceDoc *confluence.DocTree) (*HtmlConverter, error) {
	yuqueDocHtml, err := yuqueDoc.DocHtml()
	if err != nil {
		return nil, err
	}
	document, err := goquery.NewDocumentFromReader(strings.NewReader(yuqueDocHtml))
	if err != nil {
		return nil, err
	}

	return &HtmlConverter{
		yuqueDoc:      yuqueDoc,
		confluenceDoc: confluenceDoc,
		document:      document,
	}, nil
}

func (c *HtmlConverter) Convert() error {
	c.ConvertStrongSeparator()
	c.ConvertCode()
	c.ConvertImg()
	c.ConvertSvg()
	c.ConvertList()
	c.ConvertTodoList()
	c.ConvertFirstDiv()

	if err := c.UpdateDoc(); err != nil {
		return err
	}

	return nil
}

func (c *HtmlConverter) UpdateDoc() error {
	htmlBody, err := c.document.Find("html").Html()
	if err != nil {
		return err
	}
	if err := c.confluenceDoc.UpdateDoc(strings.TrimPrefix(c.confluenceDoc.Title(), "[Temp]"), htmlBody); err != nil {
		return err
	}
	return nil
}

func (c *HtmlConverter) ConvertStrongSeparator() {
	c.document.Find("strong").Each(func(i int, selection *goquery.Selection) {
		if selection.Text() == "" {
			selection.ReplaceWithNodes(selection.Children().Nodes...)
		}
	})
}

func (c *HtmlConverter) ConvertCode() {
	c.document.Find("pre").Each(func(i int, selection *goquery.Selection) {
		selection.ReplaceWithNodes(
			NewNode(html.ElementNode, "ac:structured-macro").
				AddAttr("ac:name", "code").AddAttr("ac:schema-version", "1").
				AddAttr("ac:macro-id", uuid.New().String()).AddChild(
				NewNode(html.ElementNode, "ac:plain-text-body").AddChild(
					NewNode(html.RawNode, "<![CDATA["+selection.Text()+"]]>"))).Node())
	})
}

func (c *HtmlConverter) ConvertFirstDiv() {
	div := c.document.Find("div").First()

	div.RemoveAttr("class")
	div.RemoveAttr("typography")

	div.AppendNodes(
		NewNode(html.ElementNode, "span").AddAttr("class", "ne-text").AddChild(
			NewNode(html.ElementNode, "ac:structured-macro").AddAttr("ac:name", "easy-heading-free").
				AddAttr("ac:schema-version", "1").AddAttr("ac:macro-id", uuid.NewString())).Node())
}

func (c *HtmlConverter) ConvertSvgHtml(svgText string) (string, error) {
	d, err := goquery.NewDocumentFromReader(strings.NewReader(string(svgText)))
	if err != nil {
		return "", err
	}
	d.Find("svg").Each(func(i int, selection *goquery.Selection) {
		width, _ := selection.Attr("width")
		height, _ := selection.Attr("height")
		selection.SetAttr("style", fmt.Sprintf("max-width:%s;max-height:%s;width:100%%;height:100%%;", width, height))
	})
	svgHtml, err := d.Find("svg").Parent().Html()
	if err != nil {
		return "", err
	}

	return svgHtml, nil
}

func (c *HtmlConverter) ConvertSvg() {
	c.document.Find("img").Each(func(i int, selection *goquery.Selection) {
		url, exist := selection.Attr("src")
		if !exist {
			return
		}
		if !c.IsSvg(url) {
			return
		}
		body, err := c.GetImage(url)
		if err != nil {
			return
		}
		svgHtml, err := c.ConvertSvgHtml(string(body))
		if err != nil {
			return
		}

		selection.ReplaceWithNodes(
			NewNode(html.ElementNode, "ac:structured-macro").AddAttr("ac:name", "html").
				AddAttr("ac:schema-version", "1").AddAttr("ac:macro-id", uuid.NewString()).AddChild(
				NewNode(html.ElementNode, "ac:plain-text-body").AddChild(
					NewNode(html.RawNode, "<![CDATA["+svgHtml+"]]>"))).Node())
	})
}

func (c *HtmlConverter) IsSvg(url string) bool {
	return strings.HasSuffix(url, ".svg")
}

func (c *HtmlConverter) GetImage(url string) ([]byte, error) {
	body, err := httputil.Get(url, nil, nil)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func (c *HtmlConverter) ConvertImg() {
	c.document.Find("img").Each(func(i int, selection *goquery.Selection) {
		url, exist := selection.Attr("src")
		if !exist {
			return
		}
		if c.IsSvg(url) {
			return
		}
		body, err := c.GetImage(url)
		if err != nil {
			return
		}
		fileName := filepath.Base(url)
		if err := c.confluenceDoc.AddDocAttachment(body, fileName); err != nil {
			return
		}

		selection.ReplaceWithNodes(
			NewNode(html.ElementNode, "ac:image").AddAttr("ac:thumbnail", "true").AddChild(
				NewNode(html.ElementNode, "ri:attachment").AddAttr("ri:filename", fileName)).Node())
	})
}

func (c *HtmlConverter) ConvertList() {
	c.document.Find("ul:not([ne-level]), ol:not([ne-level])").Each(func(i int, selection *goquery.Selection) {
		class, exist := selection.Attr("class")
		if !exist {
			return
		}
		data := goquery.NodeName(selection)

		selection.ReplaceWithNodes(
			NewNode(html.ElementNode, data).AddAttr("class", class).AddAttr("ne-level", "0").AddChildren(
				BuildNodes(c.cloneNodes(selection.Children().Nodes))).AddChildren(
				BuildNodes(c.ConvertNode(selection, 1))).Node())
	})

	ul := c.document.Find("ul[ne-level=\"0\"], ol[ne-level=\"0\"]").First()
	for {
		ul = c.mergeAdjoinNode(ul, 0)
		if ul == nil || len(ul.Nodes) == 0 {
			break
		}
	}
}

func (c *HtmlConverter) ConvertNode(selection *goquery.Selection, level int) []*html.Node {
	children := make([]*html.Node, 0)
	n := selection.Next()
	for {
		if !n.HasClass("ne-list-wrap") || n.Find(fmt.Sprintf("[ne-level=\"%d\"]", level)) == nil ||
			len(n.Find(fmt.Sprintf("[ne-level=\"%d\"]", level)).Nodes) == 0 {
			break
		}
		class, exist := selection.Attr("class")
		if !exist {
			n = n.Next()
			continue
		}
		data := goquery.NodeName(n)

		children = append(children,
			NewNode(html.ElementNode, data).AddAttr("class", class).
				AddAttr("ne-level", strconv.FormatInt(int64(level), 10)).AddChildren(
				BuildNodes(c.cloneNodes(n.Find(fmt.Sprintf("[ne-level=\"%d\"]", level)).Children().Nodes))).AddChildren(
				BuildNodes(c.ConvertNode(n, level+1))).Node())
		oldn := n
		n = n.Next()
		oldn.Remove()
	}

	return children
}

func (c *HtmlConverter) mergeAdjoinNode(selection *goquery.Selection, level int) *goquery.Selection {
	selectionSet := make([]*goquery.Selection, 0)
	if selection == nil {
		return nil
	}
	if !selection.Is(fmt.Sprintf(fmt.Sprintf("ul[ne-level=\"%d\"], ol[ne-level=\"%d\"]", level, level))) {
		return selection.Next()
	}
	class, exist := selection.Attr("class")
	if !exist {
		return selection.Next()
	}
	data := goquery.NodeName(selection)
	n := selection

	for {
		if !n.Is(fmt.Sprintf("[ne-level=\"%d\"][class=%s]", level, class)) {
			break
		}
		selectionSet = append(selectionSet, n)
		n = n.Next()
	}

	node := NewNode(html.ElementNode, data).AddAttr("class", class).AddAttr("ne-level", strconv.FormatInt(int64(level), 10))

	for _, s := range selectionSet {
		ul := s.Children().First()
		for {
			ul = c.mergeAdjoinNode(ul, level+1)
			if ul == nil || len(ul.Nodes) == 0 {
				break
			}
		}

		node.AddChildren(BuildNodes(c.cloneNodes(s.Children().Nodes)))
	}

	selection.ReplaceWithNodes(node.Node())

	if len(selectionSet) > 1 {
		for _, s := range selectionSet[1:] {
			s.Remove()
		}
	}

	return n.Next()
}

func (c *HtmlConverter) cloneNodes(nodes []*html.Node) []*html.Node {
	tmpNodes := make([]*html.Node, 0, len(nodes))
	for _, n := range nodes {
		tmpNodes = append(tmpNodes, c.cloneNode(n))
	}
	return tmpNodes
}

func (c *HtmlConverter) cloneNode(node *html.Node) *html.Node {
	if node == nil {
		return nil
	}

	newNode := &html.Node{
		Parent:      nil,
		NextSibling: nil,
		PrevSibling: nil,
		Type:        node.Type,
		Data:        node.Data,
		DataAtom:    node.DataAtom,
		Namespace:   node.Namespace,
		Attr:        node.Attr,
	}

	firstChild := node.FirstChild
	if firstChild != nil {
		firstChild.Parent = newNode
	}

	lastChild := node.LastChild
	if lastChild != nil {
		lastChild.Parent = newNode
	}

	newNode.FirstChild = firstChild
	newNode.LastChild = lastChild

	return newNode
}

func (c *HtmlConverter) ConvertTodoList() {
	taskId := 1
	c.document.Find("ul[class=ne-tl][ne-level=\"0\"]").Each(func(i int, selection *goquery.Selection) {
		taskId = c.ConvertTodoNodes(selection.Children().First(), taskId)
		selection.ReplaceWithNodes(NewNode(html.ElementNode, "ac:task-list").AddChildren(
			BuildNodes(c.cloneNodes(selection.Children().Nodes))).Node())
	})
}

func (c *HtmlConverter) ConvertTodoNodes(n *goquery.Selection, taskId int) int {
	if n == nil {
		return taskId
	}
	next := &goquery.Selection{}

	for {
		if n == nil || len(n.Nodes) == 0 {
			break
		}
		next = n.Next()

		if n.Is("li") {
			n.Find("[class=ne-tli-symbol]").Each(func(i int, selection *goquery.Selection) {
				selection.Remove()
			})

			n.ReplaceWithNodes(
				NewNode(html.ElementNode, "ac:task").AddChild(
					NewNode(html.ElementNode, "ac:task-id").AddChild(
						NewNode(html.TextNode, strconv.FormatInt(int64(taskId), 10)))).AddChild(
					NewNode(html.ElementNode, "ac:task-status").AddChild(
						NewNode(html.TextNode, "incomplete"))).AddChild(
					NewNode(html.ElementNode, "ac:task-body").AddChildren(
						BuildNodes(c.cloneNodes(n.Children().Nodes)))).Node())
			taskId++
		} else if n.Is("ul") {
			taskId = c.ConvertTodoNodes(n.Children().First(), taskId)

			// goquery查找时需要对:进行转义，:是关键字符
			n.Prev().Find("ac\\:task-body").AppendNodes(
				NewNode(html.ElementNode, "ac:task-list").AddChildren(
					BuildNodes(c.cloneNodes(n.Children().Nodes))).Node())
			n.Remove()
		}

		n = next
	}

	return taskId
}
