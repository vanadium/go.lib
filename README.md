
[![Build Status](https://travis-ci.org/vanadium/go.lib.svg?branch=master)](https://travis-ci.org/vanadium/go.lib)

This repository contains general purpose libraries created by and used by the
[github.com/vanadium](Vanadium project). They do not depend on Vanadium and
are more broadly useful.

* Creating and managing command lines
  * cmdline - comprehensive support for multi-level command lines (as per git etc including support for generating godoc output fully documenting the command via go generate.
  * cmd/linewrap
  * cmd/flagvar

* Testing and Utilities

* Networking
  * host
  * netconfig
  * netstate

* Miscelleneous 

dbutil
envvar
gosh

ibe
lookpath
metadata
nsync
set
simplemr
textutil
timing
toposort
vlog