package static
import "github.com/apesternikov/bindata"
import (
  docroot "github.com/apesternikov/backplane/src/backplane/static/docroot"
  tpls "github.com/apesternikov/backplane/src/backplane/static/tpls"
)
var Files = []*bindata.Bindata{  }
var Dirs = []*bindata.Dir{ docroot.Dir, tpls.Dir }
var Dir = &bindata.Dir{Pkg: "static", Files: Files, Dirs: Dirs, FullPkgName: "github.com/apesternikov/backplane/src/backplane/static"}
