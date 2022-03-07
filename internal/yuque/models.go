package yuque

import (
	"yuque-sync-confluence/config"
)

type Space struct {
	Repos []*Repo
}

type Repo struct {
	RepoInfo *RepoBrief
	Children []*DocTree
}

type DocTree struct {
	DocInfo  *DocDetail
	Children []*DocTree
}

type RepoBrief struct {
	Id    string       `json:"id"`
	Title string       `json:"title"`
	Mtime uint64       `json:"mtime"`
	Docs  []*DocDetail `json:"docs"`
}

type DocDetail struct {
	Id       string `json:"id"`
	RepoId   string `json:"repo_id"`
	Title    string `json:"title"`
	Mtime    uint64 `json:"mtime"`
	Uuid     string `json:"uuid"`
	Ancestor string `json:"ancestor"`
	Body     string `json:"body"`
}

type DocBrief struct {
	Id       string `json:"id"`
	Uuid     string `json:"uuid"`
	Ancestor string `json:"ancestor"`
}

var client *Api

func initClient(conf *config.YuqueConfig) {
	client = newClient(conf)
}

func NewSpace(conf *config.YuqueConfig) (*Space, error) {
	initClient(conf)

	repoBriefs, err := client.getDocListFull()
	if err != nil {
		return nil, err
	}

	repos, err := BuildRepos(repoBriefs)
	if err != nil {
		return nil, err
	}

	repos = filterOutSyncRepos(repos, conf.SyncRepos)
	repos = filterOutSyncRepoDocs(repos, conf.OutSyncRepos)

	return &Space{
		Repos: repos,
	}, nil
}

func filterOutSyncRepos(repos []*Repo, syncRepos []string) []*Repo {
	repoSyncMap := make(map[string]bool, 0)
	for _, repo := range syncRepos {
		repoSyncMap[repo] = true
	}

	rs := make([]*Repo, 0, len(repos))
	for _, repo := range repos {
		if _, exist := repoSyncMap[repo.RepoInfo.Title]; exist {
			rs = append(rs, repo)
		}
	}

	return rs
}

func filterOutSyncRepoDocs(repos []*Repo, outSyncDocs []string) []*Repo {
	outSyncMap := make(map[string]bool, 0)
	for _, doc := range outSyncDocs {
		outSyncMap[doc] = true
	}

	rs := make([]*Repo, 0, len(repos))
	for _, repo := range repos {
		repo.Children = filterOutSyncDocs(repo.Children, outSyncMap)
		rs = append(rs, repo)
	}

	return repos
}

func filterOutSyncDocs(docs []*DocTree, outSyncMap map[string]bool) []*DocTree {
	ds := make([]*DocTree, 0, len(docs))
	for _, doc := range docs {
		if _, exist := outSyncMap[doc.Title()]; !exist {
			doc.Children = filterOutSyncDocs(doc.Children, outSyncMap)
			ds = append(ds, doc)
		}
	}

	return ds
}

func BuildRepos(briefs []*RepoBrief) ([]*Repo, error) {
	repos := make([]*Repo, 0, len(briefs))
	for _, r := range briefs {
		repos = append(repos, ConvertRepoBriefToRepo(r))
	}

	return repos, nil
}

func ConvertRepoBriefToRepo(repo *RepoBrief) *Repo {
	children := make([]*DocTree, 0, len(repo.Docs))
	childrenMap := make(map[string]*DocTree)
	for _, doc := range repo.Docs {
		if doc.Ancestor == "" {
			tree := ConvertDocToTree(doc, repo.Docs)
			children = append(children, tree)
			childrenMap[doc.Title] = tree
		}
	}

	return &Repo{
		RepoInfo: repo,
		Children: children,
	}
}

func ConvertDocToTree(doc *DocDetail, docs []*DocDetail) *DocTree {
	children := make([]*DocTree, 0, len(docs))
	for _, d := range docs {
		if d.Ancestor == doc.Uuid {
			tree := ConvertDocToTree(d, docs)
			children = append(children, tree)
		}
	}

	return &DocTree{
		DocInfo:  doc,
		Children: children,
	}
}

func (t *DocTree) ChildByIndex(num int) *DocTree {
	if t.ChildCount() <= num {
		return nil
	}
	return t.Children[num]
}

func (t *DocTree) Mtime() uint64 {
	return t.DocInfo.Mtime
}

func (t *DocTree) RepoId() string {
	return t.DocInfo.RepoId
}

func (t *DocTree) DocId() string {
	return t.DocInfo.Id
}

func (t *DocTree) Title() string {
	return t.DocInfo.Title
}

func (t *DocTree) ChildCount() int {
	return len(t.Children)
}

func (t *DocTree) DocHtml() (string, error) {
	doc, err := client.getDoc(t.DocInfo.RepoId, t.DocInfo.Id)
	if err != nil {
		return "", err
	}

	return doc.Body, nil
}
