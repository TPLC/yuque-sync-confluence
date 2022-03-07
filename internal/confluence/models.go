package confluence

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"yuque-sync-confluence/config"
)

type Space struct {
	SpaceInfo *SpaceBrief
	Repos     []*Repo
	ReposMap  map[string]*Repo
}

type Repo struct {
	RepoInfo *RepoBrief
	TreeInfo *DocTree
}

type DocTree struct {
	DocInfo     *DocDetail
	Children    []*DocTree
	Parent      *DocTree
	ChildrenMap map[string]*DocTree
}

type DocDetail struct {
	Id        string           `json:"id"`
	Title     string           `json:"title"`
	Version   float64          `json:"version"`
	Mtime     uint64           `json:"mtime"`
	Ancestors []AncestorDetail `json:"ancestors"`
	Body      string           `json:"body"`
}

type DocBrief struct {
	Id       string `json:"id"`
	Title    string `json:"title"`
	SpaceKey string `json:"space_name"`
}

type RepoBrief struct {
	Id        string           `json:"id"`
	Title     string           `json:"title"`
	Version   float64          `json:"version"`
	Mtime     uint64           `json:"mtime"`
	Ancestors []AncestorDetail `json:"ancestors"`
}

type spaceDetail struct {
	Key string `json:"key"`
}

type AncestorDetail struct {
	Id string `json:"id"`
}

type storageDetail struct {
	Value          string `json:"value"`
	Representation string `json:"representation"`
}

type bodyDetail struct {
	Storage storageDetail `json:"storage"`
}

type versionDetail struct {
	Number float64 `json:"number"`
}

type SpaceBrief struct {
	Id  string `json:"id"`
	Key string `json:"key"`
}

type FileDetail struct {
	Id     string `json:"id"`
	Type   string `json:"type"`
	Status string `json:"status"`
	Title  string `json:"title"`
}

var client *Api

func initClient(conf *config.ConfluenceConfig) {
	client = newClient(conf)
}

func NewSpace(conf *config.ConfluenceConfig) (*Space, error) {
	initClient(conf)
	docs, err := client.getDocListFull()
	if err != nil {
		return nil, err
	}

	root := &SpaceBrief{}
	for _, doc := range docs {
		if len(doc.Ancestors) == 0 {
			root.Id = doc.Id
			root.Key = conf.Space
		}
	}

	repos, err := BuildRepos(root, docs)
	if err != nil {
		return nil, err
	}

	reposMap := make(map[string]*Repo, 0)
	for _, r := range repos {
		reposMap[r.RepoInfo.Title] = r
	}

	return &Space{
		SpaceInfo: root,
		Repos:     repos,
		ReposMap:  reposMap,
	}, nil
}

func BuildRepos(root *SpaceBrief, docs []*DocDetail) ([]*Repo, error) {
	repos := make([]*Repo, 0)
	for _, doc := range docs {
		if len(doc.Ancestors) == 0 {
			continue
		}
		if doc.Ancestors[len(doc.Ancestors)-1].Id == root.Id {
			repos = append(repos, ConvertDocToRepo(doc, docs))
		}
	}

	return repos, nil
}

func ConvertDocToRepo(doc *DocDetail, docs []*DocDetail) *Repo {
	children := make([]*DocTree, 0)
	childrenMap := make(map[string]*DocTree)
	tree := &DocTree{}
	repo := &Repo{
		RepoInfo: &RepoBrief{
			Id:      doc.Id,
			Title:   doc.Title,
			Version: doc.Version,
			Mtime:   doc.Mtime,
		},
		TreeInfo: tree,
	}
	for _, d := range docs {
		if len(d.Ancestors) == 0 {
			continue
		}
		if d.Ancestors[len(d.Ancestors)-1].Id == doc.Id {
			t := ConvertDocToTree(d, docs, tree)
			children = append(children, t)
			childrenMap[d.Title] = t
		}
	}

	tree.Children = children
	tree.ChildrenMap = childrenMap

	return repo
}

func ConvertDocToTree(doc *DocDetail, docs []*DocDetail, parent *DocTree) *DocTree {
	tree := &DocTree{}
	children := make([]*DocTree, 0, len(docs))
	childrenMap := make(map[string]*DocTree)
	for _, d := range docs {
		if len(d.Ancestors) == 0 {
			continue
		}
		if d.Ancestors[len(d.Ancestors)-1].Id == doc.Id {
			t := ConvertDocToTree(d, docs, tree)
			children = append(children, t)
			childrenMap[d.Title] = t
		}
	}

	tree.DocInfo = doc
	tree.Parent = parent
	tree.Children = children
	tree.ChildrenMap = childrenMap

	return tree
}

