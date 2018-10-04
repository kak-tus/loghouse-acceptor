# merger

[![GoDoc](https://godoc.org/github.com/iph0/merger?status.svg)](https://godoc.org/github.com/iph0/merger) [![Build Status](https://travis-ci.org/iph0/merger.svg?branch=master)](https://travis-ci.org/iph0/merger) [![Go Report Card](https://goreportcard.com/badge/github.com/iph0/merger?style=flat)](https://goreportcard.com/report/github.com/iph0/merger)

Package merger performs recursive merge of maps or structures into new one.
Non-zero values from the right side has higher precedence. Slices do not
merging, because main use case of this package is merging configuration
parameters, and in this case merging of slices is unacceptable. Slices from
the right side has higher precedence.

See documentation on [GoDoc](https://godoc.org/github.com/iph0/merger) for more
information.