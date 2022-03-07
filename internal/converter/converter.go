package converter

import (
	"errors"
	"fmt"
	"strings"
	"yuque-sync-confluence/config"
	"yuque-sync-confluence/internal/confluence"
	"yuque-sync-confluence/internal/yuque"
)

type Converter struct {
	confluenceSpace *confluence.Space
	yuqueSpace      *yuque.Space
	err             error
}

func NewConverter(cfg *config.Config) (*Converter, error) {
	confluenceSpace, err := confluence.NewSpace(cfg.Confluence)
	if err != nil {
		return nil, err
	}
	yuqueSpace, err := yuque.NewSpace(cfg.Yuque)
	if err != nil {
		return nil, err
	}
	return &Converter{
		confluenceSpace: confluenceSpace,
		yuqueSpace:      yuqueSpace,
	}, nil
}

func (c *Converter) Execute() error {
	if err := c.ClearTempDocs(); err != nil {
		return err
	}
	if err := c.DeprecateDocs(); err != nil {
		return err
	}
	if err := c.ConvertRepos(); err != nil {
		return err
	}
	if err := c.ClearTempDocs(); err != nil {
		return err
	}

	return nil
}

func (c *Converter) ClearTempDocs() error {
	for _, r := range c.confluenceSpace.Repos {
		for _, d := range r.TreeInfo.Children {
			if strings.HasPrefix(d.Title(), "[Temp]") {
				own, err := d.ConfirmDocOwner(c.confluenceSpace.SpaceInfo.Key)
				if err != nil {
					return err
				}
				if own == false {
					return errors.New(fmt.Sprintf("%v is not belong to %v", d.Title(), c.confluenceSpace.SpaceInfo.Key))
				}
				if err := d.DeleteDoc(); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (c *Converter) ConvertRepos() error {
	var (
		confluenceSpace = c.confluenceSpace
		yuqueSpace      = c.yuqueSpace
	)

	for i := 0; i < len(yuqueSpace.Repos); i++ {
		yRepo := yuqueSpace.Repos[i]
		cRepo, exist := confluenceSpace.ReposMap[yRepo.RepoInfo.Title]
		if !exist {
			repo, err := confluenceSpace.AddRepo(&confluence.RepoBrief{
				Title: yRepo.RepoInfo.Title,
				Ancestors: []confluence.AncestorDetail{
					{
						Id: confluenceSpace.SpaceInfo.Id,
					},
				},
			})
			if err != nil {
				return err
			}
			cRepo = repo
		}

		if err := c.ConvertRepo(yRepo, cRepo); err != nil {
			return err
		}
	}

	return nil
}

func (c *Converter) ConvertRepo(yuqueRepo *yuque.Repo, confluenceRepo *confluence.Repo) error {
	yuqueTree := &yuque.DocTree{
		DocInfo: &yuque.DocDetail{
			Title: yuqueRepo.RepoInfo.Title,
		},
		Children: yuqueRepo.Children,
	}
	confluenceTree := &confluence.DocTree{
		DocInfo: &confluence.DocDetail{
			Id:        confluenceRepo.RepoInfo.Id,
			Title:     confluenceRepo.RepoInfo.Title,
			Version:   confluenceRepo.RepoInfo.Version,
			Mtime:     confluenceRepo.RepoInfo.Mtime,
			Ancestors: confluenceRepo.RepoInfo.Ancestors,
		},
		Children:    confluenceRepo.TreeInfo.Children,
		ChildrenMap: confluenceRepo.TreeInfo.ChildrenMap,
	}
	if err := c.Convert(yuqueTree, confluenceTree); err != nil {
		return err
	}

	return nil
}

func (c *Converter) Convert(yuqueTree *yuque.DocTree, confluenceTree *confluence.DocTree) error {
	for i := 0; i < yuqueTree.ChildCount(); i++ {
		yTree := yuqueTree.ChildByIndex(i)
		cTree := confluenceTree.ChildByName(yTree.Title())
		if cTree != nil {
			if cTree.Mtime() < yTree.Mtime() {
				htmlConverter, err := NewHtmlConverter(yTree, cTree)
				if err != nil {
					return err
				}
				if err := htmlConverter.Convert(); err != nil {
					return err
				}
			}
		} else {
			tree, err := confluenceTree.AddEmptyDoc(yTree.Title())
			if err != nil {
				return err
			}
			htmlConverter, err := NewHtmlConverter(yTree, tree)
			if err != nil {
				return err
			}
			if err := htmlConverter.Convert(); err != nil {
				return err
			}
			cTree = tree
		}

		if err := c.Convert(yTree, cTree); err != nil {
			return err
		}
	}

	return nil
}

func (c *Converter) DeprecateDocs() error {
	for _, yuqueRepo := range c.yuqueSpace.Repos {
		if confluenceRepo, exist := c.confluenceSpace.ReposMap[yuqueRepo.RepoInfo.Title]; exist {
			yuqueTree := &yuque.DocTree{
				DocInfo: &yuque.DocDetail{
					Title: yuqueRepo.RepoInfo.Title,
				},
				Children: yuqueRepo.Children,
			}
			confluenceTree := &confluence.DocTree{
				DocInfo: &confluence.DocDetail{
					Title: confluenceRepo.RepoInfo.Title,
				},
				Children:    confluenceRepo.TreeInfo.Children,
				ChildrenMap: confluenceRepo.TreeInfo.ChildrenMap,
			}
			if err := c.DeprecateChildDocs(yuqueTree, confluenceTree); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *Converter) DeprecateChildDocs(yuqueTree *yuque.DocTree, confluenceTree *confluence.DocTree) error {
	yuqueTitleMap := make(map[string]*yuque.DocTree, 0)
	for i := 0; i < yuqueTree.ChildCount(); i++ {
		yuqueTitleMap[yuqueTree.ChildByIndex(i).Title()] = yuqueTree.ChildByIndex(i)
	}

	for _, cTree := range confluenceTree.Children {
		if yTree, exist := yuqueTitleMap[cTree.Title()]; exist {
			if err := c.DeprecateChildDocs(yTree, cTree); err != nil {
				return err
			}
		} else {
			if strings.HasPrefix(cTree.Title(), "[Protected]") {
				continue
			}
			if strings.HasPrefix(cTree.Title(), "[Deprecated]") {
				continue
			}
			if err := cTree.MarkDeprecated(); err != nil {
				return err
			}
		}
	}

	return nil
}
