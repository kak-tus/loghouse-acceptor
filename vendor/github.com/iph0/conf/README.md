# conf

[![GoDoc](https://godoc.org/github.com/iph0/conf?status.svg)](https://godoc.org/github.com/iph0/conf) [![Build Status](https://travis-ci.org/iph0/conf.svg?branch=master)](https://travis-ci.org/iph0/conf) [![Go Report Card](https://goreportcard.com/badge/github.com/iph0/conf)](https://goreportcard.com/report/github.com/iph0/conf)

Package conf is an extensible solution for cascading configuration. Package conf
provides configuration processor that can load configuration layers from
different sources and merges them into the one configuration tree. In addition
configuration processor can expand references on configuration parameters in
string values and process _ref and _include directives in resulting configuration
tree. Package conf comes with built-in configuration loaders: fileconf and
envconf, and can be extended by third-party configuration loaders. Package conf
do not watch for configuration changes, but you can implement this feature in
the custom configuration loader. You can find full example in repository.

See full documentation on [GoDoc](https://godoc.org/github.com/iph0/conf) for
more information.