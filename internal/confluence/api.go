package confluence

import (
	"encoding/json"
	"strconv"
	"time"
	"yuque-sync-confluence/config"
	"yuque-sync-confluence/internal/httputil"
)

type Api struct {
	domain string
	space  string
	auth   string
}

func newClient(conf *config.ConfluenceConfig) *Api {
	return &Api{
		domain: conf.Domain,
		space:  conf.Space,
		auth:   conf.Auth,
	}
}

func (a *Api) getDocListFull() ([]*DocDetail, error) {
	docList := make([]*DocDetail, 0)
	start := uint64(0)
	limit := uint64(100)

	for {
		docs, err := a.getDocList(start, limit)
		if err != nil {
			return nil, err
		}
		docList = append(docList, docs...)
		start += limit

		if uint64(len(docs)) < limit {
			break
		}
	}

	return docList, nil
}

func (a *Api) getDocList(start uint64, limit uint64) ([]*DocDetail, error) {
	url := a.domain + "/rest/api/content/"
	options := map[string]string{
		"Authorization": a.auth,
	}
	params := map[string]string{
		"spaceKey": a.space,
		"expand":   "version,ancestors",
		"start":    strconv.FormatUint(start, 10),
		"limit":    strconv.FormatUint(limit, 10),
	}
	respBody, err := httputil.Get(url, options, params)
	if err != nil {
		return nil, err
	}

	var respData struct {
		Results []struct {
			Id      string `json:"id"`
			Title   string `json:"title"`
			Version struct {
				Number float64 `json:"number"`
				When   string  `json:"when"`
			}
			Ancestors []struct {
				Id string `json:"id"`
			}
		}
	}
	if err := json.Unmarshal(respBody, &respData); err != nil {
		return nil, err
	}

	docs := make([]*DocDetail, 0, len(respData.Results))
	for _, result := range respData.Results {
		doc := &DocDetail{}
		doc.Id = result.Id
		doc.Title = result.Title
		doc.Version = result.Version.Number
		when, err := time.Parse(time.RFC3339, result.Version.When)
		if err != nil {
			return nil, err
		}
		doc.Mtime = uint64(when.Unix())
		ancestors := make([]AncestorDetail, 0, len(result.Ancestors))
		for _, r := range result.Ancestors {
			ancestors = append(ancestors, AncestorDetail{
				Id: r.Id,
			})
		}
		doc.Ancestors = ancestors
		docs = append(docs, doc)
	}

	return docs, nil
}

func (a *Api) createDoc(docTitle string, docAncestors []AncestorDetail, docBody string) (*DocDetail, error) {
	url := a.domain + "/rest/api/content/"
	options := map[string]string{
		"Authorization": a.auth,
		"Content-Type":  "application/json",
	}
	reqBytes, err := json.Marshal(struct {
		Space     spaceDetail      `json:"space"`
		Type      string           `json:"type"`
		Title     string           `json:"title"`
		Ancestors []AncestorDetail `json:"ancestors"`
		Body      bodyDetail       `json:"body"`
	}{
		Space: spaceDetail{
			Key: a.space,
		},
		Type:      "page",
		Title:     docTitle,
		Ancestors: docAncestors,
		Body: bodyDetail{
			Storage: storageDetail{
				Value:          docBody,
				Representation: "storage",
			},
		},
	})
	if err != nil {
		return nil, err
	}
	respBody, err := httputil.Post(url, options, nil, reqBytes)
	if err != nil {
		return nil, err
	}

	var respData struct {
		Id      string `json:"id"`
		Title   string `json:"title"`
		Version struct {
			Number float64 `json:"number"`
			When   string  `json:"when"`
		}
		Ancestors []struct {
			Id string `json:"id"`
		}
	}
	if err := json.Unmarshal(respBody, &respData); err != nil {
		return nil, err
	}

	doc := &DocDetail{}
	doc.Id = respData.Id
	doc.Title = respData.Title
	doc.Version = respData.Version.Number
	when, err := time.Parse(time.RFC3339, respData.Version.When)
	if err != nil {
		return nil, err
	}
	doc.Mtime = uint64(when.Unix())
	ancestors := make([]AncestorDetail, 0, len(respData.Ancestors))
	for _, r := range respData.Ancestors {
		ancestors = append(ancestors, AncestorDetail{
			Id: r.Id,
		})
	}
	doc.Ancestors = ancestors

	return doc, nil
}

