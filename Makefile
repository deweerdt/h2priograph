h2priograph: src/h2priograph/h2priograph.go checkgh
	export GOPATH=$(shell pwd); go build h2priograph

.PHONY=checkgh
checkgh:
	[ -e src/github.com/lucasb-eyer/go-colorful/ ] || (export GOPATH=$(shell pwd); go get github.com/lucasb-eyer/go-colorful)
