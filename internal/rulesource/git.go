package rulesource

import (
	"fmt"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
)

// resolveRef contacts the remote at url and returns the commit SHA and full
// reference name for ref. An empty ref resolves the remote's default-branch
// HEAD; a non-empty ref is matched against branches then tags.
func resolveRef(url, ref string) (sha string, name plumbing.ReferenceName, err error) {
	remote := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{url},
	})
	refs, err := remote.List(&git.ListOptions{})
	if err != nil {
		return "", "", fmt.Errorf("contact rules repo %s: %w", url, err)
	}

	if ref == "" {
		// HEAD may come back as a hash ref directly or as a symbolic ref
		// whose target we resolve among the listed refs.
		var headTarget plumbing.ReferenceName
		for _, r := range refs {
			if r.Name() == plumbing.HEAD {
				if r.Type() == plumbing.HashReference {
					return r.Hash().String(), "", nil
				}
				headTarget = r.Target()
			}
		}
		for _, r := range refs {
			if r.Name() == headTarget {
				return r.Hash().String(), "", nil
			}
		}
		return "", "", fmt.Errorf("rules repo %s: could not resolve HEAD", url)
	}

	branch := plumbing.NewBranchReferenceName(ref)
	tag := plumbing.NewTagReferenceName(ref)
	for _, r := range refs {
		if r.Name() == branch || r.Name() == tag {
			return r.Hash().String(), r.Name(), nil
		}
	}
	return "", "", fmt.Errorf("rules repo %s: ref %q not found", url, ref)
}

// cloneInto shallow-clones url at refName into dest. An empty refName clones
// the default branch.
func cloneInto(url string, refName plumbing.ReferenceName, dest string) error {
	opts := &git.CloneOptions{URL: url, Depth: 1, SingleBranch: true}
	if refName != "" {
		opts.ReferenceName = refName
	}
	if _, err := git.PlainClone(dest, false, opts); err != nil {
		return fmt.Errorf("clone rules repo %s: %w", url, err)
	}
	return nil
}
