package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	//"github.com/davecgh/go-spew/spew"
	"github.com/golang/glog"
)

var (
	canonicalSubversionURLBase = flag.String("canonical_subversion_url_base", "svn+ssh://svn.gna.org/svn/gnustep", "base URL hosting GNUstep repositories which is to be used in git-svn-id in commit descriptions")
	actualSubversionURLBase    = flag.String("actual_subversion_url_base", "svn+ssh://svn.gna.org/svn/gnustep", "actual URL from which the GNUstep repositories will be fetched; this could be a locally rsync'ed copy")

	authorsFilePath = flag.String("authors_file_path", os.Getenv("GOPATH")+"/src/github.com/gnustep/gitsvn-scripts/data/authors.txt", "path to the authors.txt file")

	outputGitPathBase       = flag.String("output_git_path_base", os.Getenv("GOPATH")+"/gs-svn/git", "base path at which output git repos will be placed / updated")
	matchFileOutputPathBase = flag.String("match_file_output_path_base", os.Getenv("GOPATH")+"/gs-svn/matchfiles", "base path at which matchfiles will be placed / updated; base path will be created")

	oldGitPathBase      = flag.String("old_git_path_base", os.Getenv("GOPATH")+"/gs-svn/oldgit", "base path for old git repositories for which to generate replace refs")
	generateReplaceRefs = flag.Bool("generate_replace_refs", true, "whether replace refs should be generated for this repository")

	stdLayout = flag.Bool("stdlayout", true, "whether the Subversion repository contains a 'standard' layout; i.e. trunk/branches/tags structure")
	subpath   = flag.String("subpath", "", "subpath to lib or app to convert, sans the 'trunk' part. basename will be used as the output git repo name. example: libs/gui => gui, apps/gorm => gorm.")

	svnClone  = flag.Bool("svn_clone", true, "whether to perform the SVN->Git clone (or just use the local copy blindly)")
	matchGits = flag.Bool("match_gits", true, "whether to perform matching between the old git and new git repo")
	exportDo  = flag.Bool("export", false, "whether to prepare and perform export onto github")

	// flags for export
	exportSkippingOldIsFine = flag.Bool("export_skipping_old_is_fine", false, "whether it's okay to skip pushing the old branch when exporting; only do this if completely sure of the outcome")
)

func main() {
	os.Exit(mainWithExitCode())
}

func mainWithExitCode() int {
	flag.Set("logtostderr", "true")
	flag.Parse()

	if *subpath == "" {
		fmt.Println("specify --subpath")
		return 1
	}

	// perform an svn clone
	if *svnClone {
		c := SvnCloner{
			ActualSubversionURLBase:    *actualSubversionURLBase,
			CanonicalSubversionURLBase: *canonicalSubversionURLBase,
			OutputGitPathBase:          *outputGitPathBase,
			StdLayout:                  *stdLayout,
			Subpath:                    *subpath,
			AuthorsFilePath:            *authorsFilePath,
		}
		err := c.Clone(context.TODO())
		if err != nil {
			glog.Errorf("could not clone svn repo: %s", err)
			return 2
		}

		err = c.CopySubversionRemotesToTagsAndHeads(context.TODO())
		if err != nil {
			fmt.Errorf("could not copy subversion remotes to tags and heads: %s\n", err)
			return 10
		}
	}

	// perform matching between old and new repo
	if *matchGits {
		// fetch the oldgit repo
		_, err := oldGit()
		if err != nil {
			return 3
		}

		glog.Info("matching")
		matches, err := matcher(context.TODO())
		if err != nil {
			glog.Errorf("failed to match: %s", err)
			return 6
		}
		//spew.Dump(matches)

		if err := os.MkdirAll(*matchFileOutputPathBase, os.ModeDir|0755); err != nil {
			glog.Errorf("failed to create matchfile's directory: %s", err)
			return 8
		}
		if err := writeMatchFile(context.TODO(), matches, *matchFileOutputPathBase+"/"+*subpath+".json"); err != nil {
			glog.Errorf("failed to write matchfile: %s", err)
			return 9
		}

		if err := mixer(context.TODO(), matches); err != nil {
			glog.Errorf("failed to mix: %s", err)
			return 7
		}

	}

	if *exportDo {
		if err := export(context.TODO(), *exportSkippingOldIsFine); err != nil {
			glog.Errorf("failed to export: %s", err)
			return 12
		}
	}

	return 0
}