func (s *Space) AddRepo(repo *RepoBrief) (*Repo, error) {
	DocDetail, err := client.createDoc(repo.Title, repo.Ancestors, "")
	if err != nil {
		return nil, err
	}

	newRepo := &Repo{
		RepoInfo: &RepoBrief{
			Id:        DocDetail.Id,
			Title:     DocDetail.Title,
			Ancestors: DocDetail.Ancestors,
			Mtime:     DocDetail.Mtime,
		},
		TreeInfo: &DocTree{
			Children:    make([]*DocTree, 0),
			ChildrenMap: make(map[string]*DocTree),
		},
	}
	s.Repos = append(s.Repos, newRepo)

	return newRepo, nil
}

func (r *Repo) ChildIndex(name string) int {
	for i, c := range r.TreeInfo.Children {
		if c.Title() == name {
			return i
		}
	}

	return -1
}

func (t *DocTree) ChildIndex(name string) int {
	for i, c := range t.Children {
		if c.Title() == name {
			return i
		}
	}

	return -1
}

func (t *DocTree) ChildByName(name string) *DocTree {
	return t.ChildrenMap[name]
}

func (t *DocTree) ChildByIndex(i int) *DocTree {
	return t.Children[i]
}

func (t *DocTree) Mtime() uint64 {
	return t.DocInfo.Mtime
}

func (t *DocTree) DocId() string {
	return t.DocInfo.Id
}

func (t *DocTree) Title() string {
	return t.DocInfo.Title
}

func (t *DocTree) Version() float64 {
	return t.DocInfo.Version
}

func (t *DocTree) Ancestors() []AncestorDetail {
	return t.DocInfo.Ancestors
}

func (t *DocTree) AddEmptyDoc(title string) (*DocTree, error) {
	DocDetail, err := client.createDoc("[Temp]"+title, []AncestorDetail{{Id: t.DocInfo.Id}}, "")
	if err != nil {
		return nil, err
	}

	tree := &DocTree{
		DocInfo:     DocDetail,
		Children:    make([]*DocTree, 0),
		ChildrenMap: make(map[string]*DocTree),
	}
	t.Children = append(t.Children, tree)
	t.ChildrenMap[DocDetail.Title] = tree

	return tree, nil
}

func (t *DocTree) UpdateDoc(title string, docHtml string) error {
	DocDetail, err := client.updateDoc(t.DocInfo.Id, title, t.DocInfo.Version+1, docHtml)
	if err != nil {
		return err
	}

	t.DocInfo = DocDetail
	return nil
}

func (t *DocTree) AddDocAttachment(fileBody []byte, fileName string) error {
	// 判断文档是否有同名的附件
	fileDetail, err := client.GetAttachment(t.DocId(), fileName)
	if err != nil {
		return err
	}
	if fileDetail != nil {
		return nil
	}

	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)
	content, _ := writer.CreateFormFile("file", fileName)
	if _, err := io.Copy(content, bytes.NewReader(fileBody)); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	if err := client.createAttachment(payload.Bytes(), t.DocId(), writer.FormDataContentType()); err != nil {
		return err
	}

	return nil
}

func (t *DocTree) MarkDeprecated() error {
	newDocName := "[Deprecated]" + t.Title()
	newDocVersion := t.Version() + 1
	if err := client.updateDocTitle(t.DocId(), newDocName, newDocVersion); err != nil {
		return err
	}

	t.DocInfo.Title = newDocName
	t.DocInfo.Version = newDocVersion

	return nil
}

func (t *DocTree) DeleteDoc() error {
	if err := client.deleteDoc(t.DocId()); err != nil {
		return err
	}

	index := t.Parent.ChildIndex(t.Title())
	if index == -1 {
		return errors.New(fmt.Sprintf("doc %v not dound in doc", t.Title()))
	}
	t.Parent.Children = append(t.Parent.Children[:index], t.Parent.Children[index+1:]...)
	delete(t.Parent.ChildrenMap, t.Title())

	return nil
}

func (t *DocTree) ConfirmDocOwner(space string) (bool, error) {
	docBrief, err := client.getDocBrief(t.DocId())
	if err != nil {
		return false, err
	}
	if docBrief.SpaceKey != space {
		return false, nil
	}

	return true, nil
}
