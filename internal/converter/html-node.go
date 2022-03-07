package converter

import (
	"golang.org/x/net/html"
)

type Node struct {
	htmlNode *html.Node
}

func NewNode(nodeType html.NodeType, nodeData string) *Node {
	return &Node{
		htmlNode: &html.Node{
			Type: nodeType,
			Data: nodeData,
		},
	}
}

func BuildNodes(nodes []*html.Node) []*Node {
	ns := make([]*Node, 0, len(nodes))
	for _, node := range nodes {
		ns = append(ns, &Node{
			htmlNode: node,
		})
	}
	return ns
}

func (n *Node) AddAttr(key, val string) *Node {
	n.htmlNode.Attr = append(n.htmlNode.Attr, html.Attribute{Key: key, Val: val})
	return n
}

func (n *Node) AddChild(node *Node) *Node {
	n.htmlNode.AppendChild(node.htmlNode)
	return n
}

func (n *Node) AddChildren(nodes []*Node) *Node {
	for _, node := range nodes {
		n.htmlNode.AppendChild(node.htmlNode)
	}
	return n
}

func (n *Node) Node() *html.Node {
	return n.htmlNode
}
