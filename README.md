[![CircleCI](https://circleci.com/gh/vanadium/go.lib.svg?style=shield)](https://circleci.com/gh/vanadium/go.lib/)
![GithubActions](https://github.com/cosnicolaou/pbzip2/actions/workflows/macos.yml/badge.svg)


This repository contains general purpose libraries created by and used by the
[github.com/vanadium](Vanadium project). They do not depend on Vanadium and
are more broadly useful.

* Creating and managing command lines, flag definitions and variables.
  * cmdline - comprehensive support for multi-level command lines (as per git etc) including support for generating godoc output fully documenting the command via go generate.
  * cmd/linewrap - formats text with appropriate word wrapping.
  * cmd/flagvar - allows appropriately tagged fields in structs to be used as flag variables.
  * lookpath - utilities for finding executables given a search path (typically $PATH).
  * envvar - routines for managing environment variables.

* Networking
  * host - utilities for accessing host information.
  * netconfig - provides the ability to monitor the underlying host for network changes and to read the OS route table. It is typically used by applications that need to monitor for IP address and routing changes in order to reconfigure themselves accordingly.
  * netstate - a comprehensive set of IPv4 and v6 aware functions for comparing a prior network state with the current one. This approach is the only way to reliably determine how a host's network configuration has changed.

* Miscelleneous 
  * metadata - provides a mechanism for setting and retrieving
    metadata stored in program binaries
  * nsync - mutex and condition variables that support cancelation.
  * toposort - a topoligcal sort implementation.
  * set - utility functions for manipulating sets of primitive type elements represented as maps
  * simplemr - a simple map reduce framework for use by single-process applications
  * textutil - utilities for handling human-readable tex
  * timing - utilities for tracking timing information
  * vlog - wraps the Google glog package to make it easier to configure and integrate with other command lines.

* Security
  * ibe - provides identity-based encryption as per ["Identity-Based Cryptosystems
    And Signature Schemes"](   (http://discovery.csc.ncsu.edu/Courses/csc774-S08/reading-assignments/shamir84.pdf))

* Testing
  * gosh - allows for running aribrary commands as subprocesses (and also as builtin functions). It is very useful for testing and also for running subproceeses in general since it takes care of managing all I/O and allows for 'tee-ing' of output etc.

