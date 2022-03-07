package yuque

import (
	"encoding/json"
	"strconv"
	"time"
	"yuque-sync-confluence/config"
	"yuque-sync-confluence/internal/httputil"
)

type Api struct {
	domain string
	userId string
	auth   string
}

func newClient(conf *config.YuqueConfig) *Api {
	return &Api{
		domain: conf.Domain,
		userId: conf.UserId,
		auth:   conf.Auth,
	}
}

func (a *Api) getDocListFull() ([]*RepoBrief, error) {
	repoList, err := a.getRepoListFull()
	if err != nil {
		return nil, err
	}

	for _, repo := range repoList {
		if err := a.setRepoDocs(repo); err != nil {
			return nil, err
		}
	}

	return repoList, nil
}

func (a *Api) setRepoDocs(repo *RepoBrief) error {
	docs, err := a.getRepoDocListFull(repo.Id)
	if err != nil {
		return err
	}

	docBriefs, err := a.getDocBriefs(repo.Id)
	if err != nil {
		return err
	}

	a.setRepoDocsAncestorAndUuid(docs, docBriefs)

	repo.Docs = docs
	return nil
}

func (a *Api) setRepoDocsAncestorAndUuid(docs []*DocDetail, briefs []*DocBrief) {
	for _, b := range briefs {
		doc := a.findDoc(b.Id, docs)
		doc.Uuid = b.Uuid
		doc.Ancestor = b.Ancestor
	}
	return
}

func (a *Api) getDocBriefs(repoId string) ([]*DocBrief, error) {
	// toc接口不支持offset和limit，直接拉取全量数据
	// toc接口未来可能弃用 https://www.yuque.com/yuque/developer/hq9l5y
	url := a.domain + "/api/v2/repos/" + repoId + "/toc"
	options := map[string]string{
		"X-Auth-Token": a.auth,
	}

	respBody, err := httputil.Get(url, options, nil)
	if err != nil {
		return nil, err
	}

	var respData struct {
		Data []struct {
			Id         int    `json:"id"`
			Uuid       string `json:"uuid"`
			ParentUuid string `json:"parent_uuid"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &respData); err != nil {
		return nil, err
	}

	docs := make([]*DocBrief, 0, len(respData.Data))
	for _, d := range respData.Data {
		docs = append(docs, &DocBrief{
			Id:       strconv.FormatInt(int64(d.Id), 10),
			Uuid:     d.Uuid,
			Ancestor: d.ParentUuid,
		})
	}

	return docs, nil
}

func (a *Api) findDoc(id string, docs []*DocDetail) *DocDetail {
	for _, d := range docs {
		if d.Id == id {
			return d
		}
	}
	return nil
}

func (a *Api) getRepoListFull() ([]*RepoBrief, error) {
	repoList := make([]*RepoBrief, 0)
	offset := uint64(0)
	limit := uint64(20)

	for {
		repos, err := a.getRepoList(offset)
		if err != nil {
			return nil, err
		}
		repoList = append(repoList, repos...)
		offset += limit

		if uint64(len(repos)) < limit {
			break
		}
	}

	return repoList, nil
}

func (a *Api) getRepoList(offset uint64) ([]*RepoBrief, error) {
	// repos接口仅支持offset不支持limit，固定一次拉取20条
	url := a.domain + "/api/v2/users/" + a.userId + "/repos"
	options := map[string]string{
		"X-Auth-Token": a.auth,
	}
	params := map[string]string{
		"offset": strconv.FormatUint(offset, 10),
	}
	respBody, err := httputil.Get(url, options, params)
	if err != nil {
		return nil, err
	}

	var respData struct {
		Data []struct {
			Id         int    `json:"id"`
			Title      string `json:"name"`
			UpdateTime string `json:"updated_at"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBody, &respData); err != nil {
		return nil, err
	}

	repos := make([]*RepoBrief, 0, len(respData.Data))
	for _, d := range respData.Data {
		mtime, err := time.Parse(time.RFC3339, d.UpdateTime)
		if err != nil {
			return nil, err
		}

		repos = append(repos, &RepoBrief{
			Id:    strconv.FormatInt(int64(d.Id), 10),
			Title: d.Title,
			Mtime: uint64(mtime.Unix()),
		})
	}

	return repos, nil
}

func (a *Api) getRepoDocListFull(repoId string) ([]*DocDetail, error) {
	docList := make([]*DocDetail, 0)
	offset := uint64(0)
	limit := uint64(20)

	for {
		docs, err := a.getRepoDocList(repoId, offset, limit)
		if err != nil {
			return nil, err
		}
		docList = append(docList, docs...)
		offset += limit

		if uint64(len(docs)) < limit {
			break
		}
	}

	return docList, nil
}

func (a *Api) getRepoDocList(repoId string, offset uint64, limit uint64) ([]*DocDetail, error) {
	// docs接口支持offset和limit
	url := a.domain + "/api/v2/repos/" + repoId + "/docs"
	options := map[string]string{
		"X-Auth-Token": a.auth,
	}
	params := map[string]string{
		"offset": strconv.FormatUint(offset, 10),
		"limit":  strconv.FormatUint(limit, 10),
	}
	respBody, err := httputil.Get(url, options, params)
	if err != nil {
		return nil, err
	}

	var respData struct {
		Data []struct {
			Id         int    `json:"id"`
			Title      string `json:"title"`
			UpdateTime string `json:"updated_at"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBody, &respData); err != nil {
		return nil, err
	}

	docs := make([]*DocDetail, 0, len(respData.Data))
	for _, d := range respData.Data {
		mtime, err := time.Parse(time.RFC3339, d.UpdateTime)
		if err != nil {
			return nil, err
		}

		docs = append(docs, &DocDetail{
			Id:     strconv.FormatInt(int64(d.Id), 10),
			RepoId: repoId,
			Title:  d.Title,
			Mtime:  uint64(mtime.Unix()),
		})
	}

	return docs, nil
}

func (a *Api) getDoc(repoId string, docId string) (*DocDetail, error) {
	url := a.domain + "/api/v2/repos/" + repoId + "/docs/" + docId
	options := map[string]string{
		"X-Auth-Token": a.auth,
	}
	respBody, err := httputil.Get(url, options, nil)
	if err != nil {
		return nil, err
	}

	var respData struct {
		Data struct {
			Id       int    `json:"id"`
			Title    string `json:"title"`
			BodyHtml string `json:"body_html"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &respData); err != nil {
		return nil, err
	}

	return &DocDetail{
		Id:    strconv.FormatInt(int64(respData.Data.Id), 10),
		Title: respData.Data.Title,
		Body:  respData.Data.BodyHtml,
	}, nil
}