func (a *Api) updateDoc(docId string, docTitle string, docVersion float64, docBody string) (*DocDetail, error) {
	url := a.domain + "/rest/api/content/" + docId
	options := map[string]string{
		"Authorization": a.auth,
		"Content-Type":  "application/json",
	}
	var docBodyDetail bodyDetail
	if docBody != "" {
		docBodyDetail = bodyDetail{
			Storage: storageDetail{
				Value:          docBody,
				Representation: "storage",
			},
		}
	}
	req := struct {
		Version versionDetail `json:"version"`
		Type    string        `json:"type"`
		Title   string        `json:"title"`
		Body    bodyDetail    `json:"body"`
	}{
		Version: versionDetail{
			Number: docVersion,
		},
		Type:  "page",
		Title: docTitle,
		Body:  docBodyDetail,
	}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	respBody, err := httputil.Put(url, options, nil, reqBytes)
	if err != nil {
		return nil, err
	}

	var respData struct {
		Id      string `json:"id"`
		Title   string `json:"title"`
		Version struct {
			Number float64 `json:"number"`
			When   string  `json:"when"`
		}
		Ancestors []struct {
			Id string `json:"id"`
		}
	}
	if err := json.Unmarshal(respBody, &respData); err != nil {
		return nil, err
	}

	doc := &DocDetail{}
	doc.Id = respData.Id
	doc.Title = respData.Title
	doc.Version = respData.Version.Number
	when, err := time.Parse(time.RFC3339, respData.Version.When)
	if err != nil {
		return nil, err
	}
	doc.Mtime = uint64(when.Unix())
	ancestors := make([]AncestorDetail, 0, len(respData.Ancestors))
	for _, r := range respData.Ancestors {
		ancestors = append(ancestors, AncestorDetail{
			Id: r.Id,
		})
	}
	doc.Ancestors = ancestors

	return doc, nil
}

func (a *Api) createAttachment(body []byte, docID string, contentType string) error {
	url := a.domain + "/rest/api/content/" + docID + "/child/attachment"
	options := map[string]string{
		"Authorization":     a.auth,
		"Content-Type":      contentType,
		"X-Atlassian-Token": "no-check",
	}
	_, err := httputil.Post(url, options, nil, body)
	if err != nil {
		return err
	}

	return nil
}

func (a *Api) markDeprecatedDoc(tree *DocTree) error {
	for _, child := range tree.Children {
		if err := a.markDeprecatedDoc(child); err != nil {
			return err
		}
	}

	if err := a.updateDocTitle(tree.DocId(), "[Deprecated]"+tree.Title(), tree.Version()+1); err != nil {
		return err
	}

	return nil
}

func (a *Api) updateDocTitle(docId string, docTitle string, docVersion float64) error {
	url := a.domain + "/rest/api/content/" + docId
	options := map[string]string{
		"Authorization": a.auth,
		"Content-Type":  "application/json",
	}
	req := struct {
		Version versionDetail `json:"version"`
		Type    string        `json:"type"`
		Title   string        `json:"title"`
	}{
		Version: versionDetail{
			Number: docVersion,
		},
		Type:  "page",
		Title: docTitle,
	}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return err
	}
	if _, err = httputil.Put(url, options, nil, reqBytes); err != nil {
		return err
	}

	return nil
}

func (a *Api) deleteDoc(docId string) error {
	url := a.domain + "/rest/api/content/" + docId
	options := map[string]string{
		"Authorization": a.auth,
	}
	if _, err := httputil.Delete(url, options, nil); err != nil {
		return err
	}

	return nil
}

func (a *Api) getDocBrief(docId string) (*DocBrief, error) {
	url := a.domain + "/rest/api/content/" + docId
	options := map[string]string{
		"Authorization": a.auth,
	}
	respBody, err := httputil.Get(url, options, nil)
	if err != nil {
		return nil, err
	}

	var respData struct {
		Id    string      `json:"id"`
		Title string      `json:"title"`
		Space spaceDetail `json:"space"`
	}
	if err := json.Unmarshal(respBody, &respData); err != nil {
		return nil, err
	}

	return &DocBrief{
		Id:       respData.Id,
		Title:    respData.Title,
		SpaceKey: respData.Space.Key,
	}, nil
}

func (a *Api) GetAttachment(docId string, fileName string) (*FileDetail, error) {
	url := a.domain + "/rest/api/content/" + docId + "/child/attachment"
	options := map[string]string{
		"Authorization": a.auth,
	}
	params := map[string]string{
		"filename": fileName,
	}
	respBody, err := httputil.Get(url, options, params)
	if err != nil {
		return nil, err
	}

	var respData struct {
		Results []struct {
			Id     string `json:"id"`
			Type   string `json:"type"`
			Status string `json:"status"`
			Title  string `json:"title"`
		} `json:"results"`
	}
	if err := json.Unmarshal(respBody, &respData); err != nil {
		return nil, err
	}

	if len(respData.Results) == 0 {
		return nil, nil
	}

	return &FileDetail{
		Id:     respData.Results[0].Id,
		Status: respData.Results[0].Status,
		Type:   respData.Results[0].Type,
		Title:  respData.Results[0].Title,
	}, nil
}
